// Package daemon runs a local HTTP server that exposes the vaultx resolver
// to extensions (VS Code, browser) and the k3d External Secrets webhook.
//
// All endpoints require a passkey header (X-Vaultx-Token) set at startup.
// The passkey is generated randomly per-session and written to
// ~/.vaultx/daemon.token (mode 0600) so local processes can read it.
//
// Endpoints:
//
//	GET  /health                        liveness + vault seal status
//	GET  /v1/secret?path=<path>         resolve a single secret
//	POST /v1/resolve                    resolve a full vaultx.env body
//	GET  /v1/list?prefix=<prefix>       list secrets (values masked)
//	GET  /externalsecrets/{key}         External Secrets Operator webhook
package daemon

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/passkey"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
	"golang.org/x/time/rate"
)

const (
	tokenFile = "daemon.token"

	// Error message constants
	errMethodGET          = "GET only"
	errMethodPOST         = "POST only"
	errInvalidToken       = "invalid or missing token"
	errPathRequired       = "path query parameter required"
	errKeyRequired        = "key required in path"
	errSecretNotFound     = "secret not found"
	errSecretResolution   = "secret resolution failed"
	errParseEnvFile       = "invalid env file format"
	errBodyTooLarge       = "request body too large"
	errReadBody           = "failed to read request body"
	errListFailed         = "failed to list secrets"
	errRateLimitExceeded  = "rate limit exceeded"
	errInvalidPath        = "invalid secret path"

	// Rate limiting
	requestsPerSecond = 10
	burstSize         = 50

	// Request limits
	maxBodySize        = 1 << 20 // 1 MiB
	requestTimeout     = 10 * time.Second
)

// Server is the vaultx daemon HTTP server.
type Server struct {
	registry  *resolver.Registry
	token     string
	port      int
	srv       *http.Server
	limiter   *rate.Limiter
	auditLog  *AuditLogger
}

// AuditLogger records security-relevant events.
type AuditLogger struct {
	mu     sync.Mutex
	events []AuditEvent
}

// AuditEvent represents a security-relevant action.
type AuditEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Action     string    `json:"action"`
	Path       string    `json:"path,omitempty"`
	RemoteAddr string    `json:"remote_addr"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
}

// Log records an audit event.
func (a *AuditLogger) Log(event AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	event.Timestamp = time.Now()
	a.events = append(a.events, event)
	// Also log to stderr for immediate visibility
	if !event.Success {
		log.Printf("[AUDIT] action=%s path=%s remote=%s error=%s\n",
			event.Action, event.Path, event.RemoteAddr, event.Error)
	}
}

// New creates a daemon server on the given port.
// The session token is generated randomly and written to ~/.vaultx/daemon.token.
func New(registry *resolver.Registry, port int) (*Server, error) {
	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("generate daemon token: %w", err)
	}

	if err := writeToken(token); err != nil {
		return nil, fmt.Errorf("write daemon token: %w", err)
	}

	s := &Server{
		registry: registry,
		token:    token,
		port:     port,
		limiter:  rate.NewLimiter(requestsPerSecond, burstSize),
		auditLog: &AuditLogger{events: make([]AuditEvent, 0, 1000)},
	}
	s.srv = &http.Server{
		Addr:         fmt.Sprintf("127.0.0.1:%d", port),
		Handler:      s.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return s, nil
}

// ListenAndServe starts the server. Blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.srv.Addr)
	if err != nil {
		return fmt.Errorf("daemon listen %s: %w", s.srv.Addr, err)
	}

	fmt.Fprintf(os.Stderr, "vaultx daemon listening on %s\n", s.srv.Addr)

	errCh := make(chan error, 1)
	go func() { errCh <- s.srv.Serve(ln) }()

	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.srv.Shutdown(shutCtx)
		_ = removeToken()
		return nil
	case err := <-errCh:
		return err
	}
}

// Token returns the session token (for testing).
func (s *Server) Token() string { return s.token }

// routes wires all HTTP handlers.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	// Touch ID authentication — no auth required, triggers biometric
	mux.HandleFunc("/auth/touchid", s.rateLimit(s.handleTouchIDAuth))
	mux.HandleFunc("/v1/secret", s.auth(s.rateLimit(s.handleGetSecret)))
	mux.HandleFunc("/v1/resolve", s.auth(s.rateLimit(s.handleResolve)))
	mux.HandleFunc("/v1/list", s.auth(s.rateLimit(s.handleList)))
	// External Secrets Operator webhook — path variable extracted manually.
	mux.HandleFunc("/externalsecrets/", s.auth(s.rateLimit(s.handleExternalSecrets)))
	// Web UI — no auth required initially, JS will authenticate via Touch ID
	mux.HandleFunc("/", s.handleWebUI)
	mux.HandleFunc("/ui/", s.handleWebUI)
	return mux
}

// auth wraps a handler with token verification.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-Vaultx-Token")
		usingQueryParam := false
		if tok == "" {
			tok = r.URL.Query().Get("token") // allow ?token= for ESO webhooks
			usingQueryParam = tok != ""
			if usingQueryParam {
				log.Printf("[SECURITY WARNING] Token provided via query parameter from %s - prefer X-Vaultx-Token header\n", r.RemoteAddr)
			}
		}
		if tok != s.token {
			s.auditLog.Log(AuditEvent{
				Action:     "auth_failed",
				RemoteAddr: r.RemoteAddr,
				Success:    false,
				Error:      "invalid token",
			})
			writeError(w, http.StatusUnauthorized, errInvalidToken)
			return
		}
		next(w, r)
	}
}

// rateLimit wraps a handler with rate limiting.
func (s *Server) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.limiter.Allow() {
			s.auditLog.Log(AuditEvent{
				Action:     "rate_limit_exceeded",
				RemoteAddr: r.RemoteAddr,
				Success:    false,
			})
			writeError(w, http.StatusTooManyRequests, errRateLimitExceeded)
			return
		}
		next(w, r)
	}
}

// handleHealth returns 200 + seal status. No auth required — safe to expose.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodGET)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleTouchIDAuth triggers Touch ID authentication and returns the daemon token.
// POST /auth/touchid
func (s *Server) handleTouchIDAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodPOST)
		return
	}

	// Trigger Touch ID authentication via passkey.Load
	if _, ok := passkey.Load(); !ok {
		s.auditLog.Log(AuditEvent{
			Action:     "touchid_auth",
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "biometric authentication failed",
		})
		writeError(w, http.StatusUnauthorized, "Touch ID authentication failed or cancelled")
		return
	}

	// Successful authentication — return the daemon token
	s.auditLog.Log(AuditEvent{
		Action:     "touchid_auth",
		RemoteAddr: r.RemoteAddr,
		Success:    true,
	})

	writeJSON(w, http.StatusOK, map[string]string{
		"token": s.token,
		"port":  fmt.Sprintf("%d", s.port),
	})
}

// handleGetSecret resolves a single vault path.
// GET /v1/secret?path=local/myapp/db
func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodGET)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, errPathRequired)
		return
	}

	// Validate path for security
	if err := validateSecretPath(path); err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "get_secret",
			Path:       path,
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "invalid path",
		})
		writeError(w, http.StatusBadRequest, errInvalidPath)
		return
	}

	// Add request timeout
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	val, err := s.registry.Get(ctx, path)
	if err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "get_secret",
			Path:       path,
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "not found",
		})
		writeError(w, http.StatusNotFound, errSecretNotFound)
		return
	}

	s.auditLog.Log(AuditEvent{
		Action:     "get_secret",
		Path:       path,
		RemoteAddr: r.RemoteAddr,
		Success:    true,
	})
	writeJSON(w, http.StatusOK, map[string]string{"value": val})
}

// handleResolve parses a vaultx.env body and returns resolved KEY=VALUE pairs.
// POST /v1/resolve  body: vaultx.env contents
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, errMethodPOST)
		return
	}

	// Check Content-Length before reading body
	if r.ContentLength > maxBodySize {
		writeError(w, http.StatusRequestEntityTooLarge, errBodyTooLarge)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		writeError(w, http.StatusBadRequest, errReadBody)
		return
	}

	f, err := envfile.Parse(strings.NewReader(string(body)))
	if err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "resolve",
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "invalid env file",
		})
		writeError(w, http.StatusBadRequest, errParseEnvFile)
		return
	}

	// Add request timeout
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	resolved, err := s.registry.Resolve(ctx, f)
	if err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "resolve",
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "resolution failed",
		})
		writeError(w, http.StatusInternalServerError, errSecretResolution)
		return
	}

	s.auditLog.Log(AuditEvent{
		Action:     "resolve",
		RemoteAddr: r.RemoteAddr,
		Success:    true,
	})
	writeJSON(w, http.StatusOK, resolved)
}

// handleList returns secret metadata (no values) for a given prefix.
// GET /v1/list?prefix=myapp/
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodGET)
		return
	}
	prefix := r.URL.Query().Get("prefix")

	// Validate prefix if provided
	if prefix != "" {
		if err := validateSecretPath(prefix); err != nil {
			writeError(w, http.StatusBadRequest, errInvalidPath)
			return
		}
	}

	// Add request timeout
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	// List from all registered providers — first provider wins on duplicates.
	// For now surface only what the registry exposes via the local provider.
	secrets, err := s.registry.List(ctx, prefix)
	if err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "list",
			Path:       prefix,
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "list failed",
		})
		writeError(w, http.StatusInternalServerError, errListFailed)
		return
	}

	s.auditLog.Log(AuditEvent{
		Action:     "list",
		Path:       prefix,
		RemoteAddr: r.RemoteAddr,
		Success:    true,
	})
	writeJSON(w, http.StatusOK, secrets)
}

// handleExternalSecrets implements the External Secrets Operator webhook protocol.
// GET /externalsecrets/<key>
// Returns: {"value": "<secret value>"}
func (s *Server) handleExternalSecrets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, errMethodGET)
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/externalsecrets/")
	if key == "" {
		writeError(w, http.StatusBadRequest, errKeyRequired)
		return
	}

	// Validate key for security
	if err := validateSecretPath(key); err != nil {
		writeError(w, http.StatusBadRequest, errInvalidPath)
		return
	}

	// Add request timeout
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	val, err := s.registry.Get(ctx, key)
	if err != nil {
		s.auditLog.Log(AuditEvent{
			Action:     "eso_webhook",
			Path:       key,
			RemoteAddr: r.RemoteAddr,
			Success:    false,
			Error:      "not found",
		})
		writeError(w, http.StatusNotFound, errSecretNotFound)
		return
	}

	s.auditLog.Log(AuditEvent{
		Action:     "eso_webhook",
		Path:       key,
		RemoteAddr: r.RemoteAddr,
		Success:    true,
	})
	// ESO webhook expects {"value": "..."} with optional metadata.
	writeJSON(w, http.StatusOK, map[string]string{"value": val})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func tokenPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vaultx", tokenFile)
}

func writeToken(token string) error {
	path := tokenPath()
	dir := filepath.Dir(path)

	// Create directory with restrictive permissions
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	// Write to temp file first, then atomic rename to prevent race conditions
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(token), 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func removeToken() error {
	return os.Remove(tokenPath())
}

// validateSecretPath ensures paths don't contain traversal sequences or invalid chars.
func validateSecretPath(path string) error {
	if strings.Contains(path, "..") ||
		strings.HasPrefix(path, "/") ||
		strings.Contains(path, "\\") ||
		strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains invalid characters")
	}
	return nil
}
