package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/passkey"
)

// check is a single doctor result.
type check struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
	Fix      string `json:"fix,omitempty"`
}

// doctorReport is the full result set.
type doctorReport struct {
	Ready   bool    `json:"ready"`
	Checks  []check `json:"checks"`
	Elapsed string  `json:"elapsed"`
}

func cmdDoctor() *cobra.Command {
	var asJSON bool
	var fix bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check runtime dependencies and vault health",
		Long: strings.TrimSpace(`Run a comprehensive health check on your vaultx installation.

Each check is marked required or optional. All required checks must pass
for vaultx to function correctly.

Use --fix to auto-repair issues that can be resolved without user input
(creates missing directories, installs shell completion, etc.).`),
		Example: "  vaultx doctor\n" +
			"  vaultx doctor --fix\n" +
			"  vaultx doctor --json",
		RunE: func(cmd *cobra.Command, _ []string) error {
			start := time.Now()
			ux := uxFor(cmd)
			checks := runAllChecks(fix, ux)

			ready := true
			for _, c := range checks {
				if c.Required && !c.OK {
					ready = false
					break
				}
			}

			report := doctorReport{
				Ready:   ready,
				Checks:  checks,
				Elapsed: time.Since(start).Round(time.Millisecond).String(),
			}

			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}

			printReport(cmd, report, ux)

			if !ready {
				return fmt.Errorf("required checks failed — run: vaultx doctor --fix")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, "json", false, "output report as JSON")
	cmd.Flags().BoolVar(&fix, "fix", false, "auto-repair issues that can be resolved without user input")
	return cmd
}

func runAllChecks(fix bool, ux terminalUX) []check {
	home, _ := os.UserHomeDir()
	vaultxDir := filepath.Join(home, ".vaultx")
	vaultFile := filepath.Join(vaultxDir, "vault.enc")
	configFile := filepath.Join(vaultxDir, "config.toml")

	var checks []check

	// ── Core ──────────────────────────────────────────────────────────────────

	// Data directory
	if _, err := os.Stat(vaultxDir); err != nil {
		detail := fmt.Sprintf("~/.vaultx/ does not exist")
		fixCmd := "vaultx init"
		if fix {
			if mkErr := os.MkdirAll(vaultxDir, 0o700); mkErr == nil {
				checks = append(checks, check{"vaultx data dir", true, true, "created ~/.vaultx/", ""})
			} else {
				checks = append(checks, check{"vaultx data dir", false, true, detail, fixCmd})
			}
		} else {
			checks = append(checks, check{"vaultx data dir", false, true, detail, fixCmd})
		}
	} else {
		checks = append(checks, check{"vaultx data dir", true, true, vaultxDir, ""})
	}

	// Vault file
	if _, err := os.Stat(vaultFile); err != nil {
		checks = append(checks, check{
			"vault initialized", false, true,
			"~/.vaultx/vault.enc not found",
			"vaultx init",
		})
	} else {
		checks = append(checks, check{"vault initialized", true, true, vaultFile, ""})
	}

	// Config file (optional — defaults are used when absent)
	if _, err := os.Stat(configFile); err != nil {
		checks = append(checks, check{
			"config file", false, false,
			"~/.vaultx/config.toml not found (using defaults)",
			"",
		})
	} else {
		checks = append(checks, check{"config file", true, false, configFile, ""})
	}

	// ── macOS Keychain ────────────────────────────────────────────────────────

	if runtime.GOOS == "darwin" {
		if _, err := exec.LookPath("security"); err != nil {
			checks = append(checks, check{
				"security CLI", false, true,
				"macOS security CLI not found — keychain operations will fail",
				"Install Xcode Command Line Tools: xcode-select --install",
			})
		} else {
			checks = append(checks, check{"security CLI", true, true, "macOS keychain available", ""})
		}

		biometricAvail, reason := passkey.BiometricAvailable()
		checks = append(checks, check{
			"Touch ID available", biometricAvail, false,
			reason,
			func() string {
				if !biometricAvail {
					return ""
				}
				return "vaultx init --biometric"
			}(),
		})

		if biometricAvail {
			// BiometricEntryExists checks keychain entry presence without reading
			// the password value — avoids triggering a Touch ID / ACL prompt.
			configured := passkey.BiometricEntryExists()
			fixHint := ""
			detail := "Touch ID unlock is active"
			if !configured {
				detail = "Touch ID not configured for vault unlock"
				fixHint = "vaultx init --biometric  OR  vaultx unlock --biometric"
			}
			checks = append(checks, check{"Touch ID configured", configured, false, detail, fixHint})
		}
	}

	// ── Optional tooling ──────────────────────────────────────────────────────

	tools := []struct {
		bin     string
		name    string
		fixHint string
	}{
		{"docker", "Docker", "Install Docker Desktop: https://docs.docker.com/get-docker/"},
		{"kubectl", "kubectl", "Install kubectl: https://kubernetes.io/docs/tasks/tools/"},
		{"helm", "Helm", "Install Helm: https://helm.sh/docs/intro/install/"},
		{"op", "1Password CLI (op)", "Install op: https://developer.1password.com/docs/cli/"},
	}

	for _, t := range tools {
		if _, err := exec.LookPath(t.bin); err != nil {
			checks = append(checks, check{
				t.name, false, false,
				fmt.Sprintf("%s not found in PATH (needed for vaultx %s commands)", t.bin, strings.ToLower(t.name)),
				t.fixHint,
			})
		} else {
			path, _ := exec.LookPath(t.bin)
			checks = append(checks, check{t.name, true, false, path, ""})
		}
	}

	// ── Shell completion ──────────────────────────────────────────────────────

	completionInstalled := false
	completionDetail := "not installed"
	shell := filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	switch strings.ToLower(shell) {
	case "zsh":
		p := filepath.Join(home, ".vaultx", "completions", "_vaultx")
		if _, err := os.Stat(p); err == nil {
			completionInstalled = true
			completionDetail = p
		}
	case "bash":
		p := filepath.Join(home, ".local", "share", "bash-completion", "completions", "vaultx")
		if _, err := os.Stat(p); err == nil {
			completionInstalled = true
			completionDetail = p
		}
	case "fish":
		p := filepath.Join(home, ".config", "fish", "completions", "vaultx.fish")
		if _, err := os.Stat(p); err == nil {
			completionInstalled = true
			completionDetail = p
		}
	}

	fixHint := ""
	if !completionInstalled {
		fixHint = "vaultx completion"
		if fix {
			// Auto-run completion install.
			if err := runCompletionInstall(); err == nil {
				completionInstalled = true
				completionDetail = "installed by --fix"
				fixHint = ""
			}
		}
	}

	checks = append(checks, check{
		"shell completion", completionInstalled, false, completionDetail, fixHint,
	})

	return checks
}

// runCompletionInstall is called by --fix to install completion automatically.
func runCompletionInstall() error {
	shellName, err := detectShell("auto")
	if err != nil {
		return err
	}
	// We need a cobra root to generate completions. Use the package-level state.
	// Since we're inside RunE, state.vault is available but we don't have root here.
	// Fall back to spawning the binary.
	self, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(self, "completion", shellName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func printReport(cmd *cobra.Command, r doctorReport, ux terminalUX) {
	w := cmd.OutOrStdout()

	headIcon := icon(ux.Emoji, "warn")
	headColor := ansiBold + ansiYellow
	if r.Ready {
		headIcon = icon(ux.Emoji, "ok")
		headColor = ansiBold + ansiGreen
	}
	fmt.Fprintf(w, "%s  %s  (%s)\n\n",
		headIcon,
		style(ux.Color, headColor, fmt.Sprintf("Ready: %v", r.Ready)),
		style(ux.Color, ansiDim, r.Elapsed),
	)

	for _, c := range r.Checks {
		rowIcon := icon(ux.Emoji, "warn")
		nameColor := ansiYellow
		if c.OK {
			rowIcon = icon(ux.Emoji, "ok")
			nameColor = ansiGreen
		} else if c.Required {
			rowIcon = icon(ux.Emoji, "error")
			nameColor = ansiRed
		}

		req := style(ux.Color, ansiDim, "optional")
		if c.Required {
			req = style(ux.Color, ansiDim, "required")
		}

		fmt.Fprintf(w, "%s  %-28s [%s]  %s\n",
			rowIcon,
			style(ux.Color, nameColor, c.Name),
			req,
			style(ux.Color, ansiDim, c.Detail),
		)
		if !c.OK && c.Fix != "" {
			fmt.Fprintf(w, "   %s  Fix: %s\n",
				style(ux.Color, ansiDim, "→"),
				style(ux.Color, ansiCyan, c.Fix),
			)
		}
	}

	fmt.Fprintln(w)
}
