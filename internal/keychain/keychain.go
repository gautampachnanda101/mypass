// Package keychain provides secure storage for daemon tokens using OS-native keychains.
// On macOS, uses the Keychain; on Linux, uses Secret Service; on Windows, uses Credential Manager.
package keychain

const (
	// Service identifier for vaultx daemon tokens
	daemonTokenService = "vaultx-daemon"
	daemonTokenAccount = "session-token"
)

// StoreDaemonToken saves the daemon session token to the OS keychain.
func StoreDaemonToken(token string) error {
	return store(daemonTokenService, daemonTokenAccount, token)
}

// LoadDaemonToken retrieves the daemon session token from the OS keychain.
// Returns the token and true if found, empty string and false otherwise.
func LoadDaemonToken() (string, bool) {
	token, err := load(daemonTokenService, daemonTokenAccount)
	if err != nil {
		return "", false
	}
	return token, true
}

// DeleteDaemonToken removes the daemon session token from the OS keychain.
func DeleteDaemonToken() error {
	return delete(daemonTokenService, daemonTokenAccount)
}

// store saves a secret to the keychain (OS-specific implementation).
func store(service, account, secret string) error {
	return storeImpl(service, account, secret)
}

// load retrieves a secret from the keychain (OS-specific implementation).
func load(service, account string) (string, error) {
	return loadImpl(service, account)
}

// delete removes a secret from the keychain (OS-specific implementation).
func delete(service, account string) error {
	return deleteImpl(service, account)
}
