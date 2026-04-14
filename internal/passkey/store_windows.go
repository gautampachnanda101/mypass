//go:build windows

package passkey

// Load always returns ("", false) on Windows — no system keychain integration.
func Load() (string, bool) { return "", false }

// Store is a no-op on Windows.
func Store(_ string, _ bool) error { return nil }

// BiometricAvailable always returns false on Windows.
func BiometricAvailable() (bool, string) { return false, "biometric unlock not available on Windows" }

// BiometricConfigured always returns false on Windows.
func BiometricConfigured() bool { return false }

// Clear is a no-op on Windows.
func Clear() {}
