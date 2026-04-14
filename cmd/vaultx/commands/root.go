package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/config"
	"github.com/gautampachnanda101/vaultx/internal/providers/local"
	"github.com/gautampachnanda101/vaultx/internal/providers/onepassword"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
)

// globalFlags are shared across all subcommands.
var globalFlags struct {
	configPath string
	envFile    string
}

// appState holds lazily-initialised shared objects.
type appState struct {
	cfg      *config.Config
	vault    *local.Provider
	registry *resolver.Registry
}

var state appState

const ansiReset = "\033[0m"

// ANSI helpers — disabled when NO_COLOR is set or stdout is not a TTY.
func isColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

func bold(s string) string {
	if !isColorEnabled() {
		return s
	}
	return "\033[1m" + s + ansiReset
}

func cyan(s string) string {
	if !isColorEnabled() {
		return s
	}
	return "\033[36m" + s + ansiReset
}

func green(s string) string {
	if !isColorEnabled() {
		return s
	}
	return "\033[32m" + s + ansiReset
}

func yellow(s string) string {
	if !isColorEnabled() {
		return s
	}
	return "\033[33m" + s + ansiReset
}

func dim(s string) string {
	if !isColorEnabled() {
		return s
	}
	return "\033[2m" + s + ansiReset
}

// Root returns the root cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "vaultx",
		Short: "The convenience of an env file. The power of a zero-trust vault.",
		Long: bold("vaultx") + " — zero-trust secrets broker\n\n" +
			cyan("vaultx.env") + " is safe to commit. It contains only references, never values.\n" +
			"At runtime, " + cyan("vaultx run") + " resolves each reference and injects the real\n" +
			"secret into your process — nothing ever touches disk.\n\n" +
			bold("Quick start:") + "\n" +
			"  " + green("vaultx init") + "                        Create a new local vault\n" +
			"  " + green("vaultx set myapp/db_pass s3cr3t") + "    Store a secret\n" +
			"  " + green("vaultx run -- npm start") + "            Resolve vaultx.env and run your app\n" +
			"  " + green("eval $(vaultx shell)") + "               Inject secrets into current shell\n\n" +
			bold("Configuration:") + "\n" +
			"  " + dim("~/.vaultx/config.toml") + "  Multi-provider config (local, 1Password, HashiCorp, AWS)\n" +
			"  " + dim("~/.vaultx/vault.enc") + "    Local encrypted vault (Argon2id + AES-256-GCM)\n" +
			"  " + dim("vaultx.env") + "              Secret reference file — commit this\n\n" +
			bold("Troubleshooting:") + "\n" +
			"  " + yellow("vaultx providers") + "  Check all configured providers and health\n" +
			"  " + yellow("vaultx version") + "    Show version",
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

	// Register command groups — controls how commands are bucketed in --help output.
	root.AddGroup(
		&cobra.Group{ID: "vault", Title: bold("Vault") + "  " + dim("— lifecycle and authentication")},
		&cobra.Group{ID: "secrets", Title: bold("Secrets") + "  " + dim("— store, retrieve and manage")},
		&cobra.Group{ID: "inject", Title: bold("Inject") + "  " + dim("— run commands and shells with secrets")},
		&cobra.Group{ID: "infra", Title: bold("Infrastructure") + "  " + dim("— Docker, Kubernetes, HTTP daemon")},
		&cobra.Group{ID: "data", Title: bold("Data") + "  " + dim("— import, export, providers")},
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
	)

	// Override both templates so Long only appears once and groups render correctly.
	root.SetHelpTemplate("{{.UsageString}}")
	root.SetUsageTemplate(usageTemplate())

	return root
}

func usageTemplate() string {
	cmdEntry := "  " + cyan("{{rpad .Name .NamePadding }}") + " {{.Short}}\n"
	return `{{if .Long}}{{.Long}}

{{end}}` + bold("Usage:") + `
  {{.UseLine}}{{if .HasAvailableSubCommands}} [command]{{end}}{{if gt (len .Aliases) 0}}

` + bold("Aliases:") + `
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

` + bold("Examples:") + `
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

` + bold("Available Commands:") + `
{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}` + cmdEntry + `{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{$group.Title}}
{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}` + cmdEntry + `{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

` + dim("Other Commands:") + `
{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}` + cmdEntry + `{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

` + bold("Flags:") + `
{{.LocalFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasAvailableInheritedFlags}}

` + bold("Global Flags:") + `
{{.InheritedFlags.FlagUsages | trimRightSpace}}{{end}}{{if .HasHelpSubCommands}}

` + bold("Additional help topics:") + `
{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}
{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

` + dim(`Use "{{.CommandPath}} [command] --help" for more information about a command.`) + `
{{end}}`
}

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

// requireUnlocked prompts for the master password if the vault is sealed.
func requireUnlocked() error {
	if !state.vault.IsSealed() {
		return nil
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
	var pass string
	if _, err := fmt.Fscanln(os.Stdin, &pass); err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return pass, nil
}

