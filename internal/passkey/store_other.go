//go:build !darwin && !linux && !windows

package passkey

// Load always returns ("", false) on unsupported platforms.
func Load() (string, bool) { return "", false }

// Store is a no-op on unsupported platforms.
func Store(_ string, _ bool) error { return nil }

// BiometricAvailable always returns false on unsupported platforms.
func BiometricAvailable() (bool, string) { return false, "biometric unlock not available on this platform" }

// BiometricConfigured always returns false on unsupported platforms.
func BiometricConfigured() bool { return false }

// Clear is a no-op on unsupported platforms.
func Clear() {}
