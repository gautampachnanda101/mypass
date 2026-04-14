package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/gautampachnanda101/vaultx/internal/config"
	"github.com/gautampachnanda101/vaultx/internal/passkey"
	"github.com/gautampachnanda101/vaultx/internal/providers/local"
	"github.com/gautampachnanda101/vaultx/internal/providers/onepassword"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
)

// buildInfo is injected from main via SetBuildInfo.
var buildInfo struct {
	version string
	commit  string
	date    string
}

// SetBuildInfo is called from main() to pass in ldflags values.
func SetBuildInfo(version, commit, date string) {
	buildInfo.version = version
	buildInfo.commit = commit
	buildInfo.date = date
}

// globalFlags are shared across all subcommands.
var globalFlags struct {
	configPath string
	envFile    string
	colorMode  string
	emojiMode  string
}

// appState holds lazily-initialised shared objects.
type appState struct {
	cfg      *config.Config
	vault    *local.Provider
	registry *resolver.Registry
}

var state appState

// ── ANSI constants ─────────────────────────────────────────────────────────────

const (
	ansiReset   = "\x1b[0m"
	ansiDim     = "\x1b[2m"
	ansiBold    = "\x1b[1m"
	ansiCyan    = "\x1b[36m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiBlue    = "\x1b[34m"
	ansiRed     = "\x1b[31m"
	ansiMagenta = "\x1b[35m"
)

// ── Color / emoji detection ────────────────────────────────────────────────────

func displayMode(raw string) string {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "always", "never", "auto":
		return v
	default:
		return "auto"
	}
}

func supportsANSIColor(w io.Writer) bool {
	switch displayMode(globalFlags.colorMode) {
	case "always":
		return true
	case "never":
		return false
	}
	if strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")) != "" || strings.TrimSpace(os.Getenv("FORCE_COLOR")) != "" {
		return true
	}
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return true
	}
	if term.IsTerminal(int(f.Fd())) {
		return true
	}
	termProg := strings.ToLower(strings.TrimSpace(os.Getenv("TERM_PROGRAM")))
	if termProg == "vscode" || termProg == "apple_terminal" || termProg == "iterm.app" || termProg == "wezterm" {
		return true
	}
	return false
}

func supportsEmoji(w io.Writer) bool {
	switch displayMode(globalFlags.emojiMode) {
	case "always":
		return true
	case "never":
		return false
	}
	if strings.TrimSpace(os.Getenv("NO_EMOJI")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// style wraps s with the given ANSI escape code when enabled.
func style(enabled bool, code, s string) string {
	if !enabled {
		return s
	}
	return code + s + ansiReset
}

// terminalUX bundles color and emoji support flags for a command.
type terminalUX struct {
	Color bool
	Emoji bool
}

// uxFor returns the color/emoji capabilities for the given command's output.
func uxFor(cmd *cobra.Command) terminalUX {
	w := cmd.OutOrStdout()
	return terminalUX{Color: supportsANSIColor(w), Emoji: supportsEmoji(w)}
}

// icon returns an emoji or ASCII fallback depending on support.
func icon(enabled bool, kind string) string {
	if !enabled {
		switch kind {
		case "ok":
			return "[OK]"
		case "warn":
			return "[WARN]"
		case "error":
			return "[ERR]"
		case "info":
			return "[INFO]"
		default:
			return "[>]"
		}
	}
	switch kind {
	case "ok":
		return "✅"
	case "warn":
		return "⚠️"
	case "error":
		return "❌"
	case "info":
		return "ℹ️"
	default:
		return "•"
	}
}

// ── Help decoration ────────────────────────────────────────────────────────────

func decoratedHelpHeader(root *cobra.Command, title string) string {
	useColor := supportsANSIColor(root.OutOrStdout())
	useEmoji := supportsEmoji(root.OutOrStdout())

	iconPrefix := ""
	color := ansiBold + ansiCyan
	switch strings.TrimSpace(strings.ToLower(title)) {
	case "usage:":
		iconPrefix = "📌 "
		color = ansiBold + ansiBlue
	case "available commands:":
		iconPrefix = "🧭 "
		color = ansiBold + ansiCyan
	case "examples:":
		iconPrefix = "✨ "
		color = ansiBold + ansiYellow
	case "flags:", "global flags:":
		iconPrefix = "⚙️  "
		color = ansiBold + ansiMagenta
	case "context guide:":
		iconPrefix = "🧠 "
		color = ansiBold + ansiYellow
	}
	if !useEmoji {
		iconPrefix = ""
	}
	return style(useColor, color, iconPrefix+title)
}

func groupTitleDecorated(title string, useColor, useEmoji bool) string {
	prefix := ""
	color := ansiBold + ansiCyan
	switch strings.TrimSpace(strings.ToLower(title)) {
	case "vault":
		prefix = "🔐 "
		color = ansiBold + ansiGreen
	case "secrets":
		prefix = "🔑 "
		color = ansiBold + ansiBlue
	case "inject":
		prefix = "⚡ "
		color = ansiBold + ansiYellow
	case "infrastructure":
		prefix = "🚢 "
		color = ansiBold + ansiMagenta
	case "data":
		prefix = "📦 "
		color = ansiBold + ansiCyan
	}
	if !useEmoji {
		prefix = ""
	}
	return style(useColor, color, prefix+title)
}

func groupedHelpCommands(cmd *cobra.Command) string {
	if cmd == nil || cmd.Parent() != nil {
		return ""
	}
	useColor := supportsANSIColor(cmd.OutOrStdout())
	useEmoji := supportsEmoji(cmd.OutOrStdout())

	type group struct {
		Title string
		Names []string
	}
	groups := []group{
		{Title: "Vault", Names: []string{"init", "unlock", "lock"}},
		{Title: "Secrets", Names: []string{"get", "set", "delete", "list"}},
		{Title: "Inject", Names: []string{"run", "shell"}},
		{Title: "Infrastructure", Names: []string{"serve", "docker", "k3d"}},
		{Title: "Data", Names: []string{"import", "export", "providers", "version", "completion"}},
	}

	lookup := map[string]*cobra.Command{}
	for _, c := range cmd.Commands() {
		if c != nil && c.IsAvailableCommand() {
			lookup[c.Name()] = c
		}
	}

	const namePad = 20
	var b strings.Builder
	for _, g := range groups {
		written := 0
		for _, name := range g.Names {
			c, ok := lookup[name]
			if !ok {
				continue
			}
			if written == 0 {
				b.WriteString("  ")
				b.WriteString(groupTitleDecorated(g.Title, useColor, useEmoji))
				b.WriteByte('\n')
			}
			b.WriteString("  ")
			b.WriteString(style(useColor, ansiGreen, padRight(c.Name(), namePad)))
			b.WriteByte(' ')
			b.WriteString(style(useColor, ansiDim, c.Short))
			b.WriteByte('\n')
			written++
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func padRight(s string, pad int) string {
	if len(s) >= pad {
		return s
	}
	return s + strings.Repeat(" ", pad-len(s))
}

// contextualHelpGuide returns per-command next-step hints shown in "Context Guide:".
func contextualHelpGuide(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	path := strings.TrimSpace(strings.ToLower(cmd.CommandPath()))
	switch {
	case path == "vaultx":
		return strings.TrimSpace(`- First time: vaultx init --biometric
- Store a secret: vaultx set myapp/db_password "s3cr3t"
- Run your app: vaultx run -- npm start
- Inject into current shell: eval $(vaultx shell)
- Health check: vaultx providers`)
	case strings.HasSuffix(path, "init"):
		return strings.TrimSpace(`- Enable Touch ID on macOS: vaultx init --biometric
- Next — store your first secret: vaultx set myapp/api_key "value"
- Verify the vault: vaultx providers`)
	case strings.HasSuffix(path, "unlock"):
		return strings.TrimSpace(`- Enable Touch ID for future unlocks: vaultx unlock --biometric
- Check if Touch ID is configured: vaultx providers
- Lock the vault when done: vaultx lock`)
	case strings.HasSuffix(path, "lock"):
		return strings.TrimSpace(`- Unlock again: vaultx unlock
- Clear saved Touch ID credential: vaultx lock (clears session only; use vaultx init to reset biometric)`)
	case strings.HasSuffix(path, "get"):
		return strings.TrimSpace(`- Capture in a script: TOKEN=$(vaultx get myapp/api_key)
- See all keys: vaultx list
- Inject all at once: vaultx run -- <your-command>`)
	case strings.HasSuffix(path, "set"):
		return strings.TrimSpace(`- Verify storage: vaultx get myapp/key
- Bulk import: vaultx import -f env -i .env
- Reference in vaultx.env: KEY=vaultx://local/myapp/key`)
	case strings.HasSuffix(path, "delete"):
		return strings.TrimSpace(`- Back up first: vaultx export -f vaultx -o backup.json
- List remaining keys: vaultx list`)
	case strings.HasSuffix(path, "list"):
		return strings.TrimSpace(`- Filter by prefix: vaultx list myapp/
- Get a value: vaultx get myapp/key
- Export everything: vaultx export -f vaultx -o backup.json`)
	case strings.HasSuffix(path, "run"):
		return strings.TrimSpace(`- Dry run (see what would be injected): vaultx shell
- Use a custom env file: vaultx run -e staging.env -- node server.js
- Debug resolution: vaultx providers`)
	case strings.HasSuffix(path, "shell"):
		return strings.TrimSpace(`- Inject into current session: eval $(vaultx shell)
- Use a non-default env file: vaultx shell -e staging.env
- One-shot run: vaultx run -- <command>`)
	case strings.HasSuffix(path, "serve"):
		return strings.TrimSpace(`- Start daemon: vaultx serve --port 7474
- Kubernetes ESO setup: vaultx k3d setup
- Docker integration: vaultx docker run -- <image>`)
	case strings.HasSuffix(path, "import"):
		return strings.TrimSpace(`- From .env file: vaultx import -f env -i .env
- From JSON export: vaultx import -f vaultx -i backup.json
- Verify import: vaultx list`)
	case strings.HasSuffix(path, "export"):
		return strings.TrimSpace(`- Export all: vaultx export -f vaultx -o backup.json
- Export as .env: vaultx export -f env -o .env.backup
- Import on another machine: vaultx import -f vaultx -i backup.json`)
	case strings.HasSuffix(path, "providers"):
		return strings.TrimSpace(`- Check vault health: vaultx providers
- Config file: ~/.vaultx/config.toml
- Unlock and recheck: vaultx unlock && vaultx providers`)
	case strings.HasSuffix(path, "completion"):
		return strings.TrimSpace(`- Auto-detect and install: vaultx completion
- Force a specific shell: vaultx completion zsh
- Replace outdated file: vaultx completion --overwrite
- Reload without restarting: source ~/.zshrc  (or ~/.bashrc)`)
	default:
		return ""
	}
}

// ── Root command ───────────────────────────────────────────────────────────────

// Root returns the root cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "vaultx",
		Short: "The convenience of an env file. The power of a zero-trust vault.",
		Long: strings.TrimSpace(`vaultx — zero-trust secrets broker

vaultx.env is safe to commit. It contains only references, never values.
At runtime, vaultx run resolves each reference and injects the real
secret into your process — nothing ever touches disk.

Quick start:
  vaultx init --biometric       Create vault + enable Touch ID
  vaultx set myapp/db_pass s3cr3t    Store a secret
  vaultx run -- npm start            Resolve vaultx.env and run your app
  eval $(vaultx shell)               Inject secrets into current shell

Configuration:
  ~/.vaultx/config.toml   Multi-provider config (local, 1Password, HashiCorp, AWS)
  ~/.vaultx/vault.enc     Local encrypted vault (Argon2id + AES-256-GCM)
  vaultx.env              Secret reference file — commit this

Troubleshooting:
  vaultx providers    Check all configured providers and health
  vaultx version      Show version`),
		Example: "  vaultx init --biometric\n  vaultx set myapp/db_password \"s3cr3t\"\n  vaultx run -- npm start\n  eval $(vaultx shell)\n  vaultx providers",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			skip := map[string]bool{"init": true, "help": true, "version": true}
			if skip[cmd.Name()] {
				return nil
			}
			return loadState()
		},
	}

	root.PersistentFlags().StringVar(&globalFlags.configPath, "config", config.DefaultPath(), "config file path")
	root.PersistentFlags().StringVarP(&globalFlags.envFile, "env", "e", "", "vaultx.env file (default: auto-detect)")
	root.PersistentFlags().StringVar(&globalFlags.colorMode, "color", "auto", "color output: auto|always|never")
	root.PersistentFlags().StringVar(&globalFlags.emojiMode, "emoji", "auto", "emoji output: auto|always|never")

	root.AddGroup(
		&cobra.Group{ID: "vault", Title: "Vault"},
		&cobra.Group{ID: "secrets", Title: "Secrets"},
		&cobra.Group{ID: "inject", Title: "Inject"},
		&cobra.Group{ID: "infra", Title: "Infrastructure"},
		&cobra.Group{ID: "data", Title: "Data"},
	)

	initCmd := cmdInit()
	unlock := cmdUnlock()
	lock := cmdLock()
	initCmd.GroupID = "vault"
	unlock.GroupID = "vault"
	lock.GroupID = "vault"

	get := cmdGet()
	set := cmdSet()
	del := cmdDelete()
	list := cmdList()
	get.GroupID = "secrets"
	set.GroupID = "secrets"
	del.GroupID = "secrets"
	list.GroupID = "secrets"

	run := cmdRun()
	shell := cmdShell()
	run.GroupID = "inject"
	shell.GroupID = "inject"

	serve := cmdServe()
	docker := cmdDocker()
	k3d := cmdK3d()
	serve.GroupID = "infra"
	docker.GroupID = "infra"
	k3d.GroupID = "infra"

	imp := cmdImport()
	exp := cmdExport()
	prov := cmdProviders()
	imp.GroupID = "data"
	exp.GroupID = "data"
	prov.GroupID = "data"

	root.AddCommand(
		initCmd, unlock, lock,
		get, set, del, list,
		run, shell,
		serve, docker, k3d,
		imp, exp, prov,
		cmdVersion(),
		cmdCompletion(),
	)

	configureRichHelp(root)
	return root
}

func configureRichHelp(root *cobra.Command) {
	cobra.AddTemplateFunc("vaultxHeader", func(s string) string {
		return decoratedHelpHeader(root, s)
	})
	cobra.AddTemplateFunc("vaultxCmd", func(s string) string {
		return style(supportsANSIColor(root.OutOrStdout()), ansiBold+ansiGreen, s)
	})
	cobra.AddTemplateFunc("vaultxDim", func(s string) string {
		return style(supportsANSIColor(root.OutOrStdout()), ansiDim, s)
	})
	cobra.AddTemplateFunc("vaultxGuide", func(cmd *cobra.Command) string {
		return contextualHelpGuide(cmd)
	})
	cobra.AddTemplateFunc("vaultxGroupedCommands", func(cmd *cobra.Command) string {
		return groupedHelpCommands(cmd)
	})

	root.SetHelpTemplate(strings.TrimSpace(`
{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{vaultxHeader "Usage:"}}
  {{if .Runnable}}{{.UseLine}}{{else}}{{.CommandPath}} [command]{{end}}{{end}}{{if .HasAvailableSubCommands}}

{{vaultxHeader "Available Commands:"}}
{{with (vaultxGroupedCommands .)}}{{.}}{{else}}{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{vaultxCmd (rpad .Name .NamePadding) }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{if .HasExample}}

{{vaultxHeader "Examples:"}}
{{.Example}}{{end}}{{if .HasAvailableLocalFlags}}

{{vaultxHeader "Flags:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

{{vaultxHeader "Global Flags:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

{{vaultxHeader "Additional Help Topics:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{vaultxCmd (rpad .CommandPath .CommandPathPadding)}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

{{vaultxDim (printf "Use \"%s [command] --help\" for more information about a command." .CommandPath)}}{{end}}
{{with (vaultxGuide .)}}

{{vaultxHeader "Context Guide:"}}
{{.}}
{{end}}
`) + "\n")
}

// ── State management ───────────────────────────────────────────────────────────

// loadState initialises cfg, vault, and registry from the config file.
func loadState() error {
	cfg, err := config.Load(globalFlags.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	state.cfg = cfg

	vaultPath := cfg.Vault.Path
	if vaultPath == "" {
		vaultPath = config.DefaultVaultPath()
	}
	state.vault = local.New("local", vaultPath)

	state.registry = resolver.NewRegistry()
	state.registry.Register(state.vault, true)

	for _, pc := range cfg.Providers {
		switch pc.Type {
		case "onepassword":
			p := onepassword.New(pc.ID, pc.Account, pc.Vault)
			state.registry.Register(p, pc.Default)
		}
	}

	return nil
}

// requireUnlocked unlocks the vault. It first tries the passkey store (Touch ID
// on macOS), then falls back to prompting the user for their master password.
func requireUnlocked() error {
	if !state.vault.IsSealed() {
		return nil
	}
	// Try the passkey store first — Touch ID on macOS, no-op elsewhere.
	if stored, ok := passkey.Load(); ok {
		if err := state.vault.Unlock(stored); err == nil {
			return nil
		}
		// Stored credential is stale — clear it and fall through to prompt.
		passkey.Clear()
	}
	pass, err := readPassword("Master password: ")
	if err != nil {
		return err
	}
	return state.vault.Unlock(pass)
}

// readPassword reads a password from the terminal without echo.
func readPassword(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // newline after silent input
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	// Non-interactive fallback (pipes, CI).
	var pass string
	if _, err := fmt.Fscanln(os.Stdin, &pass); err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return pass, nil
}
