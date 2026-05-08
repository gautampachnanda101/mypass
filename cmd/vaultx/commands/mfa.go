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

When MFA is enabled, unlocking the vault requires BOTH authentication factors:
  1. Touch ID (if configured) OR master password
  2. 6-digit TOTP code from your authenticator app

Supported authenticator apps: Google Authenticator, Authy, 1Password, Microsoft Authenticator, etc.

Workflow:
  • After Touch ID or password unlock, you'll be prompted: "Authenticator code:"
  • Enter the 6-digit code from your authenticator app
  • Codes refresh every 30 seconds

Recovery codes:
  • 10 single-use codes generated during MFA setup
  • Use INSTEAD of TOTP code when prompted (enter at "Authenticator code:" prompt)
  • Keep them in a secure location (password manager, safe, etc.)
  • Each code can only be used once

Troubleshooting:
  • If codes don't work: ensure your device time is synchronized (Settings → General → Date & Time)
  • If you enabled MFA multiple times: delete old entries from your authenticator app
  • If you lost your authenticator: use a recovery code at the "Authenticator code:" prompt`,
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

Setup steps:
  1. Run 'vaultx mfa enable'
  2. A QR code will be displayed in your terminal
  3. Open your authenticator app (Google Authenticator, Authy, etc.)
  4. Scan the QR code OR enter the secret manually
  5. Save the 10 recovery codes in a secure location

After setup:
  • Next vault unlock will prompt for "Authenticator code:" after Touch ID/password
  • Enter the 6-digit code from your authenticator app
  • Codes rotate every 30 seconds

IMPORTANT:
  • Save your recovery codes! They're the only way to unlock if you lose your device.
  • If you enable MFA multiple times, DELETE old entries from your authenticator app
  • Keep only the most recent "vaultx-vault" entry in your authenticator`,
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
		Long:  `Turn off MFA. Subsequent unlocks will only require the master password (or Touch ID).

You will be prompted for your authenticator code or a recovery code to disable MFA.
This prevents unauthorized MFA removal.`,
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

Each recovery code can only be used once. Use recovery codes at the
"Authenticator code:" prompt if you lose access to your authenticator device.

When you run low on codes (< 3 remaining), consider disabling and
re-enabling MFA to generate 10 fresh codes.`,
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
