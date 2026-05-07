package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gautampachnanda101/vaultx/internal/mfa"
	"github.com/spf13/cobra"
)

func cmdMFA() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mfa",
		Short: "Manage multi-factor authentication (TOTP)",
		Long: `Enable, disable, or manage TOTP-based multi-factor authentication.

When MFA is enabled, unlocking the vault requires both your master password
and a 6-digit code from your authenticator app (Google Authenticator, Authy, 1Password, etc.).

Recovery codes are generated during setup and can be used if you lose access
to your authenticator device.`,
		Example: `  # Enable MFA
  vaultx mfa enable

  # Disable MFA
  vaultx mfa disable

  # View remaining recovery codes
  vaultx mfa recovery-codes

  # Check MFA status
  vaultx mfa status`,
	}

	cmd.AddCommand(
		cmdMFAEnable(),
		cmdMFADisable(),
		cmdMFAStatus(),
		cmdMFARecoveryCodes(),
	)

	return cmd
}

func cmdMFAEnable() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable TOTP multi-factor authentication",
		Long: `Generate a new TOTP secret and recovery codes.

A QR code will be displayed in the terminal for easy setup with your
authenticator app. You can also enter the secret manually.

IMPORTANT: Save your recovery codes in a secure location. If you lose
access to your authenticator device, recovery codes are the only way
to unlock your vault.`,
		Example: `  vaultx mfa enable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			mfaPath := filepath.Join(home, ".vaultx", "mfa.json")
			mgr := mfa.New(mfaPath)

			// Check if already enabled
			enabled, err := mgr.IsEnabled()
			if err != nil {
				return err
			}
			if enabled {
				return fmt.Errorf("MFA is already enabled. Disable first to re-enroll.")
			}

			// Generate new MFA config
			secret, qrURL, recoveryCodes, err := mgr.Enable()
			if err != nil {
				return err
			}

			// Render QR code in terminal
			qrText, err := mfa.RenderQRTerminal(qrURL)
			if err != nil {
				return fmt.Errorf("render QR code: %w", err)
			}

			fmt.Println("\n✅ MFA Enabled Successfully")
			fmt.Println("Scan this QR code with your authenticator app:")
			fmt.Println(qrText)
			fmt.Printf("\nOr enter this secret manually:\n")
			fmt.Printf("  Secret:  %s\n", secret)
			fmt.Printf("  Account: vaultx-vault\n")
			fmt.Printf("  Issuer:  vaultx\n\n")

			fmt.Println("Recovery Codes (save these securely):")
			for i, code := range recoveryCodes {
				fmt.Printf("  %2d. %s\n", i+1, code)
			}
			fmt.Println("\n⚠️  These recovery codes will only be displayed ONCE.")
			fmt.Println("   Store them in a secure location (password manager, safe, etc.)")
			fmt.Println("\nNext unlock will require your authenticator code.")

			return nil
		},
	}
}

func cmdMFADisable() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable TOTP multi-factor authentication",
		Long:  `Turn off MFA. Subsequent unlocks will only require the master password.`,
		Example: `  vaultx mfa disable`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			mfaPath := filepath.Join(home, ".vaultx", "mfa.json")
			mgr := mfa.New(mfaPath)

			enabled, err := mgr.IsEnabled()
			if err != nil {
				return err
			}
			if !enabled {
				return fmt.Errorf("MFA is not currently enabled")
			}

			if err := mgr.Disable(); err != nil {
				return err
			}

			fmt.Println("✅ MFA disabled. Vault unlocks will no longer require a code.")
			return nil
		},
	}
}

func cmdMFAStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show MFA status",
		Long:  `Display whether MFA is enabled and how many recovery codes remain.`,
		Example: `  vaultx mfa status`,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			mfaPath := filepath.Join(home, ".vaultx", "mfa.json")
			mgr := mfa.New(mfaPath)

			enabled, err := mgr.IsEnabled()
			if err != nil {
				return err
			}

			if !enabled {
				fmt.Println("MFA Status: ❌ Disabled")
				return nil
			}

			codes, err := mgr.GetRecoveryCodes()
			if err != nil {
				return err
			}

			fmt.Println("MFA Status: ✅ Enabled")
			fmt.Printf("Recovery Codes Remaining: %d\n", len(codes))
			if len(codes) < 3 {
				fmt.Println("⚠️  You're running low on recovery codes. Consider re-enrolling MFA.")
			}

			return nil
		},
	}
}

func cmdMFARecoveryCodes() *cobra.Command {
	return &cobra.Command{
		Use:   "recovery-codes",
		Short: "View remaining recovery codes",
		Long: `Display the remaining one-time recovery codes.

Each recovery code can only be used once. When you run low, consider
disabling and re-enabling MFA to generate fresh codes.`,
		Example: `  vaultx mfa recovery-codes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireUnlocked(); err != nil {
				return err
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("get home dir: %w", err)
			}
			mfaPath := filepath.Join(home, ".vaultx", "mfa.json")
			mgr := mfa.New(mfaPath)

			enabled, err := mgr.IsEnabled()
			if err != nil {
				return err
			}
			if !enabled {
				return fmt.Errorf("MFA is not enabled")
			}

			codes, err := mgr.GetRecoveryCodes()
			if err != nil {
				return err
			}

			if len(codes) == 0 {
				fmt.Println("No recovery codes remaining.")
				fmt.Println("Disable and re-enable MFA to generate new codes.")
				return nil
			}

			fmt.Printf("Remaining Recovery Codes (%d):\n\n", len(codes))
			for i, code := range codes {
				fmt.Printf("  %2d. %s\n", i+1, code)
			}
			fmt.Println("\n⚠️  Each code can only be used once.")

			return nil
		},
	}
}
