package memprotect

import (
	"bytes"
	"testing"
)

func TestLock_EmptySlice(t *testing.T) {
	var empty []byte

	unlock, err := Lock(empty)
	if err != nil {
		t.Fatalf("Lock empty slice: %v", err)
	}

	if unlock == nil {
		t.Fatal("unlock function should not be nil")
	}

	// Should not panic
	unlock()
}

func TestLock_NonEmptySlice(t *testing.T) {
	data := make([]byte, 4096)
	copy(data, []byte("sensitive secret data"))

	unlock, err := Lock(data)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}

	if unlock == nil {
		t.Fatal("unlock function should not be nil")
	}

	// Data should still be accessible
	if !bytes.Contains(data, []byte("sensitive")) {
		t.Error("locked data should still be readable")
	}

	// Unlock should not panic
	unlock()

	// Data should still be readable after unlock
	if !bytes.Contains(data, []byte("sensitive")) {
		t.Error("data should still be readable after unlock")
	}
}

func TestLock_MultiplePages(t *testing.T) {
	// Allocate multiple pages (typically 4KB per page)
	data := make([]byte, 16*1024) // 16 KB
	copy(data, []byte("test data across multiple pages"))

	unlock, err := Lock(data)
	if err != nil {
		t.Fatalf("Lock multiple pages: %v", err)
	}
	defer unlock()

	// Verify data is intact
	if !bytes.Contains(data, []byte("test data")) {
		t.Error("data should be readable")
	}
}

func TestZero(t *testing.T) {
	data := []byte("secret password 123")
	original := make([]byte, len(data))
	copy(original, data)

	// Verify data is not zeroed initially
	if bytes.Equal(data, make([]byte, len(data))) {
		t.Fatal("test data should not be zero initially")
	}

	// Zero the data
	Zero(data)

	// Verify all bytes are zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d not zeroed: got %d", i, b)
		}
	}

	// Verify original is unchanged (sanity check)
	if bytes.Equal(original, make([]byte, len(original))) {
		t.Error("original should not be affected")
	}
}

func TestZero_EmptySlice(t *testing.T) {
	var empty []byte

	// Should not panic
	Zero(empty)
}

func TestZero_AlreadyZero(t *testing.T) {
	data := make([]byte, 100)

	// Should not panic or error
	Zero(data)

	// Should still be zero
	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d not zero: got %d", i, b)
		}
	}
}

func TestLockAndZero(t *testing.T) {
	// Common use case: lock sensitive data, use it, then zero it
	password := []byte("super-secret-password")

	unlock, err := Lock(password)
	if err != nil {
		t.Fatalf("Lock: %v", err)
	}
	defer unlock()

	// Simulate using the password
	if !bytes.Contains(password, []byte("secret")) {
		t.Error("password should be readable while locked")
	}

	// Zero before unlock
	Zero(password)

	// Verify zeroed
	for i, b := range password {
		if b != 0 {
			t.Errorf("byte %d not zeroed: got %d", i, b)
		}
	}

	unlock()
}

func TestMultipleLocks(t *testing.T) {
	// Test locking multiple separate regions
	data1 := make([]byte, 4096)
	data2 := make([]byte, 4096)
	copy(data1, []byte("data1"))
	copy(data2, []byte("data2"))

	unlock1, err := Lock(data1)
	if err != nil {
		t.Fatalf("Lock data1: %v", err)
	}
	defer unlock1()

	unlock2, err := Lock(data2)
	if err != nil {
		t.Fatalf("Lock data2: %v", err)
	}
	defer unlock2()

	// Both should be readable
	if !bytes.Contains(data1, []byte("data1")) {
		t.Error("data1 should be readable")
	}
	if !bytes.Contains(data2, []byte("data2")) {
		t.Error("data2 should be readable")
	}
}

func TestLockTwice(t *testing.T) {
	data := make([]byte, 4096)

	unlock1, err := Lock(data)
	if err != nil {
		t.Fatalf("first Lock: %v", err)
	}
	defer unlock1()

	// Locking the same region twice might succeed or fail depending on OS
	// We just verify it doesn't panic
	unlock2, err := Lock(data)
	if err != nil {
		t.Logf("second Lock failed (expected on some systems): %v", err)
		return
	}
	defer unlock2()
}

func TestZero_LargeSlice(t *testing.T) {
	// Test zeroing a large slice
	data := make([]byte, 1024*1024) // 1 MB
	for i := range data {
		data[i] = byte(i % 256)
	}

	Zero(data)

	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d not zeroed: got %d", i, b)
			if i > 10 {
				break // Don't spam errors
			}
		}
	}
}

// Benchmark tests to ensure performance is reasonable
func BenchmarkLock(b *testing.B) {
	data := make([]byte, 4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unlock, err := Lock(data)
		if err != nil {
			b.Fatalf("Lock: %v", err)
		}
		unlock()
	}
}

func BenchmarkZero(b *testing.B) {
	data := make([]byte, 4096)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Zero(data)
	}
}

func BenchmarkZeroLarge(b *testing.B) {
	data := make([]byte, 1024*1024) // 1 MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Zero(data)
	}
}
