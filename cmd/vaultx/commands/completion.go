package commands

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

func cmdCompletion() *cobra.Command {
	var overwrite bool

	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Install shell completion (zsh, bash, fish, powershell)",
		Long: strings.TrimSpace(`Install vaultx shell completion directly from the binary.

Auto-detects your shell when no argument is given. Writes the completion
script to a user-local path and adds a source line to your shell profile
(.zshrc, .bashrc, etc.).`),
		Example: "  vaultx completion           # auto-detect shell and install\n" +
			"  vaultx completion zsh        # install for zsh\n" +
			"  vaultx completion bash       # install for bash\n" +
			"  vaultx completion fish       # install for fish\n" +
			"  vaultx completion powershell # install for PowerShell",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"zsh", "bash", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			shellArg := "auto"
			if len(args) == 1 {
				shellArg = args[0]
			}

			selectedShell, err := detectShell(shellArg)
			if err != nil {
				return err
			}

			script, outPath, rcPath, rcSnippet, err := buildCompletionArtifacts(cmd.Root(), selectedShell)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("create completion dir: %w", err)
			}
			if _, statErr := os.Stat(outPath); statErr == nil && !overwrite {
				return fmt.Errorf("completion file already exists at %s (use --overwrite to replace)", outPath)
			}
			if err := os.WriteFile(outPath, script, 0o644); err != nil {
				return fmt.Errorf("write completion file: %w", err)
			}

			rcUpdated := false
			if rcPath != "" && rcSnippet != "" {
				if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
					return fmt.Errorf("create profile dir: %w", err)
				}
				updated, err := ensureCompletionSnippet(rcPath, rcSnippet)
				if err != nil {
					return fmt.Errorf("update profile: %w", err)
				}
				rcUpdated = updated
			}

			ux := uxFor(cmd)
			ok := icon(ux.Emoji, "ok")
			info := icon(ux.Emoji, "info")
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Shell:           %s\n", ok, selectedShell)
			fmt.Fprintf(cmd.OutOrStdout(), "%s  Completion file: %s\n", ok, outPath)
			if rcPath != "" {
				if rcUpdated {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  Profile updated: %s\n", ok, rcPath)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  Profile already configured: %s\n", info, rcPath)
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s  Restart your shell or run: source %s\n", info, rcPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&overwrite, "overwrite", false, "overwrite existing completion file")
	return cmd
}

func detectShell(shellFlag string) (string, error) {
	v := strings.TrimSpace(strings.ToLower(shellFlag))
	if v != "" && v != "auto" {
		switch v {
		case "zsh", "bash", "fish", "powershell":
			return v, nil
		default:
			return "", fmt.Errorf("unsupported shell %q — choose: zsh, bash, fish, powershell", shellFlag)
		}
	}
	if runtime.GOOS == "windows" {
		return "powershell", nil
	}
	base := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	switch strings.ToLower(base) {
	case "zsh", "bash", "fish":
		return strings.ToLower(base), nil
	}
	return "", errors.New("could not detect shell; pass shell name: vaultx completion zsh|bash|fish|powershell")
}

func buildCompletionArtifacts(root *cobra.Command, shell string) (script []byte, outPath, rcPath, rcSnippet string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", "", "", err
	}

	var buf bytes.Buffer
	switch shell {
	case "zsh":
		if err := root.GenZshCompletion(&buf); err != nil {
			return nil, "", "", "", err
		}
		out := filepath.Join(home, ".vaultx", "completions", "_vaultx")
		rc := filepath.Join(home, ".zshrc")
		snippet := `# >>> vaultx completion >>>
fpath=("$HOME/.vaultx/completions" $fpath)
autoload -Uz compinit
compinit
# <<< vaultx completion <<<`
		return buf.Bytes(), out, rc, snippet, nil

	case "bash":
		if err := root.GenBashCompletionV2(&buf, true); err != nil {
			return nil, "", "", "", err
		}
		out := filepath.Join(home, ".local", "share", "bash-completion", "completions", "vaultx")
		rc := filepath.Join(home, ".bashrc")
		snippet := `# >>> vaultx completion >>>
if [ -f "$HOME/.local/share/bash-completion/completions/vaultx" ]; then
  . "$HOME/.local/share/bash-completion/completions/vaultx"
fi
# <<< vaultx completion <<<`
		return buf.Bytes(), out, rc, snippet, nil

	case "fish":
		if err := root.GenFishCompletion(&buf, true); err != nil {
			return nil, "", "", "", err
		}
		out := filepath.Join(home, ".config", "fish", "completions", "vaultx.fish")
		// fish sources completions dir automatically — no rc update needed.
		return buf.Bytes(), out, "", "", nil

	case "powershell":
		if err := root.GenPowerShellCompletionWithDesc(&buf); err != nil {
			return nil, "", "", "", err
		}
		out := filepath.Join(home, ".vaultx", "completions", "vaultx.ps1")
		rcPath := detectPowerShellProfile(home)
		snippet := `# >>> vaultx completion >>>
if (Test-Path "$HOME/.vaultx/completions/vaultx.ps1") {
  . "$HOME/.vaultx/completions/vaultx.ps1"
}
# <<< vaultx completion <<<`
		return buf.Bytes(), out, rcPath, snippet, nil

	default:
		return nil, "", "", "", fmt.Errorf("unsupported shell: %s", shell)
	}
}

func detectPowerShellProfile(home string) string {
	out, err := exec.Command("pwsh", "-NoProfile", "-Command", "$PROFILE.CurrentUserAllHosts").Output()
	if err == nil {
		if p := strings.TrimSpace(string(out)); p != "" {
			return p
		}
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
	}
	return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
}

// ensureCompletionSnippet appends or replaces the vaultx completion block in rcPath.
func ensureCompletionSnippet(path, snippet string) (bool, error) {
	const start = "# >>> vaultx completion >>>"
	const end = "# <<< vaultx completion <<<"

	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	content := string(b)

	startIdx := strings.Index(content, start)
	endIdx := strings.Index(content, end)
	if startIdx >= 0 && endIdx > startIdx {
		// Replace existing block.
		blockEnd := endIdx + len(end)
		newContent := content[:startIdx] + snippet + content[blockEnd:]
		if newContent == content {
			return false, nil
		}
		return true, os.WriteFile(path, []byte(strings.TrimLeft(newContent, "\n")+"\n"), 0o644)
	}

	// No existing block.
	if strings.Contains(content, snippet) {
		return false, nil
	}
	if strings.TrimSpace(content) == "" {
		return true, os.WriteFile(path, []byte(snippet+"\n"), 0o644)
	}
	updated := strings.TrimRight(content, "\n") + "\n\n" + snippet + "\n"
	return true, os.WriteFile(path, []byte(updated), 0o644)
}
