//go:build !darwin
// +build !darwin

package keychain

import (
	"fmt"
	"os"
	"path/filepath"
)

// Fallback to file-based storage on non-Darwin platforms
// TODO: Implement Secret Service for Linux, Credential Manager for Windows

var fallbackDir = filepath.Join(os.Getenv("HOME"), ".vaultx")

func storeImpl(service, account, secret string) error {
	if err := os.MkdirAll(fallbackDir, 0700); err != nil {
		return fmt.Errorf("create keychain dir: %w", err)
	}

	tokenPath := filepath.Join(fallbackDir, fmt.Sprintf("%s-%s.token", service, account))
	if err := os.WriteFile(tokenPath, []byte(secret), 0600); err != nil {
		return fmt.Errorf("write token: %w", err)
	}
	return nil
}

func loadImpl(service, account string) (string, error) {
	tokenPath := filepath.Join(fallbackDir, fmt.Sprintf("%s-%s.token", service, account))
	data, err := os.ReadFile(tokenPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func deleteImpl(service, account string) error {
	tokenPath := filepath.Join(fallbackDir, fmt.Sprintf("%s-%s.token", service, account))
	err := os.Remove(tokenPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
