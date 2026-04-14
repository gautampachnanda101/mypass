//go:build darwin

package passkey

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const keychainTimeout = 20 * time.Second

const (
	keychainService          = "vaultx.master"
	keychainServiceBiometric = "vaultx.master.biometric"
	keychainAccount          = "default"
)

// Load returns the stored master password, gating behind Touch ID when the
// biometric entry is present. Returns ("", false) if nothing is stored.
func Load() (string, bool) {
	if v, ok := readKeychainService(keychainServiceBiometric); ok {
		if !verifyBiometricUnlock() {
			fmt.Fprintln(os.Stderr, "Touch ID authentication failed.")
			return "", false
		}
		return v, true
	}
	return readKeychainService(keychainService)
}

// Store writes the master password to the keychain, using the biometric
// service when biometric=true. On biometric write failure it falls back to
// plain keychain and prints a warning.
func Store(password string, biometric bool) error {
	pass := strings.TrimSpace(password)
	if pass == "" {
		return nil
	}
	// Never touch the real keychain in CI.
	if strings.TrimSpace(os.Getenv("CI")) != "" {
		return nil
	}

	target := keychainService
	other := keychainServiceBiometric
	if biometric {
		target = keychainServiceBiometric
		other = keychainService
	}

	if err := writeKeychainEntry(target, pass); err != nil {
		if biometric {
			fmt.Fprintln(os.Stderr, "WARN: biometric keychain write failed — storing without Touch ID. Run: vaultx unlock --biometric to retry.")
			return writeKeychainEntry(keychainService, pass)
		}
		return err
	}
	_ = deleteKeychainService(other)
	return nil
}

// BiometricAvailable reports whether Touch ID / macOS keychain is usable.
func BiometricAvailable() (bool, string) {
	if _, err := exec.LookPath("security"); err != nil {
		return false, "security CLI not available"
	}
	return true, "macOS keychain available"
}

// BiometricConfigured reports whether a biometric-gated entry already exists.
func BiometricConfigured() bool {
	_, ok := readKeychainService(keychainServiceBiometric)
	return ok
}

// BiometricEntryExists is a fast non-blocking check that returns true when the
// biometric keychain entry is present. Unlike BiometricConfigured it does NOT
// retrieve the password, so it never triggers a Touch ID / ACL prompt.
func BiometricEntryExists() bool {
	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()
	err := exec.CommandContext(ctx, "security",
		"find-generic-password", "-s", keychainServiceBiometric, "-a", keychainAccount).Run()
	return err == nil
}

// Clear removes both keychain entries (plain and biometric).
func Clear() {
	_ = deleteKeychainService(keychainService)
	_ = deleteKeychainService(keychainServiceBiometric)
}

// writeKeychainEntry stores password under service, ACL-pinned to the current
// binary. If the item already exists it deletes the stale entry and retries.
func writeKeychainEntry(service, pass string) error {
	execPath, _ := os.Executable()

	args := []string{"add-generic-password", "-s", service, "-a", keychainAccount, "-w", pass, "-U"}
	if execPath != "" {
		args = append(args, "-T", execPath)
	}

	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "security", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// Stale entry or ACL conflict — delete and retry.
		_ = deleteKeychainService(service)
		ctx2, cancel2 := context.WithTimeout(context.Background(), keychainTimeout)
		defer cancel2()
		cmd2 := exec.CommandContext(ctx2, "security", args...)
		cmd2.Stderr = &stderr
		return cmd2.Run()
	}
	return nil
}

func readKeychainService(service string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "security",
		"find-generic-password", "-s", service, "-a", keychainAccount, "-w").Output()
	if err != nil {
		return "", false
	}
	v := strings.TrimSpace(string(out))
	if v == "" {
		return "", false
	}
	return v, true
}

func deleteKeychainService(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), keychainTimeout)
	defer cancel()
	return exec.CommandContext(ctx, "security",
		"delete-generic-password", "-s", service, "-a", keychainAccount).Run()
}
