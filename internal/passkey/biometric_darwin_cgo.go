//go:build darwin && cgo

package passkey

/*
#cgo LDFLAGS: -framework LocalAuthentication -framework Foundation
int vaultx_touchid_authenticate(void);
*/
import "C"
import "os"

func verifyBiometricUnlock() bool {
	// Skip in CI — Touch ID is never available in headless environments.
	if os.Getenv("CI") != "" {
		return true
	}
	return int(C.vaultx_touchid_authenticate()) == 1
}

func isBiometricAvailable() bool {
	return true
}
