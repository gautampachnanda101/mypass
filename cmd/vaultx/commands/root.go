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

// Root returns the root cobra command.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "vaultx",
		Short: "The convenience of an env file. The power of a zero-trust vault.",
		Long: `vaultx resolves vault: references from a vaultx.env file and injects
the real secret values into processes, Docker containers, and Kubernetes pods —
without writing plaintext secrets to disk.

Quick start:
  vaultx init                    Create a new local vault
  vaultx unlock                  Unlock the vault for this session
  vaultx set myapp/db_pass s3cr3t Store a secret
  vaultx run -- npm start        Resolve vaultx.env and run your app`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			// Skip init for commands that don't need a loaded config.
			skip := map[string]bool{"init": true, "help": true, "version": true}
			if skip[cmd.Name()] {
				return nil
			}
			return loadState()
		},
	}

	root.PersistentFlags().StringVar(&globalFlags.configPath, "config", config.DefaultPath(), "config file path")
	root.PersistentFlags().StringVarP(&globalFlags.envFile, "env", "e", "", "vaultx.env file (default: auto-detect)")

	root.AddCommand(
		cmdInit(),
		cmdUnlock(),
		cmdLock(),
		cmdGet(),
		cmdSet(),
		cmdDelete(),
		cmdList(),
		cmdRun(),
		cmdShell(),
		cmdImport(),
		cmdExport(),
		cmdProviders(),
		cmdVersion(),
	)

	return root
}

// loadState initialises cfg, vault, and registry from the config file.
func loadState() error {
	cfg, err := config.Load(globalFlags.configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	state.cfg = cfg

	// Build local vault provider.
	vaultPath := cfg.Vault.Path
	if vaultPath == "" {
		vaultPath = config.DefaultVaultPath()
	}
	state.vault = local.New("local", vaultPath)

	// Build resolver registry from config.
	state.registry = resolver.NewRegistry()
	state.registry.Register(state.vault, true) // local is always registered

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
