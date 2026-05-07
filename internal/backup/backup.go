// Package backup provides encrypted vault backups with Shamir secret sharing.
package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/hashicorp/vault/shamir"
	"golang.org/x/crypto/argon2"
)

const (
	backupDir      = ".vaultx/backups"
	keyDeriveSalt  = "vaultx-backup-kdf-salt-v1"
	argonTime      = 3
	argonMemory    = 64 * 1024 // 64 MiB
	argonThreads   = 4
	argonKeyLength = 32
)

// BackupMeta holds metadata for a backup file.
type BackupMeta struct {
	Timestamp time.Time `json:"timestamp"`
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
}

// Manager handles vault backups.
type Manager struct {
	backupDir string
}

// New creates a backup manager with the given backup directory.
func New(backupDir string) *Manager {
	return &Manager{backupDir: backupDir}
}

// AutoBackup creates an encrypted backup of the vault file.
// The backup is encrypted with a key derived from the master password.
// Returns the backup filename.
func (m *Manager) AutoBackup(vaultPath, masterPassword string) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0700); err != nil {
		return "", fmt.Errorf("create backup dir: %w", err)
	}

	// Read vault file
	vaultData, err := os.ReadFile(vaultPath)
	if err != nil {
		return "", fmt.Errorf("read vault: %w", err)
	}

	// Derive backup encryption key from master password
	key := argon2.IDKey([]byte(masterPassword), []byte(keyDeriveSalt),
		argonTime, argonMemory, argonThreads, argonKeyLength)

	// Encrypt vault data with AES-256-GCM
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, vaultData, nil)

	// Write backup file with timestamp
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	backupFile := filepath.Join(m.backupDir, fmt.Sprintf("vault-%s.enc", timestamp))

	if err := os.WriteFile(backupFile, ciphertext, 0600); err != nil {
		return "", fmt.Errorf("write backup: %w", err)
	}

	return backupFile, nil
}

// ListBackups returns a list of available backups sorted by timestamp (newest first).
func (m *Manager) ListBackups() ([]BackupMeta, error) {
	entries, err := os.ReadDir(m.backupDir)
	if os.IsNotExist(err) {
		return []BackupMeta{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var backups []BackupMeta
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".enc" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupMeta{
			Timestamp: info.ModTime(),
			Filename:  entry.Name(),
			Size:      info.Size(),
		})
	}

	// Sort by timestamp descending
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Timestamp.After(backups[j].Timestamp)
	})

	return backups, nil
}

// Restore decrypts a backup and restores it to the vault path.
func (m *Manager) Restore(backupFilename, masterPassword, vaultPath string) error {
	backupPath := filepath.Join(m.backupDir, backupFilename)

	// Read encrypted backup
	ciphertext, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	// Derive decryption key
	key := argon2.IDKey([]byte(masterPassword), []byte(keyDeriveSalt),
		argonTime, argonMemory, argonThreads, argonKeyLength)

	// Decrypt
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	vaultData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt backup: %w", err)
	}

	// Write to vault path
	if err := os.WriteFile(vaultPath, vaultData, 0600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}

	return nil
}

// ShamirShare holds a single Shamir secret share.
type ShamirShare struct {
	Index int    `json:"index"`
	Share string `json:"share"` // Base64-encoded
}

// SplitBackupKey splits the backup encryption key into N shares, requiring M to reconstruct.
// Returns the shares as base64-encoded strings.
func SplitBackupKey(masterPassword string, n, m int) ([]ShamirShare, error) {
	if n < 2 || m < 2 || m > n {
		return nil, fmt.Errorf("invalid Shamir parameters: n=%d, m=%d (must have 2 <= m <= n)", n, m)
	}

	// Derive backup key
	key := argon2.IDKey([]byte(masterPassword), []byte(keyDeriveSalt),
		argonTime, argonMemory, argonThreads, argonKeyLength)

	// Split key using Shamir
	shares, err := shamir.Split(key, n, m)
	if err != nil {
		return nil, fmt.Errorf("split key: %w", err)
	}

	result := make([]ShamirShare, len(shares))
	for i, share := range shares {
		result[i] = ShamirShare{
			Index: i + 1,
			Share: base64.StdEncoding.EncodeToString(share),
		}
	}

	return result, nil
}

// CombineBackupKey reconstructs the backup encryption key from M Shamir shares.
// shares: map of share index -> base64-encoded share data
func CombineBackupKey(shares []ShamirShare) ([]byte, error) {
	if len(shares) == 0 {
		return nil, fmt.Errorf("no shares provided")
	}

	// Decode shares
	decodedShares := make([][]byte, len(shares))
	for i, s := range shares {
		decoded, err := base64.StdEncoding.DecodeString(s.Share)
		if err != nil {
			return nil, fmt.Errorf("decode share %d: %w", s.Index, err)
		}
		decodedShares[i] = decoded
	}

	// Combine shares
	key, err := shamir.Combine(decodedShares)
	if err != nil {
		return nil, fmt.Errorf("combine shares: %w", err)
	}

	return key, nil
}

// ExportShares writes Shamir shares to individual JSON files.
// Useful for distributing shares to different locations or people.
func ExportShares(shares []ShamirShare, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	for _, share := range shares {
		filename := filepath.Join(outputDir, fmt.Sprintf("share-%d.json", share.Index))
		data, err := json.MarshalIndent(share, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal share %d: %w", share.Index, err)
		}

		if err := os.WriteFile(filename, data, 0600); err != nil {
			return fmt.Errorf("write share %d: %w", share.Index, err)
		}
	}

	return nil
}

// ImportShare reads a single Shamir share from a JSON file.
func ImportShare(filename string) (*ShamirShare, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("read share file: %w", err)
	}

	var share ShamirShare
	if err := json.Unmarshal(data, &share); err != nil {
		return nil, fmt.Errorf("parse share: %w", err)
	}

	return &share, nil
}

// RestoreWithShares restores a backup using Shamir shares instead of the master password.
func (m *Manager) RestoreWithShares(backupFilename string, shares []ShamirShare, vaultPath string) error {
	// Combine shares to get backup key
	key, err := CombineBackupKey(shares)
	if err != nil {
		return err
	}

	backupPath := filepath.Join(m.backupDir, backupFilename)

	// Read encrypted backup
	ciphertext, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("read backup: %w", err)
	}

	// Decrypt
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	vaultData, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("decrypt backup: %w", err)
	}

	// Write to vault path
	if err := os.WriteFile(vaultPath, vaultData, 0600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}

	return nil
}
