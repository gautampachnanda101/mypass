package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/importexport"
)

// resolveEnvFile finds, parses, and resolves the vaultx.env file, returning
// a KEY→value map ready for injection. Called by cmdRun and cmdShell.
func resolveEnvFile() (map[string]string, error) {
	var f *envfile.File
	var err error

	if globalFlags.envFile != "" {
		f, err = envfile.ParseFile(globalFlags.envFile)
	} else {
		f, err = envfile.FindAndParse()
	}
	if err != nil {
		return nil, fmt.Errorf("parse env file: %w", err)
	}
	if f == nil {
		return map[string]string{}, nil // no vaultx.env found — fine
	}

	return state.registry.Resolve(context.Background(), f)
}

func cmdRun() *cobra.Command {
	return &cobra.Command{
		Use:   "run -- <cmd> [args...]",
		Short: "Resolve vaultx.env and exec a command with secrets injected",
		Long: "Resolve vault: references in vaultx.env and exec the given command with\n" +
			"secrets injected into its environment. Uses syscall.Exec so the child\n" +
			"replaces the vaultx process — secrets exist only in process memory.\n\n" +
			"IMPORTANT: vaultx always runs the command from your CURRENT directory.\n" +
			"Change into your project directory before running:\n\n" +
			"  cd ~/projects/my-app\n" +
			"  vaultx run -- npm start\n\n" +
			"vaultx.env lookup order (first found wins):\n" +
			"  ./vaultx.env\n" +
			"  ./.vaultx.env\n" +
			"  ~/.vaultx/default.env\n\n" +
			"Plain values (no vault: prefix) pass through unchanged.\n" +
			"Override the env file with the global --env flag.",
		Example: "  cd ~/projects/my-app && vaultx run -- npm start\n" +
			"  cd ~/projects/api    && vaultx run -- go run ./cmd/server\n" +
			"  vaultx --env staging.env run -- ./server",
		DisableFlagParsing: true, // everything after -- goes to the child
		RunE: func(cmd *cobra.Command, args []string) error {
			// Allow --help / -h even with DisableFlagParsing.
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
			// Strip leading "--" separator if present.
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			if len(args) == 0 {
				return fmt.Errorf("no command given — usage: vaultx run -- <cmd> [args...]")
			}

			if err := requireUnlocked(); err != nil {
				return err
			}

			// Warn early if no vaultx.env will be used — avoids confusing child errors.
			warnIfNoEnvFile(cmd)

			resolved, err := resolveEnvFile()
			if err != nil {
				return err
			}

			// Build the child environment: start from current env, overlay resolved secrets.
			childEnv := os.Environ()
			for k, v := range resolved {
				childEnv = append(childEnv, k+"="+v)
			}

			// exec replaces the current process — secrets only ever live in memory.
			binary, err := exec.LookPath(args[0])
			if err != nil {
				return fmt.Errorf("%q not found in PATH — is it installed?", args[0])
			}
			return syscall.Exec(binary, args, childEnv)
		},
	}
}

// warnIfNoEnvFile prints a diagnostic when no vaultx.env will be used.
// This surfaces the most common mistake (wrong CWD) before the child process runs.
func warnIfNoEnvFile(cmd *cobra.Command) {
	if globalFlags.envFile != "" {
		return // explicit --env flag set, nothing to warn about
	}
	cwd, _ := os.Getwd()
	for _, name := range []string{"vaultx.env", ".vaultx.env"} {
		if _, err := os.Stat(filepath.Join(cwd, name)); err == nil {
			return // found one
		}
	}
	// Check ~/.vaultx/default.env too.
	if home, err := os.UserHomeDir(); err == nil {
		if _, err := os.Stat(filepath.Join(home, ".vaultx", "default.env")); err == nil {
			return
		}
	}
	ux := uxFor(cmd)
	warn := icon(ux.Emoji, "warn")
	info := icon(ux.Emoji, "info")
	fmt.Fprintf(os.Stderr, "%s  No vaultx.env found in %s\n", warn, cwd)
	fmt.Fprintf(os.Stderr, "%s  Secrets will not be injected. Are you in the right directory?\n", info)
	fmt.Fprintf(os.Stderr, "%s  Create one: echo 'MY_KEY=vault:local/myapp/key' > vaultx.env\n\n", info)
}

func cmdImport() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import credentials from an external password manager",
		Long: "Import credentials from a CSV or JSON export into the local vault.\n" +
			"The format is auto-detected from the file header; use --format to override.\n\n" +
			"Supported formats:\n" +
			"  google     Google Password Manager CSV\n" +
			"  1password  1Password CSV\n" +
			"  bitwarden  Bitwarden / Vaultwarden JSON\n" +
			"  lastpass   LastPass CSV\n" +
			"  samsung    Samsung Pass CSV\n" +
			"  mcafee     McAfee True Key CSV\n" +
			"  dashlane   Dashlane CSV\n" +
			"  keeper     Keeper CSV\n" +
			"  csv        Generic CSV (name/username/password/url)\n" +
			"  vaultx     vaultx JSON backup (lossless round-trip)",
		Example: `  vaultx import ~/Downloads/bitwarden-export.json
  vaultx import ~/Downloads/google-passwords.csv
  vaultx import ~/Downloads/export.csv --format 1password`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			fh, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer fh.Close()

			format := importexport.Format(formatFlag)
			records, err := importexport.Import(fh, format)
			if err != nil {
				return err
			}

			ctx := context.Background()
			imported := 0
			for _, r := range records {
				if r.Password == "" {
					continue
				}
				path := sanitisePath(r.Name) + "/password"
				if err := state.vault.Set(ctx, path, r.Password); err != nil {
					fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
					continue
				}
				if r.Username != "" {
					_ = state.vault.Set(ctx, sanitisePath(r.Name)+"/username", r.Username)
				}
				if r.URL != "" {
					_ = state.vault.Set(ctx, sanitisePath(r.Name)+"/url", r.URL)
				}
				imported++
			}

			fmt.Fprintf(os.Stderr, "Imported %d/%d credentials\n", imported, len(records))
			return nil
		},
	}

	cmd.Flags().StringVarP(&formatFlag, "format", "f", "auto", "import format (auto|google|1password|bitwarden|lastpass|samsung|mcafee|dashlane|keeper|csv|vaultx)")
	return cmd
}

func cmdExport() *cobra.Command {
	var formatFlag string
	var outputFlag string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export credentials to a file",
		Long: "Export all secrets from the local vault to a file.\n" +
			"Use 'vaultx' format for a lossless backup; 'bitwarden' or 'csv' for migration.\n\n" +
			"Supported formats: vaultx, csv, bitwarden",
		Example: `  vaultx export -f vaultx -o backup.json       # full lossless backup
  vaultx export -f bitwarden -o bw-backup.json
  vaultx export -f csv -o secrets.csv`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			secrets, err := state.vault.List(context.Background(), "")
			if err != nil {
				return err
			}

			var records []importexport.Record
			for _, s := range secrets {
				full, err := state.vault.Get(context.Background(), s.Key)
				if err != nil {
					continue
				}
				records = append(records, importexport.Record{
					Name:      s.Key,
					Password:  full.Value,
					UpdatedAt: s.UpdatedAt,
				})
			}

			out := os.Stdout
			if outputFlag != "" {
				f, err := os.Create(outputFlag)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}

			if err := importexport.Export(out, records, importexport.Format(formatFlag)); err != nil {
				return err
			}
			if outputFlag != "" {
				fmt.Fprintf(os.Stderr, "Exported %d secrets to %s\n", len(records), outputFlag)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&formatFlag, "format", "f", "vaultx", "export format (vaultx|csv|bitwarden)")
	cmd.Flags().StringVarP(&outputFlag, "output", "o", "", "output file (default: stdout)")
	return cmd
}

// sanitisePath converts a credential name into a safe vault path segment.
func sanitisePath(name string) string {
	var b []byte
	for _, c := range []byte(name) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b = append(b, c)
		case c == ' ', c == '-', c == '.':
			b = append(b, '/')
		default:
			b = append(b, '_')
		}
	}
	// Collapse consecutive slashes.
	s := string(b)
	for len(s) != len(replaceAll(s, "//", "/")) {
		s = replaceAll(s, "//", "/")
	}
	return s
}

func replaceAll(s, old, new string) string {
	result := ""
	for {
		i := len(s)
		for j := 0; j+len(old) <= len(s); j++ {
			if s[j:j+len(old)] == old {
				i = j
				break
			}
		}
		if i == len(s) {
			return result + s
		}
		result += s[:i] + new
		s = s[i+len(old):]
	}
}
