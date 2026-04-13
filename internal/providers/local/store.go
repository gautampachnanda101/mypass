package local

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/providers"
)

// onDiskEntry holds one encrypted entry in the vault file.
type onDiskEntry struct {
	// CiphertextB64: base64([nonce || ciphertext+tag]) produced by encryptEntry.
	CiphertextB64 string    `json:"ct"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// onDiskEntries is the plaintext entries map that gets encrypted as a blob.
type onDiskEntries struct {
	Entries map[string]onDiskEntry `json:"entries"`
}

// Provider is the local encrypted file vault.
//
// Security model (mirrors HashiCorp Vault barrier design):
//   - Master password → Argon2id → barrier key (discarded after Unlock)
//   - Barrier key unwraps the encryption key (EK) stored in the vault header
//   - EK held in memory while unsealed; zeroed on Lock
//   - Each entry value encrypted with EK + unique nonce per write
//   - ChangePassword re-wraps EK only — entries are not re-encrypted
//   - Vault header authenticated via HMAC-SHA256(barrier key) — tampering detected on Unlock
type Provider struct {
	id   string
	path string

	mu     sync.RWMutex
	sealed bool
	ek     []byte       // encryption key, non-nil when unsealed
	header *vaultHeader // retained for ChangePassword
}

// New creates a local vault provider. The vault is sealed until Unlock is called.
func New(id, path string) *Provider {
	return &Provider{id: id, path: path, sealed: true}
}

func (p *Provider) ID() string { return p.id }

// Init creates a new empty vault file at p.path encrypted with password.
// Returns an error if the file already exists.
func (p *Provider) Init(password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(p.path); err == nil {
		return fmt.Errorf("vault already exists at %s", p.path)
	}

	header, ek, err := initHeader(password)
	if err != nil {
		return fmt.Errorf("init vault: %w", err)
	}

	empty := &onDiskEntries{Entries: map[string]onDiskEntry{}}
	if err := persistVault(p.path, header, empty, ek); err != nil {
		zeroBytes(ek)
		return err
	}

	p.header = header
	p.ek = ek
	p.sealed = false
	return nil
}

// Unlock reads the vault header, verifies the HMAC, derives the barrier key,
// unwraps the EK, and holds it in memory. The master password is not retained.
func (p *Provider) Unlock(password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	header, err := readHeader(p.path)
	if err != nil {
		return err
	}

	ek, err := unsealHeader(header, password)
	if err != nil {
		return err
	}

	if p.ek != nil {
		zeroBytes(p.ek)
	}
	p.ek = ek
	p.header = header
	p.sealed = false
	return nil
}

// Lock zeros the in-memory EK and marks the vault sealed.
func (p *Provider) Lock() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ek != nil {
		zeroBytes(p.ek)
		p.ek = nil
	}
	p.header = nil
	p.sealed = true
}

// IsSealed reports whether the vault is currently sealed.
func (p *Provider) IsSealed() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sealed
}

// ChangePassword re-wraps the EK with newPassword without re-encrypting entries.
func (p *Provider) ChangePassword(newPassword string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sealed {
		return &providers.ErrLocked{Provider: p.id}
	}

	newHeader, err := rewrapKey(p.header, p.ek, newPassword)
	if err != nil {
		return fmt.Errorf("change password: %w", err)
	}

	entries, err := readEntries(p.path, p.ek)
	if err != nil {
		return err
	}

	if err := persistVault(p.path, newHeader, entries, p.ek); err != nil {
		return err
	}
	p.header = newHeader
	return nil
}

func (p *Provider) Health(_ context.Context) error {
	if p.IsSealed() {
		return &providers.ErrLocked{Provider: p.id}
	}
	return nil
}

func (p *Provider) Get(_ context.Context, path string) (providers.Secret, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.sealed {
		return providers.Secret{}, &providers.ErrLocked{Provider: p.id}
	}

	entries, err := readEntries(p.path, p.ek)
	if err != nil {
		return providers.Secret{}, err
	}

	e, ok := entries.Entries[path]
	if !ok {
		return providers.Secret{}, &providers.ErrNotFound{Provider: p.id, Path: path}
	}

	value, err := decryptEntry(e.CiphertextB64, p.ek)
	if err != nil {
		return providers.Secret{}, fmt.Errorf("decrypt %s: %w", path, err)
	}

	return providers.Secret{
		Key:       path,
		Value:     value,
		Provider:  p.id,
		UpdatedAt: e.UpdatedAt,
	}, nil
}

func (p *Provider) List(_ context.Context, prefix string) ([]providers.Secret, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.sealed {
		return nil, &providers.ErrLocked{Provider: p.id}
	}

	entries, err := readEntries(p.path, p.ek)
	if err != nil {
		return nil, err
	}

	var out []providers.Secret
	for k, e := range entries.Entries {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out = append(out, providers.Secret{
				Key:       k,
				Provider:  p.id,
				UpdatedAt: e.UpdatedAt,
				// Value intentionally omitted — callers must Get explicitly
			})
		}
	}
	return out, nil
}

// Set stores or updates a secret. Each write encrypts the value with a fresh nonce.
func (p *Provider) Set(_ context.Context, path, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sealed {
		return &providers.ErrLocked{Provider: p.id}
	}

	entries, err := readEntries(p.path, p.ek)
	if err != nil {
		return err
	}

	ct, err := encryptEntry(value, p.ek)
	if err != nil {
		return err
	}

	entries.Entries[path] = onDiskEntry{CiphertextB64: ct, UpdatedAt: time.Now().UTC()}
	return persistVault(p.path, p.header, entries, p.ek)
}

// Delete removes a secret and rewrites the vault.
func (p *Provider) Delete(_ context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sealed {
		return &providers.ErrLocked{Provider: p.id}
	}

	entries, err := readEntries(p.path, p.ek)
	if err != nil {
		return err
	}

	if _, ok := entries.Entries[path]; !ok {
		return &providers.ErrNotFound{Provider: p.id, Path: path}
	}

	delete(entries.Entries, path)
	return persistVault(p.path, p.header, entries, p.ek)
}

// readHeader reads and parses only the header line from the vault file.
func readHeader(path string) (*vaultHeader, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("vault not found at %s — run: vaultx init", path)
	}
	if err != nil {
		return nil, fmt.Errorf("read vault: %w", err)
	}

	nl := strings.IndexByte(string(raw), '\n')
	if nl < 0 {
		return nil, fmt.Errorf("malformed vault: missing header separator")
	}

	var header vaultHeader
	if err := json.Unmarshal(raw[:nl], &header); err != nil {
		return nil, fmt.Errorf("parse vault header: %w", err)
	}
	return &header, nil
}

// readEntries reads and decrypts the entries section of the vault file.
func readEntries(path string, ek []byte) (*onDiskEntries, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read vault: %w", err)
	}

	nl := strings.IndexByte(string(raw), '\n')
	if nl < 0 {
		return nil, fmt.Errorf("malformed vault: missing header separator")
	}

	body := raw[nl+1:]
	if len(body) == 0 {
		return &onDiskEntries{Entries: map[string]onDiskEntry{}}, nil
	}

	plaintext, err := openBytes(body, ek)
	if err != nil {
		return nil, fmt.Errorf("decrypt entries: %w", err)
	}

	var entries onDiskEntries
	if err := json.Unmarshal(plaintext, &entries); err != nil {
		return nil, fmt.Errorf("parse entries: %w", err)
	}
	if entries.Entries == nil {
		entries.Entries = map[string]onDiskEntry{}
	}
	return &entries, nil
}

// persistVault atomically writes header + encrypted entries to disk.
// Layout: headerJSON\n<AES-256-GCM encrypted entries JSON>
func persistVault(path string, header *vaultHeader, entries *onDiskEntries, ek []byte) error {
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return fmt.Errorf("marshal header: %w", err)
	}

	entriesJSON, err := json.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal entries: %w", err)
	}

	entriesCT, err := sealBytes(entriesJSON, ek)
	zeroBytes(entriesJSON) // zero plaintext immediately after sealing
	if err != nil {
		return fmt.Errorf("encrypt entries: %w", err)
	}

	out := make([]byte, 0, len(headerJSON)+1+len(entriesCT))
	out = append(out, headerJSON...)
	out = append(out, '\n')
	out = append(out, entriesCT...)

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create vault dir: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}
	return os.Rename(tmp, path)
}
