// Package importexport handles reading and writing credentials from/to external
// password manager formats (CSV, JSON) and the vaultx-native JSON backup format.
//
// Supported import sources (auto-detected):
//   - Google Password Manager CSV
//   - 1Password CSV / 1PUX JSON
//   - Bitwarden JSON export
//   - LastPass CSV
//   - Samsung Pass CSV
//   - McAfee True Key CSV
//   - Dashlane CSV
//   - Keeper CSV
//   - Generic CSV (name/title, username/email, password, url)
//   - vaultx JSON backup
//
// Supported export targets:
//   - vaultx JSON (lossless round-trip)
//   - Generic CSV (importable by most managers)
//   - Bitwarden JSON (for migration to Bitwarden / Vaultwarden)
package importexport

import "time"

// Record is the normalised credential representation used during import/export.
// All fields are plaintext — the caller is responsible for encrypting before storage.
type Record struct {
	Name     string    `json:"name"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	URL      string    `json:"url"`
	Notes    string    `json:"notes"`
	TOTPSeed string    `json:"totp_seed,omitempty"` // base32 TOTP secret if present
	Tags     []string  `json:"tags,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// Format identifies the import/export file format.
type Format string

const (
	FormatAuto       Format = "auto"        // detect from file content
	FormatVaultx     Format = "vaultx"      // vaultx native JSON backup
	FormatCSVGeneric Format = "csv"          // generic CSV
	FormatGoogle     Format = "google"      // Google Password Manager CSV
	FormatOnePassword Format = "1password"  // 1Password CSV
	FormatBitwarden  Format = "bitwarden"   // Bitwarden JSON
	FormatLastPass   Format = "lastpass"    // LastPass CSV
	FormatSamsung    Format = "samsung"     // Samsung Pass CSV
	FormatMcAfee     Format = "mcafee"      // McAfee True Key CSV
	FormatDashlane   Format = "dashlane"    // Dashlane CSV
	FormatKeeper     Format = "keeper"      // Keeper CSV
)

// ImportResult summarises the outcome of an import operation.
type ImportResult struct {
	Total    int
	Imported int
	Skipped  int      // duplicate / blank password entries
	Errors   []string // per-row errors (non-fatal)
}
