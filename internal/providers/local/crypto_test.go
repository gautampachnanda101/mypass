package local

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte(`{"version":1,"entries":{"myapp/db":{"value":"s3cr3t"}}}`)
	password := "correct-horse-battery-staple"

	ciphertext, err := encrypt(plaintext, password)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	got, err := decrypt(ciphertext, password)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Fatalf("round-trip mismatch:\n  got  %s\n  want %s", got, plaintext)
	}
}

func TestDecryptWrongPassword(t *testing.T) {
	ciphertext, _ := encrypt([]byte("secret"), "right-password")
	_, err := decrypt(ciphertext, "wrong-password")
	if err == nil {
		t.Fatal("expected error for wrong password, got nil")
	}
}

func TestEncryptProducesUniqueOutput(t *testing.T) {
	plaintext := []byte("same plaintext")
	password := "same-password"

	c1, _ := encrypt(plaintext, password)
	c2, _ := encrypt(plaintext, password)

	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions of the same plaintext should differ (random salt+nonce)")
	}
}
