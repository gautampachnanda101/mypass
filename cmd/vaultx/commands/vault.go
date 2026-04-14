package commands

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gautampachnanda101/vaultx/internal/config"
	"github.com/gautampachnanda101/vaultx/internal/providers/local"
)

func cmdInit() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a new local vault",
		Long:  "Create a new local vault at ~/.vaultx/vault.enc encrypted with AES-256-GCM.\nYou will be prompted to choose a master password. The password is never stored —\nArgon2id derives a barrier key that wraps the encryption key in memory only.",
		Example: `  vaultx init`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(globalFlags.configPath)
			if err != nil {
				return err
			}
			vaultPath := cfg.Vault.Path
			if vaultPath == "" {
				vaultPath = config.DefaultVaultPath()
			}

			pass, err := readPassword("Choose a master password: ")
			if err != nil {
				return err
			}
			confirm, err := readPassword("Confirm master password: ")
			if err != nil {
				return err
			}
			if pass != confirm {
				return fmt.Errorf("passwords do not match")
			}

			v := state.vault
			if v == nil {
				// loadState skips init, so build the vault manually here.
				v = local.New("local", vaultPath)
			}
			if err := v.Init(pass); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Vault created at %s\n", vaultPath)
			return nil
		},
	}
}

func cmdUnlock() *cobra.Command {
	return &cobra.Command{
		Use:     "unlock",
		Short:   "Unlock the vault (cache key for this session)",
		Long:    "Prompt for the master password and cache the derived key for this session.\nThe vault remains unlocked until 'vaultx lock' is called or the process exits.",
		Example: `  vaultx unlock`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return requireUnlocked()
		},
	}
}

func cmdLock() *cobra.Command {
	return &cobra.Command{
		Use:     "lock",
		Short:   "Lock the vault (clear cached key)",
		Long:    "Zero and clear the encryption key from memory. Subsequent commands that need\nthe vault will prompt for the master password again.",
		Example: `  vaultx lock`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			state.vault.Lock()
			fmt.Fprintln(os.Stderr, "Vault locked.")
			return nil
		},
	}
}

func cmdGet() *cobra.Command {
	return &cobra.Command{
		Use:   "get <path>",
		Short: "Get a secret value",
		Long:  "Retrieve a single secret value from the vault and print it to stdout.\nThe value is printed without a trailing newline so it can be safely captured\nin scripts: TOKEN=$(vaultx get myapp/api_key)",
		Example: `  vaultx get myapp/db_password
  TOKEN=$(vaultx get myapp/api_key)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}
			val, err := state.registry.Get(context.Background(), args[0])
			if err != nil {
				return err
			}
			fmt.Println(val)
			return nil
		},
	}
}

func cmdSet() *cobra.Command {
	return &cobra.Command{
		Use:   "set <path> <value>",
		Short: "Store a secret in the local vault",
		Long: "Store a secret at the given path. Paths are hierarchical with '/' as separator.\n" +
			"The value is encrypted with AES-256-GCM and stored in ~/.vaultx/vault.enc.\n" +
			"An existing entry at the same path is overwritten.",
		Example: `  vaultx set myapp/db_password "s3cr3t"
  vaultx set myapp/api_key "sk-live-abc123"
  vaultx set infra/prod/redis-url "redis://..."`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}
			if err := state.vault.Set(context.Background(), args[0], args[1]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Set %s\n", args[0])
			return nil
		},
	}
}

func cmdDelete() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <path>",
		Short:   "Delete a secret from the local vault",
		Long:    "Permanently remove a secret from the vault. This operation cannot be undone.\nConsider running 'vaultx export -f vaultx -o backup.json' before bulk deletions.",
		Example: `  vaultx delete myapp/db_password`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}
			if err := state.vault.Delete(context.Background(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Deleted %s\n", args[0])
			return nil
		},
	}
}

func cmdList() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [prefix]",
		Short: "List secrets (values masked)",
		Long:  "List all secrets stored in the vault. Values are always masked — only paths,\nprovider names, and last-updated timestamps are shown. Safe to share or log.",
		Example: `  vaultx list                  # all secrets
  vaultx list myapp/           # secrets under myapp/
  vaultx list infra/prod/`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}
			prefix := ""
			if len(args) > 0 {
				prefix = args[0]
			}
			secrets, err := state.vault.List(context.Background(), prefix)
			if err != nil {
				return err
			}
			sort.Slice(secrets, func(i, j int) bool {
				return secrets[i].Key < secrets[j].Key
			})

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "KEY\tPROVIDER\tUPDATED")
			for _, s := range secrets {
				fmt.Fprintf(w, "%s\t%s\t%s\n",
					s.Key, s.Provider,
					s.UpdatedAt.Format("2006-01-02 15:04"),
				)
			}
			return w.Flush()
		},
	}
	return cmd
}

func cmdProviders() *cobra.Command {
	return &cobra.Command{
		Use:   "providers",
		Short: "List configured providers and their health",
		Long:  "List all providers configured in ~/.vaultx/config.toml and check their health.\nThe local vault reports 'sealed' if locked; 1Password reports health via the op CLI.",
		Example: `  vaultx providers`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTYPE\tSTATUS")
			for _, pc := range state.cfg.Providers {
				status := "ok"
				def := ""
				if pc.Default {
					def = " (default)"
				}
				// Health check local vault if it's the one configured.
				if pc.Type == "local" {
					if err := state.vault.Health(ctx); err != nil {
						status = "sealed"
					}
				}
				fmt.Fprintf(w, "%s%s\t%s\t%s\n", pc.ID, def, pc.Type, status)
			}
			return w.Flush()
		},
	}
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			v := buildInfo.version
			if v == "" {
				v = "dev"
			}
			c := buildInfo.commit
			if c == "" {
				c = "none"
			}
			d := buildInfo.date
			if d == "" {
				d = "unknown"
			}
			fmt.Printf("vaultx %s (commit %s, built %s)\n", v, c, d)
		},
	}
}

// cmdShell prints export statements for eval.
func cmdShell() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Print export statements (eval $(vaultx shell))",
		Long:  "Resolve vaultx.env and print 'export KEY=VALUE' lines to stdout.\nDesigned to be eval'd in the current shell so secrets are available\nto subsequent commands in the same session.",
		Example: `  eval $(vaultx shell)
  eval $(vaultx shell --env staging.env)`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}
			env, err := resolveEnvFile()
			if err != nil {
				return err
			}
			for k, v := range env {
				fmt.Printf("export %s=%s\n", k, shellQuote(v))
			}
			return nil
		},
	}
}

// shellQuote wraps a value in single quotes, escaping existing single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
