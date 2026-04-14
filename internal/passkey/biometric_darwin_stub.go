//go:build darwin && !cgo

package passkey

func verifyBiometricUnlock() bool {
	// Built without CGO — biometric authentication unavailable.
	return false
}

func isBiometricAvailable() bool {
	return false
}
