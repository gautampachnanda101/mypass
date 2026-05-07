// Package security implements defense-in-depth security policies for vaultx.
package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// MaxUnlockAttempts before lockout is triggered.
	MaxUnlockAttempts = 5
	// LockoutDuration after MaxUnlockAttempts failed unlocks.
	LockoutDuration = 30 * time.Minute
	// UnlockAttemptWindow for rate limiting (10 attempts per 1 minute).
	UnlockAttemptWindow = 1 * time.Minute
	// MaxAttemptsPerWindow limits unlock attempts to prevent brute force.
	MaxAttemptsPerWindow = 10
	// AutoLockIdleTime locks vault after this period of inactivity.
	AutoLockIdleTime = 15 * time.Minute
)

// PolicyManager tracks unlock attempts, enforces rate limiting, and manages lockouts.
type PolicyManager struct {
	mu sync.Mutex

	stateFile string

	// Unlock attempt tracking
	attempts      []time.Time // recent unlock attempts
	failedCount   int         // consecutive failures
	lockedUntil   time.Time   // lockout expiry
	lastActivity  time.Time   // for auto-lock

	// Auto-lock timer
	autoLockTimer *time.Timer
	autoLockFunc  func() // callback to lock vault
}

// policyState is persisted to disk for cross-session tracking.
type policyState struct {
	FailedCount  int       `json:"failed_count"`
	LockedUntil  time.Time `json:"locked_until"`
	LastAttempts []time.Time `json:"last_attempts"`
}

// NewPolicyManager creates a security policy manager.
func NewPolicyManager(stateFile string) (*PolicyManager, error) {
	pm := &PolicyManager{
		stateFile:    stateFile,
		lastActivity: time.Now(),
	}

	if err := pm.load(); err != nil {
		// If file doesn't exist, start fresh
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load policy state: %w", err)
		}
	}

	return pm, nil
}

// SetAutoLockCallback sets the function called when auto-lock timer expires.
func (pm *PolicyManager) SetAutoLockCallback(fn func()) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.autoLockFunc = fn
}

// RecordActivity updates last activity time and resets auto-lock timer.
func (pm *PolicyManager) RecordActivity() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.lastActivity = time.Now()
	pm.resetAutoLockTimer()
}

// CheckUnlockAllowed returns an error if unlock should be denied.
func (pm *PolicyManager) CheckUnlockAllowed() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()

	// Check if currently locked out
	if now.Before(pm.lockedUntil) {
		remaining := pm.lockedUntil.Sub(now).Round(time.Second)
		return fmt.Errorf("account locked due to too many failed attempts; try again in %s", remaining)
	}

	// Clear lockout if expired
	if !pm.lockedUntil.IsZero() && now.After(pm.lockedUntil) {
		pm.failedCount = 0
		pm.lockedUntil = time.Time{}
	}

	// Rate limit: check attempts in recent window
	pm.pruneOldAttempts(now)
	if len(pm.attempts) >= MaxAttemptsPerWindow {
		return fmt.Errorf("too many unlock attempts; please wait before trying again")
	}

	return nil
}

// RecordUnlockAttempt records an unlock attempt (success or failure).
func (pm *PolicyManager) RecordUnlockAttempt(success bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	now := time.Now()
	pm.attempts = append(pm.attempts, now)
	pm.pruneOldAttempts(now)

	if success {
		pm.failedCount = 0
		pm.lockedUntil = time.Time{}
		pm.lastActivity = now
		pm.resetAutoLockTimer()
	} else {
		pm.failedCount++
		if pm.failedCount >= MaxUnlockAttempts {
			pm.lockedUntil = now.Add(LockoutDuration)
		}
	}

	return pm.save()
}

// IsLockedOut reports whether the account is currently locked out.
func (pm *PolicyManager) IsLockedOut() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return time.Now().Before(pm.lockedUntil)
}

// ResetLockout clears lockout state (for emergency recovery).
func (pm *PolicyManager) ResetLockout() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.failedCount = 0
	pm.lockedUntil = time.Time{}
	pm.attempts = nil

	return pm.save()
}

// StartAutoLock starts the auto-lock timer.
func (pm *PolicyManager) StartAutoLock() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.resetAutoLockTimer()
}

// StopAutoLock stops the auto-lock timer.
func (pm *PolicyManager) StopAutoLock() {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if pm.autoLockTimer != nil {
		pm.autoLockTimer.Stop()
		pm.autoLockTimer = nil
	}
}

// resetAutoLockTimer resets the auto-lock countdown (must hold lock).
func (pm *PolicyManager) resetAutoLockTimer() {
	if pm.autoLockTimer != nil {
		pm.autoLockTimer.Stop()
	}

	if pm.autoLockFunc != nil {
		pm.autoLockTimer = time.AfterFunc(AutoLockIdleTime, func() {
			pm.autoLockFunc()
		})
	}
}

// pruneOldAttempts removes attempts outside the rate limit window (must hold lock).
func (pm *PolicyManager) pruneOldAttempts(now time.Time) {
	cutoff := now.Add(-UnlockAttemptWindow)
	var recent []time.Time
	for _, t := range pm.attempts {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	pm.attempts = recent
}

// load reads persisted policy state from disk.
func (pm *PolicyManager) load() error {
	data, err := os.ReadFile(pm.stateFile)
	if err != nil {
		return err
	}

	var state policyState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshal policy state: %w", err)
	}

	pm.failedCount = state.FailedCount
	pm.lockedUntil = state.LockedUntil
	pm.attempts = state.LastAttempts

	return nil
}

// save persists policy state to disk.
func (pm *PolicyManager) save() error {
	state := policyState{
		FailedCount:  pm.failedCount,
		LockedUntil:  pm.lockedUntil,
		LastAttempts: pm.attempts,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy state: %w", err)
	}

	dir := filepath.Dir(pm.stateFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create policy dir: %w", err)
	}

	// Atomic write
	tmpFile := pm.stateFile + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("write policy state: %w", err)
	}

	return os.Rename(tmpFile, pm.stateFile)
}
