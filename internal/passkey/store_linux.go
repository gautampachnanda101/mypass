//go:build linux

package passkey

// Load always returns ("", false) on Linux — no system keychain integration.
func Load() (string, bool) { return "", false }

// Store is a no-op on Linux.
func Store(_ string, _ bool) error { return nil }

// BiometricAvailable always returns false on Linux.
func BiometricAvailable() (bool, string) { return false, "biometric unlock not available on Linux" }

// BiometricConfigured always returns false on Linux.
func BiometricConfigured() bool { return false }

// BiometricEntryExists always returns false on Linux.
func BiometricEntryExists() bool { return false }

// Clear is a no-op on Linux.
func Clear() {}
