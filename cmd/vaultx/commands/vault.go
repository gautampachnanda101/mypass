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
		Use:   "unlock",
		Short: "Unlock the vault (cache key for this session)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return requireUnlocked()
		},
	}
}

func cmdLock() *cobra.Command {
	return &cobra.Command{
		Use:   "lock",
		Short: "Lock the vault (clear cached key)",
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
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.ExactArgs(2),
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
		Use:   "delete <path>",
		Short: "Delete a secret from the local vault",
		Args:  cobra.ExactArgs(1),
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
		Args:  cobra.MaximumNArgs(1),
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
			fmt.Println("vaultx dev")
		},
	}
}

// cmdShell prints export statements for eval.
func cmdShell() *cobra.Command {
	return &cobra.Command{
		Use:   "shell",
		Short: "Print export statements (eval $(vaultx shell))",
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
