// Package mfa provides TOTP (Time-based One-Time Password) multi-factor authentication.
package mfa

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/skip2/go-qrcode"
)

const (
	issuer  = "vaultx"
	account = "vaultx-vault"
)

// Config holds MFA configuration stored in ~/.vaultx/mfa.json
type Config struct {
	Enabled       bool      `json:"enabled"`
	Secret        string    `json:"secret"`          // Base32-encoded TOTP secret
	RecoveryCodes []string  `json:"recovery_codes"`  // One-time recovery codes
	CreatedAt     time.Time `json:"created_at"`
}

// Manager handles MFA setup, validation, and recovery.
type Manager struct {
	configPath string
}

// New creates an MFA manager with the given config path.
func New(configPath string) *Manager {
	return &Manager{configPath: configPath}
}

// LoadConfig reads the MFA configuration from disk.
func (m *Manager) LoadConfig() (*Config, error) {
	data, err := os.ReadFile(m.configPath)
	if os.IsNotExist(err) {
		return &Config{Enabled: false}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read mfa config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse mfa config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes the MFA configuration to disk.
func (m *Manager) SaveConfig(cfg *Config) error {
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create mfa config dir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal mfa config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("write mfa config: %w", err)
	}
	return nil
}

// Enable generates a new TOTP secret and recovery codes.
// Returns the secret, QR code URL, and recovery codes.
func (m *Manager) Enable() (secret, qrURL string, recoveryCodes []string, err error) {
	// Generate TOTP secret (32-byte random, base32-encoded)
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", nil, fmt.Errorf("generate secret: %w", err)
	}
	secret = base32.StdEncoding.EncodeToString(secretBytes)
	secret = strings.TrimRight(secret, "=") // Remove padding

	// Generate OTP auth URL for QR code
	qrURL = fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s",
		issuer, account, secret, issuer)

	// Generate 10 recovery codes (6 characters each, alphanumeric)
	recoveryCodes, err = generateRecoveryCodes(10)
	if err != nil {
		return "", "", nil, fmt.Errorf("generate recovery codes: %w", err)
	}

	// Save config
	cfg := &Config{
		Enabled:       true,
		Secret:        secret,
		RecoveryCodes: recoveryCodes,
		CreatedAt:     time.Now(),
	}
	if err := m.SaveConfig(cfg); err != nil {
		return "", "", nil, err
	}

	return secret, qrURL, recoveryCodes, nil
}

// Disable turns off MFA and removes the configuration.
func (m *Manager) Disable() error {
	cfg := &Config{Enabled: false}
	return m.SaveConfig(cfg)
}

// Validate checks a TOTP code or recovery code.
// Returns true if the code is valid. If a recovery code is used, it is consumed.
func (m *Manager) Validate(code string) (bool, error) {
	cfg, err := m.LoadConfig()
	if err != nil {
		return false, err
	}

	if !cfg.Enabled {
		return false, fmt.Errorf("MFA not enabled")
	}

	// Try TOTP code first
	valid := totp.Validate(code, cfg.Secret)
	if valid {
		return true, nil
	}

	// Try recovery codes
	for i, rc := range cfg.RecoveryCodes {
		if rc == code {
			// Consume the recovery code (remove it)
			cfg.RecoveryCodes = append(cfg.RecoveryCodes[:i], cfg.RecoveryCodes[i+1:]...)
			if err := m.SaveConfig(cfg); err != nil {
				return false, fmt.Errorf("consume recovery code: %w", err)
			}
			return true, nil
		}
	}

	return false, nil
}

// GenerateQRCode returns a QR code image (PNG) for the given OTP URL.
func GenerateQRCode(otpURL string) ([]byte, error) {
	return qrcode.Encode(otpURL, qrcode.Medium, 256)
}

// RenderQRTerminal renders a QR code as ASCII art in the terminal.
func RenderQRTerminal(otpURL string) (string, error) {
	qr, err := qrcode.New(otpURL, qrcode.Medium)
	if err != nil {
		return "", err
	}
	return qr.ToSmallString(false), nil
}

// generateRecoveryCodes creates n random alphanumeric codes of length 6.
// Format: XXXX-XXXX (4 chars, dash, 4 chars)
func generateRecoveryCodes(n int) ([]string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // Exclude ambiguous chars (0, O, 1, I)
	codes := make([]string, n)

	for i := 0; i < n; i++ {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}

		code := make([]byte, 8)
		for j := 0; j < 8; j++ {
			code[j] = chars[int(b[j])%len(chars)]
		}
		codes[i] = fmt.Sprintf("%s-%s", string(code[:4]), string(code[4:]))
	}

	return codes, nil
}

// IsEnabled checks if MFA is enabled.
func (m *Manager) IsEnabled() (bool, error) {
	cfg, err := m.LoadConfig()
	if err != nil {
		return false, err
	}
	return cfg.Enabled, nil
}

// GetRecoveryCodes returns the remaining recovery codes.
func (m *Manager) GetRecoveryCodes() ([]string, error) {
	cfg, err := m.LoadConfig()
	if err != nil {
		return nil, err
	}
	return cfg.RecoveryCodes, nil
}
