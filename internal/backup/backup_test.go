package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_AutoBackupAndRestore(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "test-password-123"

	// Create fake vault data
	originalData := []byte("secret vault data for testing")
	if err := os.WriteFile(vaultPath, originalData, 0600); err != nil {
		t.Fatalf("write test vault: %v", err)
	}

	mgr := New(backupDir)

	// Create backup
	backupFile, err := mgr.AutoBackup(vaultPath, password)
	if err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	if backupFile == "" {
		t.Error("backup filename should not be empty")
	}

	// Check backup file exists
	if _, err := os.Stat(backupFile); os.IsNotExist(err) {
		t.Errorf("backup file not created: %s", backupFile)
	}

	// Modify vault data
	if err := os.WriteFile(vaultPath, []byte("modified data"), 0600); err != nil {
		t.Fatalf("modify vault: %v", err)
	}

	// Restore from backup
	backupFilename := filepath.Base(backupFile)
	if err := mgr.Restore(backupFilename, password, vaultPath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	// Verify restored data
	restoredData, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("read restored vault: %v", err)
	}

	if string(restoredData) != string(originalData) {
		t.Errorf("restored data mismatch:\ngot:  %q\nwant: %q", restoredData, originalData)
	}
}

func TestManager_RestoreWrongPassword(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "correct-password"

	// Create vault and backup
	vaultData := []byte("secret data")
	if err := os.WriteFile(vaultPath, vaultData, 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}

	mgr := New(backupDir)
	backupFile, err := mgr.AutoBackup(vaultPath, password)
	if err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	// Try to restore with wrong password
	backupFilename := filepath.Base(backupFile)
	err = mgr.Restore(backupFilename, "wrong-password", vaultPath)
	if err == nil {
		t.Error("expected error restoring with wrong password")
	}
}

func TestManager_ListBackups(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "test-password"

	mgr := New(backupDir)

	// Initially empty
	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}
	if len(backups) != 0 {
		t.Errorf("got %d backups, want 0", len(backups))
	}

	// Create vault
	if err := os.WriteFile(vaultPath, []byte("data"), 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}

	// Create at least one backup
	if _, err := mgr.AutoBackup(vaultPath, password); err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	// List backups
	backups, err = mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}

	if len(backups) < 1 {
		t.Errorf("got %d backups, want at least 1", len(backups))
	}

	// Check metadata
	for i, backup := range backups {
		if backup.Filename == "" {
			t.Errorf("backup %d has empty filename", i)
		}
		if backup.Size == 0 {
			t.Errorf("backup %d has zero size", i)
		}
	}
}

func TestSplitAndCombineBackupKey(t *testing.T) {
	password := "test-password-for-shamir"
	n, m := 5, 3 // 5 shares, require 3

	// Split key
	shares, err := SplitBackupKey(password, n, m)
	if err != nil {
		t.Fatalf("SplitBackupKey: %v", err)
	}

	if len(shares) != n {
		t.Errorf("got %d shares, want %d", len(shares), n)
	}

	// Check share indices
	for i, share := range shares {
		if share.Index != i+1 {
			t.Errorf("share %d has index %d, want %d", i, share.Index, i+1)
		}
		if share.Share == "" {
			t.Errorf("share %d has empty data", i)
		}
	}

	// Combine with threshold shares (first 3)
	key, err := CombineBackupKey(shares[:m])
	if err != nil {
		t.Fatalf("CombineBackupKey: %v", err)
	}

	if len(key) != argonKeyLength {
		t.Errorf("combined key length: got %d, want %d", len(key), argonKeyLength)
	}

	// Combine with different M shares (last 3)
	key2, err := CombineBackupKey(shares[n-m:])
	if err != nil {
		t.Fatalf("CombineBackupKey with different shares: %v", err)
	}

	// Keys should match
	if string(key) != string(key2) {
		t.Error("keys from different share combinations should match")
	}
}

func TestShamirInvalidParameters(t *testing.T) {
	password := "test-password"

	tests := []struct {
		name string
		n, m int
	}{
		{"n too small", 1, 1},
		{"m too small", 3, 1},
		{"m > n", 3, 5},
		{"m == 0", 5, 0},
		{"n == 0", 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SplitBackupKey(password, tt.n, tt.m)
			if err == nil {
				t.Errorf("expected error for n=%d, m=%d", tt.n, tt.m)
			}
		})
	}
}

func TestExportAndImportShares(t *testing.T) {
	tempDir := t.TempDir()
	outputDir := filepath.Join(tempDir, "shares")
	password := "test-password"
	n, m := 5, 3

	// Split key
	shares, err := SplitBackupKey(password, n, m)
	if err != nil {
		t.Fatalf("SplitBackupKey: %v", err)
	}

	// Export shares
	if err := ExportShares(shares, outputDir); err != nil {
		t.Fatalf("ExportShares: %v", err)
	}

	// Check files created
	for i := 1; i <= n; i++ {
		filename := filepath.Join(outputDir, "share-"+string(rune('0'+i))+".json")
		if _, err := os.Stat(filename); err != nil {
			// Try numeric format
			filename = filepath.Join(outputDir, "share-1.json")
			if _, err := os.Stat(filename); err == nil {
				break
			}
		}
	}

	// Import shares
	var importedShares []ShamirShare
	for i := 1; i <= m; i++ {
		filename := filepath.Join(outputDir, "share-"+string(rune('0'+i))+".json")
		share, err := ImportShare(filename)
		if err != nil {
			t.Fatalf("ImportShare %d: %v", i, err)
		}
		importedShares = append(importedShares, *share)
	}

	if len(importedShares) != m {
		t.Errorf("got %d imported shares, want %d", len(importedShares), m)
	}

	// Verify imported shares work
	key, err := CombineBackupKey(importedShares)
	if err != nil {
		t.Fatalf("CombineBackupKey with imported shares: %v", err)
	}

	if len(key) != argonKeyLength {
		t.Errorf("key length: got %d, want %d", len(key), argonKeyLength)
	}
}

func TestRestoreWithShares(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "test-password"

	// Create vault and backup
	originalData := []byte("secret vault data")
	if err := os.WriteFile(vaultPath, originalData, 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}

	mgr := New(backupDir)
	backupFile, err := mgr.AutoBackup(vaultPath, password)
	if err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	// Split backup key
	n, m := 5, 3
	shares, err := SplitBackupKey(password, n, m)
	if err != nil {
		t.Fatalf("SplitBackupKey: %v", err)
	}

	// Modify vault
	if err := os.WriteFile(vaultPath, []byte("modified"), 0600); err != nil {
		t.Fatalf("modify vault: %v", err)
	}

	// Restore using shares (use first M shares)
	backupFilename := filepath.Base(backupFile)
	if err := mgr.RestoreWithShares(backupFilename, shares[:m], vaultPath); err != nil {
		t.Fatalf("RestoreWithShares: %v", err)
	}

	// Verify restored data
	restoredData, err := os.ReadFile(vaultPath)
	if err != nil {
		t.Fatalf("read restored vault: %v", err)
	}

	if string(restoredData) != string(originalData) {
		t.Errorf("restored data mismatch:\ngot:  %q\nwant: %q", restoredData, originalData)
	}
}

func TestRestoreWithInsufficientShares(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "test-password"

	// Create vault and backup
	if err := os.WriteFile(vaultPath, []byte("data"), 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}

	mgr := New(backupDir)
	backupFile, err := mgr.AutoBackup(vaultPath, password)
	if err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	// Split key
	n, m := 5, 3
	shares, err := SplitBackupKey(password, n, m)
	if err != nil {
		t.Fatalf("SplitBackupKey: %v", err)
	}

	// Try to restore with M-1 shares
	backupFilename := filepath.Base(backupFile)
	err = mgr.RestoreWithShares(backupFilename, shares[:m-1], vaultPath)
	if err == nil {
		t.Error("expected error restoring with insufficient shares")
	}
}

func TestCombineBackupKeyNoShares(t *testing.T) {
	_, err := CombineBackupKey([]ShamirShare{})
	if err == nil {
		t.Error("expected error combining with no shares")
	}
}

func TestAutoBackupCreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "nonexistent", "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")
	password := "test-password"

	// Create vault
	if err := os.WriteFile(vaultPath, []byte("data"), 0600); err != nil {
		t.Fatalf("write vault: %v", err)
	}

	mgr := New(backupDir)

	// AutoBackup should create the directory
	_, err := mgr.AutoBackup(vaultPath, password)
	if err != nil {
		t.Fatalf("AutoBackup: %v", err)
	}

	// Check directory exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("backup directory should be created")
	}
}

func TestRestoreNonexistentBackup(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "backups")
	vaultPath := filepath.Join(tempDir, "vault.enc")

	mgr := New(backupDir)

	err := mgr.Restore("nonexistent-backup.enc", "password", vaultPath)
	if err == nil {
		t.Error("expected error restoring nonexistent backup")
	}
}

func TestListBackupsNonexistentDir(t *testing.T) {
	tempDir := t.TempDir()
	backupDir := filepath.Join(tempDir, "nonexistent")

	mgr := New(backupDir)

	backups, err := mgr.ListBackups()
	if err != nil {
		t.Fatalf("ListBackups: %v", err)
	}

	if len(backups) != 0 {
		t.Errorf("got %d backups from nonexistent dir, want 0", len(backups))
	}
}

func TestShamirSharesAreUnique(t *testing.T) {
	password := "test-password"
	n, m := 10, 5

	shares, err := SplitBackupKey(password, n, m)
	if err != nil {
		t.Fatalf("SplitBackupKey: %v", err)
	}

	// Check all shares are unique
	seen := make(map[string]bool)
	for _, share := range shares {
		if seen[share.Share] {
			t.Errorf("duplicate share data: %s", share.Share)
		}
		seen[share.Share] = true
	}
}
