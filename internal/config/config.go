package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the top-level vaultx configuration (~/.vaultx/config.toml).
type Config struct {
	Vault     VaultConfig      `toml:"vault"`
	Providers []ProviderConfig `toml:"providers"`
	Daemon    DaemonConfig     `toml:"daemon"`
}

type VaultConfig struct {
	Path string `toml:"path"` // default: ~/.vaultx/vault.enc
	KDF  string `toml:"kdf"`  // "argon2id" (default) | "pbkdf2"
}

type ProviderConfig struct {
	ID       string `toml:"id"`
	Type     string `toml:"type"`    // local | onepassword | hashicorp | aws | env
	Default  bool   `toml:"default"` // used when no provider prefix in reference

	// onepassword
	Account string `toml:"account"`
	Vault   string `toml:"vault"`

	// hashicorp
	Address  string `toml:"address"`
	TokenEnv string `toml:"token_env"` // env var that holds the token

	// aws
	Region  string `toml:"region"`
	RoleARN string `toml:"role_arn"`
}

type DaemonConfig struct {
	Port int `toml:"port"` // default: 7474
}

// DefaultPath returns the canonical config file location.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vaultx", "config.toml")
}

// DefaultVaultPath returns the default encrypted vault file location.
func DefaultVaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vaultx", "vault.enc")
}

// Load reads config from path. Returns a default config if the file does not exist.
func Load(path string) (*Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if cfg.Vault.Path == "" {
		cfg.Vault.Path = DefaultVaultPath()
	}
	if cfg.Vault.KDF == "" {
		cfg.Vault.KDF = "argon2id"
	}
	if cfg.Daemon.Port == 0 {
		cfg.Daemon.Port = 7474
	}

	return cfg, nil
}

func defaults() *Config {
	return &Config{
		Vault: VaultConfig{
			Path: DefaultVaultPath(),
			KDF:  "argon2id",
		},
		Daemon: DaemonConfig{Port: 7474},
		Providers: []ProviderConfig{
			{ID: "local", Type: "local", Default: true},
		},
	}
}
