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

// entry is a single record in the vault file.
type entry struct {
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// vaultFile is the JSON structure written to disk (encrypted).
type vaultFile struct {
	Version int               `json:"version"`
	Entries map[string]entry  `json:"entries"`
}

// Provider is the local encrypted file vault.
type Provider struct {
	id       string
	path     string

	mu       sync.RWMutex
	unlocked bool
	key      string // master password, held in memory after unlock
	data     *vaultFile
}

// New creates a local vault provider. The vault is locked until Unlock is called.
func New(id, path string) *Provider {
	return &Provider{id: id, path: path}
}

func (p *Provider) ID() string { return p.id }

// Unlock decrypts the vault file with the master password and holds it in memory.
func (p *Provider) Unlock(password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := p.load(password)
	if err != nil {
		return err
	}

	p.key = password
	p.data = data
	p.unlocked = true
	return nil
}

// Lock clears the in-memory key and data.
func (p *Provider) Lock() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.key = ""
	p.data = nil
	p.unlocked = false
}

// IsLocked reports whether the vault is currently locked.
func (p *Provider) IsLocked() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return !p.unlocked
}

// Init creates a new empty vault file encrypted with password.
// Returns an error if the file already exists.
func (p *Provider) Init(password string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, err := os.Stat(p.path); err == nil {
		return fmt.Errorf("vault already exists at %s", p.path)
	}

	vf := &vaultFile{Version: 1, Entries: map[string]entry{}}
	if err := p.save(vf, password); err != nil {
		return err
	}

	p.key = password
	p.data = vf
	p.unlocked = true
	return nil
}

func (p *Provider) Health(_ context.Context) error {
	if p.IsLocked() {
		return &providers.ErrLocked{Provider: p.id}
	}
	return nil
}

func (p *Provider) Get(_ context.Context, path string) (providers.Secret, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.unlocked {
		return providers.Secret{}, &providers.ErrLocked{Provider: p.id}
	}

	e, ok := p.data.Entries[path]
	if !ok {
		return providers.Secret{}, &providers.ErrNotFound{Provider: p.id, Path: path}
	}

	return providers.Secret{
		Key:       path,
		Value:     e.Value,
		Provider:  p.id,
		UpdatedAt: e.UpdatedAt,
	}, nil
}

func (p *Provider) List(_ context.Context, prefix string) ([]providers.Secret, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if !p.unlocked {
		return nil, &providers.ErrLocked{Provider: p.id}
	}

	var out []providers.Secret
	for k, e := range p.data.Entries {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out = append(out, providers.Secret{
				Key:       k,
				Provider:  p.id,
				UpdatedAt: e.UpdatedAt,
				// Value intentionally omitted from list — caller must Get explicitly
			})
		}
	}
	return out, nil
}

// Set stores or updates a secret. The vault is re-encrypted and written to disk.
func (p *Provider) Set(_ context.Context, path, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.unlocked {
		return &providers.ErrLocked{Provider: p.id}
	}

	p.data.Entries[path] = entry{Value: value, UpdatedAt: time.Now().UTC()}
	return p.save(p.data, p.key)
}

// Delete removes a secret and re-encrypts the vault file.
func (p *Provider) Delete(_ context.Context, path string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.unlocked {
		return &providers.ErrLocked{Provider: p.id}
	}

	if _, ok := p.data.Entries[path]; !ok {
		return &providers.ErrNotFound{Provider: p.id, Path: path}
	}

	delete(p.data.Entries, path)
	return p.save(p.data, p.key)
}

// load reads and decrypts the vault file from disk.
func (p *Provider) load(password string) (*vaultFile, error) {
	raw, err := os.ReadFile(p.path)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("vault not found at %s — run: vaultx init", p.path)
	}
	if err != nil {
		return nil, fmt.Errorf("read vault: %w", err)
	}

	plaintext, err := decrypt(raw, password)
	if err != nil {
		return nil, err
	}

	var vf vaultFile
	if err := json.Unmarshal(plaintext, &vf); err != nil {
		return nil, fmt.Errorf("parse vault: %w", err)
	}
	return &vf, nil
}

// save encrypts the vault and writes it atomically (write to temp, rename).
func (p *Provider) save(vf *vaultFile, password string) error {
	plaintext, err := json.Marshal(vf)
	if err != nil {
		return fmt.Errorf("marshal vault: %w", err)
	}

	ciphertext, err := encrypt(plaintext, password)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return fmt.Errorf("create vault dir: %w", err)
	}

	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, ciphertext, 0600); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}
	return os.Rename(tmp, p.path)
}
