package onepassword

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/providers"
)

const (
	pathFmt    = "got vault=%q item=%q field=%q"
	valFmt     = "got %q want %q"
	notFound   = "not found"
)

// stub returns a Provider whose exec call returns the given bytes/error.
func stub(id, vault string, response []byte, err error) *Provider {
	p := New(id, "", vault)
	p.exec = func(_ context.Context, _ ...string) ([]byte, error) {
		return response, err
	}
	return p
}

// --- parsePath ---

func TestParsePathThreeSegments(t *testing.T) {
	p := New("work", "", "Work")
	vault, item, field := p.parsePath("MyVault/MyItem/username")
	if vault != "MyVault" || item != "MyItem" || field != "username" {
		t.Fatalf(pathFmt, vault, item, field)
	}
}

func TestParsePathTwoSegments(t *testing.T) {
	p := New("work", "", "Work")
	vault, item, field := p.parsePath("MyVault/MyItem")
	if vault != "MyVault" || item != "MyItem" || field != defaultField {
		t.Fatalf(pathFmt, vault, item, field)
	}
}

func TestParsePathOneSegmentUsesConfiguredVault(t *testing.T) {
	p := New("work", "", "Work")
	vault, item, field := p.parsePath("MyItem")
	if vault != "Work" || item != "MyItem" || field != defaultField {
		t.Fatalf(pathFmt, vault, item, field)
	}
}

func TestParsePathOneSegmentFallsBackToPrivate(t *testing.T) {
	p := New("work", "", "")
	vault, _, _ := p.parsePath("MyItem")
	if vault != "Private" {
		t.Fatalf("expected fallback vault 'Private', got %q", vault)
	}
}

// --- extractField ---

func TestExtractFieldSingleResponse(t *testing.T) {
	data := []byte(`{"value":"hunter2"}`)
	val, _, err := extractField(data, "password")
	if err != nil {
		t.Fatalf("extractField: %v", err)
	}
	if val != "hunter2" {
		t.Fatalf(valFmt, val, "hunter2")
	}
}

func TestExtractFieldFullItemResponse(t *testing.T) {
	data := []byte(`{
		"updated_at": "2024-01-01T00:00:00Z",
		"fields": [
			{"label": "username", "id": "username", "value": "alice"},
			{"label": "password", "id": "password", "value": "s3cr3t"}
		]
	}`)
	val, updatedAt, err := extractField(data, "password")
	if err != nil {
		t.Fatalf("extractField: %v", err)
	}
	if val != "s3cr3t" {
		t.Fatalf(valFmt, val, "s3cr3t")
	}
	if updatedAt.IsZero() {
		t.Fatal("expected non-zero updatedAt")
	}
}

func TestExtractFieldByID(t *testing.T) {
	data := []byte(`{"fields": [{"label": "API Key", "id": "apikey", "value": "tok3n"}]}`)
	val, _, err := extractField(data, "apikey")
	if err != nil || val != "tok3n" {
		t.Fatalf("match by ID: val=%q err=%v", val, err)
	}
}

func TestExtractFieldCaseInsensitive(t *testing.T) {
	data := []byte(`{"fields": [{"label": "Password", "id": "PASSWORD", "value": "abc"}]}`)
	val, _, err := extractField(data, "password")
	if err != nil || val != "abc" {
		t.Fatalf("case-insensitive match: val=%q err=%v", val, err)
	}
}

func TestExtractFieldNotFound(t *testing.T) {
	data := []byte(`{"fields": [{"label": "username", "id": "username", "value": "u"}]}`)
	_, _, err := extractField(data, "password")
	if err == nil {
		t.Fatal("expected error for missing field")
	}
}

// --- isNotFound ---

func TestIsNotFoundPatterns(t *testing.T) {
	patterns := []string{notFound, "isn't an item", "no item"}
	for _, msg := range patterns {
		if !isNotFound(fmt.Errorf("op error: %s", msg), nil) {
			t.Fatalf("expected isNotFound=true for %q", msg)
		}
	}
}

func TestIsNotFoundFalse(t *testing.T) {
	if isNotFound(fmt.Errorf("network timeout"), nil) {
		t.Fatal("network timeout should not be treated as not-found")
	}
}

func TestIsNotFoundNilErr(t *testing.T) {
	if isNotFound(nil, []byte(notFound)) {
		t.Fatal("nil err should return false")
	}
}

// --- Get (stubbed exec) ---

func TestGetReturnsSingleFieldResponse(t *testing.T) {
	p := stub("work", "Work", []byte(`{"value":"tok3n"}`), nil)
	s, err := p.Get(context.Background(), "Work/MyItem/password")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Value != "tok3n" || s.Provider != "work" {
		t.Fatalf("unexpected secret: %+v", s)
	}
}

func TestGetReturnsFullItemResponse(t *testing.T) {
	resp := []byte(`{
		"updated_at": "2024-06-01T12:00:00Z",
		"fields": [{"label": "password", "id": "password", "value": "fullitem-pass"}]
	}`)
	p := stub("work", "Work", resp, nil)
	s, err := p.Get(context.Background(), "Work/MyItem/password")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.Value != "fullitem-pass" {
		t.Fatalf(valFmt, s.Value, "fullitem-pass")
	}
	if s.UpdatedAt == (time.Time{}) {
		t.Fatal("expected non-zero UpdatedAt from full item response")
	}
}

func TestGetNotFoundError(t *testing.T) {
	p := stub("work", "Work", []byte(notFound), fmt.Errorf("op item get: %s", notFound))
	_, err := p.Get(context.Background(), "Work/Missing/password")
	if err == nil {
		t.Fatal("expected error")
	}
	var nf *providers.ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestGetUnavailableError(t *testing.T) {
	p := stub("work", "Work", nil, fmt.Errorf("exit status 1"))
	_, err := p.Get(context.Background(), "Work/Item/password")
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- List (stubbed exec) ---

func TestListReturnsMetadata(t *testing.T) {
	resp := []byte(`[
		{"id":"abc","title":"GitHub","updated_at":"2024-01-01T00:00:00Z"},
		{"id":"def","title":"AWS","updated_at":"2024-02-01T00:00:00Z"}
	]`)
	p := stub("work", "Work", resp, nil)
	secrets, err := p.List(context.Background(), "")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(secrets) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(secrets))
	}
	for _, s := range secrets {
		if s.Value != "" {
			t.Fatalf("List should not return values, got %q for %q", s.Value, s.Key)
		}
	}
}

func TestListPrefixFilter(t *testing.T) {
	resp := []byte(`[
		{"id":"1","title":"GitHub","updated_at":"2024-01-01T00:00:00Z"},
		{"id":"2","title":"AWS","updated_at":"2024-01-01T00:00:00Z"}
	]`)
	p := stub("work", "Work", resp, nil)
	secrets, err := p.List(context.Background(), "Work/G")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(secrets) != 1 || !strings.HasPrefix(secrets[0].Key, "Work/G") {
		t.Fatalf("prefix filter failed: %+v", secrets)
	}
}
