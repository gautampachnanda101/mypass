package local

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)


// TestMain runs once before all tests and sets lightweight Argon2id params so
// the suite completes in milliseconds. Production params are set at init time
// by DefaultKDFParams and are unaffected by this override.
func TestMain(m *testing.M) {
	OverrideKDFParamsForTesting(1, 64, 1) // t=1, m=64KB, p=1
	os.Exit(m.Run())
}

// --- crypto primitives ---

func TestSealOpenRoundTrip(t *testing.T) {
	key := make([]byte, keyLen)
	for i := range key {
		key[i] = byte(i + 1)
	}

	plaintext := []byte("hunter2")
	ct, err := sealBytes(plaintext, key)
	if err != nil {
		t.Fatalf("sealBytes: %v", err)
	}

	got, err := openBytes(ct, key)
	if err != nil {
		t.Fatalf("openBytes: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestOpenWrongKey(t *testing.T) {
	key := bytes.Repeat([]byte{0xAA}, keyLen)
	ct, _ := sealBytes([]byte("secret"), key)

	wrongKey := bytes.Repeat([]byte{0xBB}, keyLen)
	_, err := openBytes(ct, wrongKey)
	if err == nil {
		t.Fatal("expected error for wrong key, got nil")
	}
}

func TestSealProducesUniqueOutput(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, keyLen)
	c1, _ := sealBytes([]byte("same"), key)
	c2, _ := sealBytes([]byte("same"), key)
	if bytes.Equal(c1, c2) {
		t.Fatal("two seals of the same plaintext should differ (random nonce)")
	}
}

// --- header seal/unseal ---

func TestInitUnsealHeader(t *testing.T) {
	h, ek, err := initHeader("correct-horse")
	if err != nil {
		t.Fatalf("initHeader: %v", err)
	}
	defer zeroBytes(ek)

	got, err := unsealHeader(h, "correct-horse")
	if err != nil {
		t.Fatalf("unsealHeader: %v", err)
	}
	defer zeroBytes(got)

	if !bytes.Equal(ek, got) {
		t.Fatal("EK mismatch after unseal")
	}
}

func TestUnsealWrongPassword(t *testing.T) {
	h, ek, _ := initHeader("right-password")
	zeroBytes(ek)

	_, err := unsealHeader(h, "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestHeaderTamperDetected(t *testing.T) {
	h, ek, _ := initHeader("password")
	zeroBytes(ek)

	// Tamper with the KDF salt — should break the HMAC.
	h.KDFParams.SaltB64 = "AAAAAAAAAAAAAAAAAAAAAA=="

	_, err := unsealHeader(h, "password")
	if err == nil {
		t.Fatal("expected error after header tampering")
	}
}

func TestRewrapKey(t *testing.T) {
	h, ek, err := initHeader("old-password")
	if err != nil {
		t.Fatalf("initHeader: %v", err)
	}
	defer zeroBytes(ek)

	newH, err := rewrapKey(h, ek, "new-password")
	if err != nil {
		t.Fatalf("rewrapKey: %v", err)
	}

	// Old password must no longer work.
	_, err = unsealHeader(newH, "old-password")
	if err == nil {
		t.Fatal("old password should fail after rewrap")
	}

	// New password must return the same EK.
	got, err := unsealHeader(newH, "new-password")
	if err != nil {
		t.Fatalf("unseal with new password: %v", err)
	}
	defer zeroBytes(got)

	if !bytes.Equal(ek, got) {
		t.Fatal("EK changed after rewrap — entries would be unreadable")
	}
}

// --- full provider integration ---

const (
	vaultFile    = "vault.enc"
	gotWantFmt   = "got %q want %q"
)

func TestProviderInitGetSet(t *testing.T) {
	dir := t.TempDir()
	p := New("test", filepath.Join(dir, vaultFile))

	if err := p.Init("master-password"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	ctx := context.Background()
	if err := p.Set(ctx, "myapp/db", "s3cr3t"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	s, err := p.Get(ctx, "myapp/db")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Value != "s3cr3t" {
		t.Fatalf(gotWantFmt, s.Value, "s3cr3t")
	}
}

func TestProviderLockUnlock(t *testing.T) {
	dir := t.TempDir()
	p := New("test", filepath.Join(dir, vaultFile))
	ctx := context.Background()

	if err := p.Init("pass"); err != nil {
		t.Fatal(err)
	}
	_ = p.Set(ctx, "k", "v")
	p.Lock()

	if !p.IsSealed() {
		t.Fatal("expected vault to be sealed after Lock")
	}
	_, err := p.Get(ctx, "k")
	if err == nil {
		t.Fatal("Get should fail when sealed")
	}

	if err := p.Unlock("pass"); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	s, err := p.Get(ctx, "k")
	if err != nil {
		t.Fatalf("Get after unlock: %v", err)
	}
	if s.Value != "v" {
		t.Fatalf(gotWantFmt, s.Value, "v")
	}
}

func TestProviderChangePassword(t *testing.T) {
	dir := t.TempDir()
	p := New("test", filepath.Join(dir, vaultFile))
	ctx := context.Background()

	_ = p.Init("old")
	_ = p.Set(ctx, "key", "value")

	if err := p.ChangePassword("new"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	// Simulate restart: create a new provider and unlock with new password.
	p2 := New("test", p.path)
	if err := p2.Unlock("new"); err != nil {
		t.Fatalf("Unlock with new password: %v", err)
	}
	s, err := p2.Get(ctx, "key")
	if err != nil {
		t.Fatalf("Get after password change: %v", err)
	}
	if s.Value != "value" {
		t.Fatalf(gotWantFmt, s.Value, "value")
	}

	// Old password must be rejected.
	p3 := New("test", p.path)
	if err := p3.Unlock("old"); err == nil {
		t.Fatal("old password should be rejected after ChangePassword")
	}
}

func TestProviderDelete(t *testing.T) {
	dir := t.TempDir()
	p := New("test", filepath.Join(dir, vaultFile))
	ctx := context.Background()

	_ = p.Init("pass")
	_ = p.Set(ctx, "a", "1")
	_ = p.Set(ctx, "b", "2")

	if err := p.Delete(ctx, "a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := p.Get(ctx, "a")
	if err == nil {
		t.Fatal("Get deleted key should fail")
	}

	s, err := p.Get(ctx, "b")
	if err != nil || s.Value != "2" {
		t.Fatalf("other key should still be accessible: %v", err)
	}
}

func TestProviderListMasksValues(t *testing.T) {
	dir := t.TempDir()
	p := New("test", filepath.Join(dir, vaultFile))
	ctx := context.Background()

	_ = p.Init("pass")
	_ = p.Set(ctx, "app/secret", "topsecret")

	secrets, err := p.List(ctx, "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, s := range secrets {
		if s.Value != "" {
			t.Fatalf("List should not return values, got %q for key %q", s.Value, s.Key)
		}
	}
}
