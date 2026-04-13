package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gautampachnanda101/vaultx/internal/providers"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
)

// stubProvider for daemon tests.
type stubProvider struct {
	id      string
	secrets map[string]string
}

func (s *stubProvider) ID() string { return s.id }
func (s *stubProvider) Health(_ context.Context) error { return nil }
func (s *stubProvider) List(_ context.Context, prefix string) ([]providers.Secret, error) {
	var out []providers.Secret
	for k := range s.secrets {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			out = append(out, providers.Secret{Key: k, Provider: s.id})
		}
	}
	return out, nil
}
func (s *stubProvider) Get(_ context.Context, path string) (providers.Secret, error) {
	v, ok := s.secrets[path]
	if !ok {
		return providers.Secret{}, &providers.ErrNotFound{Provider: s.id, Path: path}
	}
	return providers.Secret{Key: path, Value: v, Provider: s.id}, nil
}

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	reg := resolver.NewRegistry()
	reg.Register(&stubProvider{
		id:      "local",
		secrets: map[string]string{"myapp/db": "s3cr3t", "myapp/api": "tok3n"},
	}, true)

	srv := &Server{
		registry: reg,
		token:    "test-token-abc",
		port:     0,
	}
	srv.srv = &http.Server{Handler: srv.routes()}
	ts := httptest.NewServer(srv.routes())
	t.Cleanup(ts.Close)
	return srv, ts
}

func get(t *testing.T, ts *httptest.Server, token, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
	if token != "" {
		req.Header.Set("X-Vaultx-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func post(t *testing.T, ts *httptest.Server, token, path, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, ts.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	if token != "" {
		req.Header.Set("X-Vaultx-Token", token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func readJSON(t *testing.T, r *http.Response) map[string]any {
	t.Helper()
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal response: %v — body: %s", err, b)
	}
	return m
}

// --- health ---

func TestHealthNoAuth(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "", "/health")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- auth ---

func TestAuthMissingToken(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "", "/v1/secret?path=myapp/db")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthWrongToken(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "wrong-token", "/v1/secret?path=myapp/db")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuthTokenViaQueryParam(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "", fmt.Sprintf("/v1/secret?path=myapp/db&token=%s", "test-token-abc"))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 via ?token=, got %d", resp.StatusCode)
	}
}

// --- /v1/secret ---

func TestGetSecretFound(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/v1/secret?path=myapp/db")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	m := readJSON(t, resp)
	if m["value"] != "s3cr3t" {
		t.Fatalf("got %q want %q", m["value"], "s3cr3t")
	}
}

func TestGetSecretNotFound(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/v1/secret?path=missing/key")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetSecretMissingPath(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/v1/secret")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- /v1/resolve ---

func TestResolveEnvFile(t *testing.T) {
	_, ts := newTestServer(t)
	body := "DB=vault:local/myapp/db\nPORT=3000\n"
	resp := post(t, ts, "test-token-abc", "/v1/resolve", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	m := readJSON(t, resp)
	if m["DB"] != "s3cr3t" {
		t.Fatalf("DB: got %q want %q", m["DB"], "s3cr3t")
	}
	if m["PORT"] != "3000" {
		t.Fatalf("PORT: got %q want %q", m["PORT"], "3000")
	}
}

func TestResolveBadEnvFile(t *testing.T) {
	_, ts := newTestServer(t)
	resp := post(t, ts, "test-token-abc", "/v1/resolve", "NOEQUALS\n")
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

// --- /v1/list ---

func TestListSecrets(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/v1/list")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var secrets []map[string]any
	b, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(b, &secrets)
	if len(secrets) < 2 {
		t.Fatalf("expected at least 2 secrets, got %d", len(secrets))
	}
	for _, s := range secrets {
		if s["Value"] != nil && s["Value"] != "" {
			t.Fatalf("List should not return values, got %v", s["Value"])
		}
	}
}

// --- /externalsecrets/ ---

func TestExternalSecretsWebhook(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/externalsecrets/myapp/api")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	m := readJSON(t, resp)
	if m["value"] != "tok3n" {
		t.Fatalf("got %q want %q", m["value"], "tok3n")
	}
}

func TestExternalSecretsNotFound(t *testing.T) {
	_, ts := newTestServer(t)
	resp := get(t, ts, "test-token-abc", "/externalsecrets/missing/key")
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
