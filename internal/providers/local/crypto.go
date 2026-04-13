// Package local implements the local encrypted vault provider.
//
// Security model (inspired by HashiCorp Vault's barrier design):
//
//  1. A random 256-bit encryption key (EK) is generated at vault init time.
//  2. The EK is wrapped with a barrier key derived from the master password via Argon2id.
//  3. At unlock time the EK is unwrapped and held in memory; the master password is
//     discarded — it is never held in memory beyond the unlock call.
//  4. All vault entries are encrypted with the EK (AES-256-GCM, unique nonce per entry).
//  5. Rotating the master password re-wraps the EK without re-encrypting all entries.
//  6. Sensitive byte slices are zeroed before being released to the GC.
//
// On-disk format (vault.enc):
//
//	[header JSON]\n[entries JSON encrypted with EK]
//
// Header (plaintext, integrity-protected by its own HMAC-SHA256):
//
//	{ "version": 1, "kdf": "argon2id", "kdf_params": {...},
//	  "wrapped_key": "<base64>", "hmac": "<base64>" }
//
// The header HMAC covers all header fields except the hmac field itself,
// keyed with the barrier key — so any tampering is detected on unseal.
package local

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"runtime"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2id parameters — tuned for ~100ms on commodity hardware.
	argon2Time    uint32 = 3
	argon2Memory  uint32 = 64 * 1024 // 64 MiB
	argon2Threads uint8  = 4
	keyLen               = 32 // AES-256 / 256-bit

	saltLen      = 16
	nonceLen     = 12
	vaultVersion = 1
)

// DefaultKDFParams are the production Argon2id parameters.
var DefaultKDFParams = struct {
	Time    uint32
	Memory  uint32
	Threads uint8
}{
	Time:    argon2Time,
	Memory:  argon2Memory,
	Threads: argon2Threads,
}

// activeKDFParams holds the parameters used for new key derivations.
// Override in tests via OverrideKDFParamsForTesting.
var activeTime    = argon2Time
var activeMemory  = argon2Memory
var activeThreads = argon2Threads

// OverrideKDFParamsForTesting replaces the Argon2id parameters.
// Must only be called from test code.
func OverrideKDFParamsForTesting(time uint32, memory uint32, threads uint8) {
	activeTime    = time
	activeMemory  = memory
	activeThreads = threads
}

// kdfParams are stored in the vault header so parameters can evolve.
type kdfParams struct {
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	SaltB64 string `json:"salt"`
}

// vaultHeader is the plaintext header written at the top of vault.enc.
// It contains the wrapped encryption key and enough metadata to unseal.
type vaultHeader struct {
	Version   int       `json:"version"`
	KDF       string    `json:"kdf"`
	KDFParams kdfParams `json:"kdf_params"`
	// WrappedKey is the AES-256 encryption key sealed with the barrier key
	// (AES-256-GCM: [nonce][ciphertext+tag], base64-encoded).
	WrappedKeyB64 string `json:"wrapped_key"`
	// HMAC-SHA256 of the canonical JSON of all other fields, keyed with the
	// barrier key — prevents offline tampering with KDF params or wrapped key.
	HMACB64 string `json:"hmac,omitempty"`
}

// barrierKeys holds ephemeral key material that must be zeroed after use.
type barrierKeys struct {
	barrier []byte // derived from master password — used only to wrap/unwrap EK
	ek      []byte // encryption key — held in memory while vault is unsealed
}

func (k *barrierKeys) zero() {
	zeroBytes(k.barrier)
	zeroBytes(k.ek)
	runtime.KeepAlive(k)
}

// zeroBytes overwrites a byte slice with zeros.
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// deriveBarrierKey runs Argon2id with the given params and returns the barrier key.
// The caller is responsible for zeroing the returned slice.
func deriveBarrierKey(password string, p kdfParams) ([]byte, error) {
	salt, err := base64.StdEncoding.DecodeString(p.SaltB64)
	if err != nil {
		return nil, fmt.Errorf("decode kdf salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, keyLen)
	return key, nil
}

// initHeader creates a new vault header, generating a fresh random EK and salt,
// and wraps the EK with a barrier key derived from the master password.
// Returns the header and the unwrapped EK (caller must zero after use).
func initHeader(password string) (*vaultHeader, []byte, error) {
	// Generate random salt for KDF.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, fmt.Errorf("generate kdf salt: %w", err)
	}

	params := kdfParams{
		Time:    activeTime,
		Memory:  activeMemory,
		Threads: activeThreads,
		SaltB64: base64.StdEncoding.EncodeToString(salt),
	}

	barrier, err := deriveBarrierKey(password, params)
	if err != nil {
		return nil, nil, err
	}
	defer zeroBytes(barrier)

	// Generate random encryption key.
	ek := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, ek); err != nil {
		zeroBytes(ek)
		return nil, nil, fmt.Errorf("generate encryption key: %w", err)
	}

	wrappedKey, err := sealBytes(ek, barrier)
	if err != nil {
		zeroBytes(ek)
		return nil, nil, fmt.Errorf("wrap encryption key: %w", err)
	}

	h := &vaultHeader{
		Version:       vaultVersion,
		KDF:           "argon2id",
		KDFParams:     params,
		WrappedKeyB64: base64.StdEncoding.EncodeToString(wrappedKey),
	}

	// Compute HMAC over header fields (barrier key as HMAC key).
	mac, err := headerHMAC(h, barrier)
	if err != nil {
		zeroBytes(ek)
		return nil, nil, err
	}
	h.HMACB64 = base64.StdEncoding.EncodeToString(mac)

	// Return ek to caller — caller must zero when done.
	return h, ek, nil
}

// unsealHeader verifies the header HMAC and unwraps the encryption key.
// Returns the EK (caller must zero) or an error if the password is wrong
// or the header has been tampered with.
func unsealHeader(h *vaultHeader, password string) ([]byte, error) {
	barrier, err := deriveBarrierKey(password, h.KDFParams)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(barrier)

	// Verify header integrity before using any values from it.
	storedMAC, err := base64.StdEncoding.DecodeString(h.HMACB64)
	if err != nil {
		return nil, fmt.Errorf("decode header hmac: %w", err)
	}
	expectedMAC, err := headerHMAC(h, barrier)
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare(storedMAC, expectedMAC) != 1 {
		return nil, fmt.Errorf("vault header authentication failed (wrong password or tampered)")
	}

	// Unwrap the encryption key.
	wrappedKey, err := base64.StdEncoding.DecodeString(h.WrappedKeyB64)
	if err != nil {
		return nil, fmt.Errorf("decode wrapped key: %w", err)
	}
	ek, err := openBytes(wrappedKey, barrier)
	if err != nil {
		return nil, fmt.Errorf("unwrap encryption key: %w", err)
	}
	return ek, nil
}

// rewrapKey re-derives the barrier key from a new password and re-wraps the EK.
// The EK itself (and all encrypted entries) are unchanged.
func rewrapKey(h *vaultHeader, ek []byte, newPassword string) (*vaultHeader, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate new salt: %w", err)
	}

	params := kdfParams{
		Time:    activeTime,
		Memory:  activeMemory,
		Threads: activeThreads,
		SaltB64: base64.StdEncoding.EncodeToString(salt),
	}

	newBarrier, err := deriveBarrierKey(newPassword, params)
	if err != nil {
		return nil, err
	}
	defer zeroBytes(newBarrier)

	wrappedKey, err := sealBytes(ek, newBarrier)
	if err != nil {
		return nil, fmt.Errorf("re-wrap key: %w", err)
	}

	newH := &vaultHeader{
		Version:       vaultVersion,
		KDF:           "argon2id",
		KDFParams:     params,
		WrappedKeyB64: base64.StdEncoding.EncodeToString(wrappedKey),
	}
	mac, err := headerHMAC(newH, newBarrier)
	if err != nil {
		return nil, err
	}
	newH.HMACB64 = base64.StdEncoding.EncodeToString(mac)
	return newH, nil
}

// headerHMAC computes HMAC-SHA256 over the canonical JSON of h (with HMAC field omitted).
func headerHMAC(h *vaultHeader, key []byte) ([]byte, error) {
	// Serialise without the hmac field.
	tmp := struct {
		Version       int       `json:"version"`
		KDF           string    `json:"kdf"`
		KDFParams     kdfParams `json:"kdf_params"`
		WrappedKeyB64 string    `json:"wrapped_key"`
	}{
		Version:       h.Version,
		KDF:           h.KDF,
		KDFParams:     h.KDFParams,
		WrappedKeyB64: h.WrappedKeyB64,
	}
	data, err := json.Marshal(tmp)
	if err != nil {
		return nil, fmt.Errorf("marshal header for hmac: %w", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil), nil
}

// sealBytes encrypts src with key using AES-256-GCM. Returns [nonce || ciphertext+tag].
func sealBytes(src, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, src, nil)
	out := make([]byte, len(nonce)+len(ct))
	copy(out, nonce)
	copy(out[len(nonce):], ct)
	return out, nil
}

// openBytes decrypts data produced by sealBytes.
func openBytes(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(data) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, data[:ns], data[ns:], nil)
}

// encryptEntry encrypts a single entry value with the EK.
// Returns [nonce || ciphertext+tag] base64-encoded.
func encryptEntry(plaintext string, ek []byte) (string, error) {
	ct, err := sealBytes([]byte(plaintext), ek)
	if err != nil {
		return "", fmt.Errorf("encrypt entry: %w", err)
	}
	return base64.StdEncoding.EncodeToString(ct), nil
}

// decryptEntry decrypts a single entry encrypted by encryptEntry.
func decryptEntry(b64 string, ek []byte) (string, error) {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("decode entry: %w", err)
	}
	pt, err := openBytes(data, ek)
	if err != nil {
		return "", fmt.Errorf("decrypt entry: %w", err)
	}
	return string(pt), nil
}
