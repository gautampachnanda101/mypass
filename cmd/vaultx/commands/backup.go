package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gautampachnanda101/vaultx/internal/backup"
	"github.com/spf13/cobra"
)

func cmdBackup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Manage encrypted vault backups",
		Long: `Create, list, and restore encrypted vault backups.

Backups are encrypted with a key derived from your master password.
Optionally, use Shamir secret sharing to split the backup key into N shares,
requiring M shares to restore.

Example workflow:
  1. Create a backup: vaultx backup create
  2. Split backup key into shares: vaultx backup split --shares 5 --threshold 3
  3. Distribute shares to different locations or people
  4. Restore from backup: vaultx backup restore --shares share-1.json,share-2.json,share-3.json`,
		Example: `  # Create a backup
  vaultx backup create

  # List backups
  vaultx backup list

  # Restore from backup (with master password)
  vaultx backup restore vault-2025-05-15T10-30-00.enc

  # Split backup key into 5 shares, requiring 3 to restore
  vaultx backup split --shares 5 --threshold 3

  # Restore using Shamir shares
  vaultx backup restore --shares share-1.json,share-2.json,share-3.json vault-2025-05-15T10-30-00.enc`,
	}

	cmd.AddCommand(
		cmdBackupCreate(),
		cmdBackupList(),
		cmdBackupRestore(),
		cmdBackupSplit(),
	)

	return cmd
}

func cmdBackupCreate() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create an encrypted backup of the vault",
		Long: `Create an encrypted backup of the vault file.

The backup is stored in ~/.vaultx/backups/ and encrypted with a key
derived from your master password.`,
		Example: `  vaultx backup create`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}

			vaultPath := filepath.Join(home, ".vaultx", "vault.enc")
			backupDir := filepath.Join(home, ".vaultx", "backups")
			mgr := backup.New(backupDir)

			// Prompt for master password
			pass, err := readPassword("Master password (for backup encryption): ")
			if err != nil {
				return err
			}

			backupFile, err := mgr.AutoBackup(vaultPath, pass)
			if err != nil {
				return err
			}

			fmt.Printf("✅ Backup created: %s\n", filepath.Base(backupFile))
			return nil
		},
	}
}

func cmdBackupList() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available backups",
		Long:  `Display all encrypted backups in ~/.vaultx/backups/`,
		Example: `  vaultx backup list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}

			backupDir := filepath.Join(home, ".vaultx", "backups")
			mgr := backup.New(backupDir)

			backups, err := mgr.ListBackups()
			if err != nil {
				return err
			}

			if len(backups) == 0 {
				fmt.Println("No backups found.")
				return nil
			}

			fmt.Printf("Available backups (%d):\n\n", len(backups))
			for i, b := range backups {
				fmt.Printf("  %2d. %s  (%s, %d bytes)\n",
					i+1, b.Filename, b.Timestamp.Format("2006-01-02 15:04:05"), b.Size)
			}

			return nil
		},
	}
}

func cmdBackupRestore() *cobra.Command {
	var sharesFlag string

	cmd := &cobra.Command{
		Use:   "restore <backup-file>",
		Short: "Restore vault from backup",
		Long: `Restore the vault from an encrypted backup.

By default, prompts for the master password.
Use --shares to restore using Shamir secret shares instead.`,
		Example: `  # Restore with master password
  vaultx backup restore vault-2025-05-15T10-30-00.enc

  # Restore with Shamir shares
  vaultx backup restore --shares share-1.json,share-2.json,share-3.json vault-2025-05-15T10-30-00.enc`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backupFilename := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}

			vaultPath := filepath.Join(home, ".vaultx", "vault.enc")
			backupDir := filepath.Join(home, ".vaultx", "backups")
			mgr := backup.New(backupDir)

			// Restore with Shamir shares
			if sharesFlag != "" {
				shareFiles := strings.Split(sharesFlag, ",")
				if len(shareFiles) == 0 {
					return fmt.Errorf("no share files provided")
				}

				shares := make([]backup.ShamirShare, len(shareFiles))
				for i, file := range shareFiles {
					share, err := backup.ImportShare(strings.TrimSpace(file))
					if err != nil {
						return fmt.Errorf("import share %s: %w", file, err)
					}
					shares[i] = *share
				}

				if err := mgr.RestoreWithShares(backupFilename, shares, vaultPath); err != nil {
					return err
				}

				fmt.Println("✅ Vault restored from backup using Shamir shares")
				fmt.Println("⚠️  Restart vaultx to load the restored vault")
				return nil
			}

			// Restore with master password
			pass, err := readPassword("Master password: ")
			if err != nil {
				return err
			}

			if err := mgr.Restore(backupFilename, pass, vaultPath); err != nil {
				return err
			}

			fmt.Println("✅ Vault restored from backup")
			fmt.Println("⚠️  Restart vaultx to load the restored vault")
			return nil
		},
	}

	cmd.Flags().StringVar(&sharesFlag, "shares", "", "comma-separated list of Shamir share files (e.g., share-1.json,share-2.json)")
	return cmd
}

func cmdBackupSplit() *cobra.Command {
	var n, m int
	var output string

	cmd := &cobra.Command{
		Use:   "split",
		Short: "Split backup key into Shamir shares",
		Long: `Split the backup encryption key into N shares, requiring M to reconstruct.

This allows you to distribute backup restore capability across multiple
people or locations. For example, split into 5 shares requiring any 3
to restore.

The shares are written to individual JSON files that can be stored separately.`,
		Example: `  # Split into 5 shares, requiring 3 to restore
  vaultx backup split --shares 5 --threshold 3

  # Split and save to a specific directory
  vaultx backup split --shares 5 --threshold 3 --output ./backup-shares`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			// Prompt for master password
			pass, err := readPassword("Master password (for key derivation): ")
			if err != nil {
				return err
			}

			shares, err := backup.SplitBackupKey(pass, n, m)
			if err != nil {
				return err
			}

			// Default output directory
			if output == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("get home dir: %w", err)
				}
				output = filepath.Join(home, ".vaultx", "backup-shares")
			}

			if err := backup.ExportShares(shares, output); err != nil {
				return err
			}

			fmt.Printf("✅ Backup key split into %d shares (threshold: %d)\n", n, m)
			fmt.Printf("   Shares saved to: %s\n\n", output)
			fmt.Println("Share files:")
			for _, share := range shares {
				fmt.Printf("  - share-%d.json\n", share.Index)
			}
			fmt.Printf("\n⚠️  Distribute shares to different secure locations.\n")
			fmt.Printf("   Any %d shares can restore your backup.\n", m)

			return nil
		},
	}

	cmd.Flags().IntVarP(&n, "shares", "n", 5, "total number of shares to create")
	cmd.Flags().IntVarP(&m, "threshold", "m", 3, "minimum shares required to restore")
	cmd.Flags().StringVarP(&output, "output", "o", "", "output directory for share files (default: ~/.vaultx/backup-shares)")

	return cmd
}
