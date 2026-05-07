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
	"golang.org/x/time/rate"
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
		limiter:  rate.NewLimiter(requestsPerSecond, burstSize),
		auditLog: &AuditLogger{events: make([]AuditEvent, 0, 1000)},
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

// --- AuditLogger ---

func TestAuditLogger_LogAndGetEvents(t *testing.T) {
	al := &AuditLogger{events: make([]AuditEvent, 0, 1000)}
	
	al.Log(AuditEvent{
		Action:     "get_secret",
		Path:       "myapp/db",
		RemoteAddr: "127.0.0.1",
		Success:    true,
	})
	al.Log(AuditEvent{
		Action:     "get_secret",
		Path:       "myapp/api",
		RemoteAddr: "127.0.0.1",
		Success:    true,
	})
	al.Log(AuditEvent{
		Action:     "get_secret",
		Path:       "missing",
		RemoteAddr: "127.0.0.1",
		Success:    false,
	})
	
	events := al.GetEvents(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	
	// Events are returned in reverse order (most recent first)
	// So events[0] is the last one logged (the failed one)
	if events[0].Success {
		t.Errorf("expected first returned event (most recent) to have success=false")
	}
	if events[0].Path != "missing" {
		t.Errorf("expected first event path=missing, got %q", events[0].Path)
	}
	
	// Last returned event is the first one logged
	if !events[2].Success {
		t.Errorf("expected last returned event (oldest) to have success=true")
	}
	if events[2].Path != "myapp/db" {
		t.Errorf("expected last event path=myapp/db, got %q", events[2].Path)
	}
}

func TestAuditLogger_GetEventsLimit(t *testing.T) {
	al := &AuditLogger{events: make([]AuditEvent, 0, 100)}
	
	// Add 10 events
	for i := 0; i < 10; i++ {
		al.Log(AuditEvent{
			Action:     "get_secret",
			Path:       fmt.Sprintf("/path%d", i),
			RemoteAddr: "127.0.0.1",
			Success:    true,
		})
	}
	
	// Request only the 5 most recent
	events := al.GetEvents(5)
	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	
	// Most recent should be path9 (last logged)
	if events[0].Path != "/path9" {
		t.Errorf("expected first event (most recent) path=/path9, got %s", events[0].Path)
	}
	
	// Oldest of the 5 returned should be path5
	if events[4].Path != "/path5" {
		t.Errorf("expected last event (5th most recent) path=/path5, got %s", events[4].Path)
	}
}



// --- validateSecretPath ---

func TestValidateSecretPath(t *testing.T) {
	tests := []struct {
		path  string
		valid bool
	}{
		{"myapp/db", true},
		{"prod/api/key", true},
		{"simple", true},
		{"", true},                  // Empty is allowed (will fail later in provider)
		{"./local", true},           // Leading ./ is allowed
		{"../etc/passwd", false},    // .. is not allowed
		{"path/../other", false},    // .. anywhere is not allowed
		{"/absolute/path", false},   // Leading / is not allowed
		{"path\\with\\backslash", false}, // Backslashes not allowed
	}
	
	for _, tt := range tests {
		err := validateSecretPath(tt.path)
		if tt.valid && err != nil {
			t.Errorf("validateSecretPath(%q) should be valid but got error: %v", tt.path, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("validateSecretPath(%q) should be invalid but got no error", tt.path)
		}
	}
}

// --- Rate Limiting ---

func TestRateLimitExceeded(t *testing.T) {
	srv, ts := newTestServer(t)
	
	// Set very low rate limit for testing
	srv.limiter = rate.NewLimiter(1, 1) // 1 req/sec, burst 1
	
	// First request should succeed
	resp1 := get(t, ts, "test-token-abc", "/v1/secret?path=myapp/db")
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", resp1.StatusCode)
	}
	
	// Immediate second request should be rate limited
	resp2 := get(t, ts, "test-token-abc", "/v1/secret?path=myapp/api")
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", resp2.StatusCode)
	}
}

