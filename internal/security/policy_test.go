package security

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPolicyManager_RateLimiting(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	// Should allow first MaxAttemptsPerWindow attempts (with successful unlocks to avoid lockout)
	for i := 0; i < MaxAttemptsPerWindow; i++ {
		if err := pm.CheckUnlockAllowed(); err != nil {
			t.Errorf("attempt %d: unexpected error: %v", i+1, err)
		}
		// Record successful attempts to avoid lockout
		if err := pm.RecordUnlockAttempt(true); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	// Next attempt should be rate limited
	if err := pm.CheckUnlockAllowed(); err == nil {
		t.Error("expected rate limit error after MaxAttemptsPerWindow attempts")
	}

	// Wait for window to pass
	time.Sleep(UnlockAttemptWindow + 100*time.Millisecond)

	// Should allow attempts again
	if err := pm.CheckUnlockAllowed(); err != nil {
		t.Errorf("after window expired: unexpected error: %v", err)
	}
}

func TestPolicyManager_Lockout(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	// Record MaxUnlockAttempts - 1 failed attempts
	for i := 0; i < MaxUnlockAttempts-1; i++ {
		if err := pm.CheckUnlockAllowed(); err != nil {
			t.Errorf("attempt %d: unexpected error: %v", i+1, err)
		}
		if err := pm.RecordUnlockAttempt(false); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	// Should not be locked yet
	if pm.IsLockedOut() {
		t.Error("should not be locked out before MaxUnlockAttempts")
	}

	// One more failure should trigger lockout
	if err := pm.RecordUnlockAttempt(false); err != nil {
		t.Fatalf("RecordUnlockAttempt: %v", err)
	}

	// Should be locked now
	if !pm.IsLockedOut() {
		t.Error("should be locked out after MaxUnlockAttempts")
	}

	// CheckUnlockAllowed should return lockout error
	if err := pm.CheckUnlockAllowed(); err == nil {
		t.Error("expected lockout error")
	} else if err != nil {
		t.Logf("lockout error message: %v", err)
	}
}

func TestPolicyManager_LockoutPersistence(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	// Create policy manager and trigger lockout
	pm1, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	for i := 0; i < MaxUnlockAttempts; i++ {
		if err := pm1.RecordUnlockAttempt(false); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	if !pm1.IsLockedOut() {
		t.Fatal("expected lockout after failures")
	}

	// Create new policy manager from same state file
	pm2, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager reload: %v", err)
	}

	// Lockout should persist
	if !pm2.IsLockedOut() {
		t.Error("lockout state should persist across restarts")
	}

	if err := pm2.CheckUnlockAllowed(); err == nil {
		t.Error("expected lockout error after reload")
	}
}

func TestPolicyManager_SuccessfulUnlockClearsFailures(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	// Record some failures
	for i := 0; i < MaxUnlockAttempts-1; i++ {
		if err := pm.RecordUnlockAttempt(false); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	// Successful unlock should reset counter
	if err := pm.RecordUnlockAttempt(true); err != nil {
		t.Fatalf("RecordUnlockAttempt success: %v", err)
	}

	// Can now fail MaxUnlockAttempts times again before lockout
	for i := 0; i < MaxUnlockAttempts; i++ {
		if err := pm.RecordUnlockAttempt(false); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	if !pm.IsLockedOut() {
		t.Error("should be locked out after MaxUnlockAttempts new failures")
	}
}

func TestPolicyManager_ResetLockout(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	// Trigger lockout
	for i := 0; i < MaxUnlockAttempts; i++ {
		if err := pm.RecordUnlockAttempt(false); err != nil {
			t.Fatalf("RecordUnlockAttempt: %v", err)
		}
	}

	if !pm.IsLockedOut() {
		t.Fatal("expected lockout")
	}

	// Reset lockout
	if err := pm.ResetLockout(); err != nil {
		t.Fatalf("ResetLockout: %v", err)
	}

	// Should no longer be locked
	if pm.IsLockedOut() {
		t.Error("should not be locked after reset")
	}

	if err := pm.CheckUnlockAllowed(); err != nil {
		t.Errorf("unexpected error after reset: %v", err)
	}
}

func TestPolicyManager_AutoLock(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	locked := false
	pm.SetAutoLockCallback(func() {
		locked = true
	})

	// Start auto-lock with very short duration for testing
	pm.StartAutoLock()

	// Replace the timer with a shorter one for testing
	pm.StopAutoLock()
	pm.mu.Lock()
	pm.autoLockTimer = time.AfterFunc(100*time.Millisecond, func() {
		pm.autoLockFunc()
	})
	pm.mu.Unlock()

	// Wait for auto-lock to trigger
	time.Sleep(200 * time.Millisecond)

	if !locked {
		t.Error("auto-lock callback should have been called")
	}
}

func TestPolicyManager_RecordActivityResetsAutoLock(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	locked := false
	pm.SetAutoLockCallback(func() {
		locked = true
	})

	// Set short timer for testing
	pm.mu.Lock()
	pm.autoLockTimer = time.AfterFunc(150*time.Millisecond, func() {
		pm.autoLockFunc()
	})
	pm.mu.Unlock()

	// Record activity after 100ms (before timer expires)
	time.Sleep(100 * time.Millisecond)
	pm.RecordActivity()

	// Wait another 100ms (total 200ms, but timer was reset at 100ms)
	time.Sleep(100 * time.Millisecond)

	// Should not be locked yet (timer was reset)
	if locked {
		t.Error("auto-lock should not trigger after activity reset")
	}

	// Clean up
	pm.StopAutoLock()
}

func TestPolicyManager_StopAutoLock(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "policy.json")

	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager: %v", err)
	}

	locked := false
	pm.SetAutoLockCallback(func() {
		locked = true
	})

	// Start with short timer
	pm.mu.Lock()
	pm.autoLockTimer = time.AfterFunc(100*time.Millisecond, func() {
		pm.autoLockFunc()
	})
	pm.mu.Unlock()

	// Stop auto-lock
	pm.StopAutoLock()

	// Wait for what would have been expiry
	time.Sleep(200 * time.Millisecond)

	// Should not be locked
	if locked {
		t.Error("auto-lock should not trigger after StopAutoLock")
	}
}

func TestPolicyManager_LoadNonexistentFile(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "nonexistent.json")

	// Should succeed even if file doesn't exist
	pm, err := NewPolicyManager(stateFile)
	if err != nil {
		t.Fatalf("NewPolicyManager with nonexistent file: %v", err)
	}

	// Should start with clean state
	if pm.IsLockedOut() {
		t.Error("fresh policy manager should not be locked out")
	}

	if err := pm.CheckUnlockAllowed(); err != nil {
		t.Errorf("fresh policy manager should allow unlocks: %v", err)
	}
}

func TestPolicyManager_CorruptedStateFile(t *testing.T) {
	tempDir := t.TempDir()
	stateFile := filepath.Join(tempDir, "corrupt.json")

	// Write corrupted JSON
	if err := os.WriteFile(stateFile, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupted file: %v", err)
	}

	// Should return error
	_, err := NewPolicyManager(stateFile)
	if err == nil {
		t.Error("expected error loading corrupted state file")
	}
}
