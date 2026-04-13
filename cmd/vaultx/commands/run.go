package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
		Use:                "run -- <cmd> [args...]",
		Short:              "Resolve vaultx.env and exec a command with secrets injected",
		DisableFlagParsing: true, // everything after -- goes to the child
		RunE: func(cmd *cobra.Command, args []string) error {
			// Strip leading "--" if present.
			if len(args) > 0 && args[0] == "--" {
				args = args[1:]
			}
			if len(args) == 0 {
				return fmt.Errorf("usage: vaultx run -- <cmd> [args...]")
			}

			if err := requireUnlocked(); err != nil {
				return err
			}

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
				return fmt.Errorf("command not found: %s", args[0])
			}
			return syscall.Exec(binary, args, childEnv)
		},
	}
}

func cmdImport() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import credentials from an external password manager",
		Long: `Import credentials from a CSV or JSON export.

Supported formats (auto-detected if --format omitted):
  google     Google Password Manager CSV
  1password  1Password CSV
  bitwarden  Bitwarden JSON
  lastpass   LastPass CSV
  samsung    Samsung Pass CSV
  mcafee     McAfee True Key CSV
  dashlane   Dashlane CSV
  keeper     Keeper CSV
  csv        Generic CSV
  vaultx     vaultx JSON backup`,
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
