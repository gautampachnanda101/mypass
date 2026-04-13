package resolver

import (
	"context"
	"strings"
	"testing"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/providers"
)

// stubProvider is an in-memory provider for testing.
type stubProvider struct {
	id      string
	secrets map[string]string
}

func (s *stubProvider) ID() string { return s.id }
func (s *stubProvider) Health(_ context.Context) error { return nil }
func (s *stubProvider) List(_ context.Context, _ string) ([]providers.Secret, error) {
	return nil, nil
}
func (s *stubProvider) Get(_ context.Context, path string) (providers.Secret, error) {
	v, ok := s.secrets[path]
	if !ok {
		return providers.Secret{}, &providers.ErrNotFound{Provider: s.id, Path: path}
	}
	return providers.Secret{Key: path, Value: v, Provider: s.id}, nil
}

func newStub(id string, kv ...string) *stubProvider {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return &stubProvider{id: id, secrets: m}
}

func TestResolveLiteral(t *testing.T) {
	reg := NewRegistry()
	f, _ := envfile.Parse(strings.NewReader("PORT=3000\n"))
	got, err := reg.Resolve(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if got["PORT"] != "3000" {
		t.Fatalf("got %q want %q", got["PORT"], "3000")
	}
}

func TestResolveVaultRef(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("local", "myapp/db", "s3cr3t"), true)

	f, _ := envfile.Parse(strings.NewReader("DB=vault:local/myapp/db\n"))
	got, err := reg.Resolve(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if got["DB"] != "s3cr3t" {
		t.Fatalf("got %q want %q", got["DB"], "s3cr3t")
	}
}

func TestResolveBareRefUsesDefault(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("local", "myapp/key", "value"), true)

	// "vault:myapp/key" — no provider prefix, uses default
	f, _ := envfile.Parse(strings.NewReader("K=vault:myapp/key\n"))
	got, err := reg.Resolve(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if got["K"] != "value" {
		t.Fatalf("got %q want %q", got["K"], "value")
	}
}

func TestResolveMultiProvider(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("local", "db/pass", "local-pass"), true)
	reg.Register(newStub("work", "Vault/api-key", "op-key"), false)

	src := "DB=vault:local/db/pass\nAPI=vault:work/Vault/api-key\n"
	f, _ := envfile.Parse(strings.NewReader(src))
	got, err := reg.Resolve(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}
	if got["DB"] != "local-pass" || got["API"] != "op-key" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestResolveUnknownProviderErrors(t *testing.T) {
	reg := NewRegistry()
	f, _ := envfile.Parse(strings.NewReader("K=vault:missing/path\n"))
	_, err := reg.Resolve(context.Background(), f)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestResolveSecretNotFoundErrors(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("local"), true)
	f, _ := envfile.Parse(strings.NewReader("K=vault:local/no/such/key\n"))
	_, err := reg.Resolve(context.Background(), f)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestGetConvenience(t *testing.T) {
	reg := NewRegistry()
	reg.Register(newStub("local", "app/token", "tok"), true)

	val, err := reg.Get(context.Background(), "local/app/token")
	if err != nil || val != "tok" {
		t.Fatalf("Get: val=%q err=%v", val, err)
	}
}
