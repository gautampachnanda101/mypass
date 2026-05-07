package mfa

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestManager_EnableDisable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	mgr := New(configPath)

	// Initially not enabled
	enabled, err := mgr.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled: %v", err)
	}
	if enabled {
		t.Error("MFA should not be enabled initially")
	}

	// Enable MFA
	secret, qrURL, recoveryCodes, err := mgr.Enable()
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Validate outputs
	if secret == "" {
		t.Error("secret should not be empty")
	}
	if len(secret) < 32 {
		t.Errorf("secret too short: got %d chars, want >= 32", len(secret))
	}

	if qrURL == "" {
		t.Error("qrURL should not be empty")
	}
	if !strings.HasPrefix(qrURL, "otpauth://totp/") {
		t.Errorf("invalid OTP URL: %s", qrURL)
	}
	if !strings.Contains(qrURL, secret) {
		t.Error("QR URL should contain the secret")
	}

	if len(recoveryCodes) != 10 {
		t.Errorf("got %d recovery codes, want 10", len(recoveryCodes))
	}
	for i, code := range recoveryCodes {
		if len(code) != 9 { // XXXX-XXXX = 9 chars
			t.Errorf("recovery code %d has invalid length: %q", i, code)
		}
		if code[4] != '-' {
			t.Errorf("recovery code %d missing dash: %q", i, code)
		}
	}

	// Should now be enabled
	enabled, err = mgr.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled after enable: %v", err)
	}
	if !enabled {
		t.Error("MFA should be enabled after Enable()")
	}

	// Disable MFA
	if err := mgr.Disable(); err != nil {
		t.Fatalf("Disable: %v", err)
	}

	// Should be disabled
	enabled, err = mgr.IsEnabled()
	if err != nil {
		t.Fatalf("IsEnabled after disable: %v", err)
	}
	if enabled {
		t.Error("MFA should be disabled after Disable()")
	}
}

func TestManager_ValidateTOTP(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	mgr := New(configPath)

	// Enable MFA
	secret, _, _, err := mgr.Enable()
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Generate valid TOTP code
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}

	// Validate the code
	valid, err := mgr.Validate(code)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if !valid {
		t.Error("generated TOTP code should be valid")
	}

	// Invalid code should fail
	valid, err = mgr.Validate("000000")
	if err != nil {
		t.Fatalf("Validate invalid code: %v", err)
	}
	if valid {
		t.Error("invalid TOTP code should not be valid")
	}
}

func TestManager_ValidateRecoveryCode(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	mgr := New(configPath)

	// Enable MFA
	_, _, recoveryCodes, err := mgr.Enable()
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	if len(recoveryCodes) == 0 {
		t.Fatal("no recovery codes generated")
	}

	firstCode := recoveryCodes[0]

	// Validate recovery code
	valid, err := mgr.Validate(firstCode)
	if err != nil {
		t.Fatalf("Validate recovery code: %v", err)
	}
	if !valid {
		t.Error("recovery code should be valid")
	}

	// Same code should not work twice (consumed)
	valid, err = mgr.Validate(firstCode)
	if err != nil {
		t.Fatalf("Validate used recovery code: %v", err)
	}
	if valid {
		t.Error("used recovery code should not be valid again")
	}

	// Check remaining codes
	remaining, err := mgr.GetRecoveryCodes()
	if err != nil {
		t.Fatalf("GetRecoveryCodes: %v", err)
	}
	if len(remaining) != 9 {
		t.Errorf("got %d remaining codes, want 9", len(remaining))
	}

	// First code should not be in remaining
	for _, code := range remaining {
		if code == firstCode {
			t.Error("used recovery code should not appear in remaining codes")
		}
	}
}

func TestManager_ValidateWhenDisabled(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	mgr := New(configPath)

	// Try to validate without enabling
	_, err := mgr.Validate("123456")
	if err == nil {
		t.Error("expected error validating when MFA not enabled")
	}
}

func TestManager_LoadNonexistentConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "nonexistent.json")

	mgr := New(configPath)

	cfg, err := mgr.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig nonexistent: %v", err)
	}

	if cfg.Enabled {
		t.Error("nonexistent config should be disabled")
	}
}

func TestManager_ConfigPersistence(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	// Enable with first manager
	mgr1 := New(configPath)
	secret, _, recoveryCodes, err := mgr1.Enable()
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Load with new manager
	mgr2 := New(configPath)
	cfg, err := mgr2.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if !cfg.Enabled {
		t.Error("config should persist as enabled")
	}
	if cfg.Secret != secret {
		t.Error("secret should persist")
	}
	if len(cfg.RecoveryCodes) != len(recoveryCodes) {
		t.Error("recovery codes should persist")
	}
}

func TestGenerateQRCode(t *testing.T) {
	otpURL := "otpauth://totp/vaultx:vaultx-vault?secret=TESTSECRET&issuer=vaultx"

	data, err := GenerateQRCode(otpURL)
	if err != nil {
		t.Fatalf("GenerateQRCode: %v", err)
	}

	if len(data) == 0 {
		t.Error("QR code data should not be empty")
	}

	// PNG signature check
	if len(data) < 8 {
		t.Error("QR code data too short to be valid PNG")
	}
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("QR code data does not have PNG signature")
	}
}

func TestRenderQRTerminal(t *testing.T) {
	otpURL := "otpauth://totp/vaultx:vaultx-vault?secret=TESTSECRET&issuer=vaultx"

	ascii, err := RenderQRTerminal(otpURL)
	if err != nil {
		t.Fatalf("RenderQRTerminal: %v", err)
	}

	if ascii == "" {
		t.Error("ASCII QR code should not be empty")
	}

	// Should contain block characters or spaces
	if !strings.ContainsAny(ascii, "█ ") {
		t.Error("ASCII QR code should contain block characters or spaces")
	}
}

func TestRecoveryCodeFormat(t *testing.T) {
	codes, err := generateRecoveryCodes(5)
	if err != nil {
		t.Fatalf("generateRecoveryCodes: %v", err)
	}

	if len(codes) != 5 {
		t.Errorf("got %d codes, want 5", len(codes))
	}

	for i, code := range codes {
		// Check length
		if len(code) != 9 {
			t.Errorf("code %d has length %d, want 9", i, len(code))
		}

		// Check format: XXXX-XXXX
		if code[4] != '-' {
			t.Errorf("code %d missing dash at position 4: %q", i, code)
		}

		// Check no ambiguous characters (0, O, 1, I)
		if strings.ContainsAny(code, "01OI") {
			t.Errorf("code %d contains ambiguous chars: %q", i, code)
		}

		// Check all chars are alphanumeric (excluding dash)
		for j, ch := range code {
			if j == 4 {
				continue // skip dash
			}
			if !((ch >= 'A' && ch <= 'Z') || (ch >= '2' && ch <= '9')) {
				t.Errorf("code %d has invalid char at position %d: %q", i, j, ch)
			}
		}
	}
}

func TestRecoveryCodeUniqueness(t *testing.T) {
	codes, err := generateRecoveryCodes(100)
	if err != nil {
		t.Fatalf("generateRecoveryCodes: %v", err)
	}

	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate recovery code generated: %q", code)
		}
		seen[code] = true
	}
}

func TestManager_CorruptedConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	// Write corrupted JSON
	if err := os.WriteFile(configPath, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	mgr := New(configPath)
	_, err := mgr.LoadConfig()
	if err == nil {
		t.Error("expected error loading corrupted config")
	}
}

func TestManager_MultipleRecoveryCodeConsumption(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "mfa.json")

	mgr := New(configPath)

	// Enable MFA
	_, _, recoveryCodes, err := mgr.Enable()
	if err != nil {
		t.Fatalf("Enable: %v", err)
	}

	// Use first 3 recovery codes
	for i := 0; i < 3; i++ {
		valid, err := mgr.Validate(recoveryCodes[i])
		if err != nil {
			t.Fatalf("Validate recovery code %d: %v", i, err)
		}
		if !valid {
			t.Errorf("recovery code %d should be valid", i)
		}
	}

	// Check remaining
	remaining, err := mgr.GetRecoveryCodes()
	if err != nil {
		t.Fatalf("GetRecoveryCodes: %v", err)
	}
	if len(remaining) != 7 {
		t.Errorf("got %d remaining codes, want 7", len(remaining))
	}

	// Verify used codes are not in remaining
	for i := 0; i < 3; i++ {
		for _, code := range remaining {
			if code == recoveryCodes[i] {
				t.Errorf("used recovery code %d should not be in remaining", i)
			}
		}
	}
}
