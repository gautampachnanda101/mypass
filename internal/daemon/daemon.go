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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/resolver"
)

const tokenFile = "daemon.token"

// Server is the vaultx daemon HTTP server.
type Server struct {
	registry *resolver.Registry
	token    string
	port     int
	srv      *http.Server
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

	s := &Server{registry: registry, token: token, port: port}
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
	mux.HandleFunc("/v1/secret", s.auth(s.handleGetSecret))
	mux.HandleFunc("/v1/resolve", s.auth(s.handleResolve))
	mux.HandleFunc("/v1/list", s.auth(s.handleList))
	// External Secrets Operator webhook — path variable extracted manually.
	mux.HandleFunc("/externalsecrets/", s.auth(s.handleExternalSecrets))
	return mux
}

// auth wraps a handler with token verification.
func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.Header.Get("X-Vaultx-Token")
		if tok == "" {
			tok = r.URL.Query().Get("token") // allow ?token= for ESO webhooks
		}
		if tok != s.token {
			writeError(w, http.StatusUnauthorized, "invalid or missing token")
			return
		}
		next(w, r)
	}
}

// handleHealth returns 200 + seal status. No auth required — safe to expose.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleGetSecret resolves a single vault path.
// GET /v1/secret?path=local/myapp/db
func (s *Server) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path query parameter required")
		return
	}

	val, err := s.registry.Get(r.Context(), path)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"value": val})
}

// handleResolve parses a vaultx.env body and returns resolved KEY=VALUE pairs.
// POST /v1/resolve  body: vaultx.env contents
func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "POST only")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB limit
	if err != nil {
		writeError(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	f, err := envfile.Parse(strings.NewReader(string(body)))
	if err != nil {
		writeError(w, http.StatusBadRequest, "parse env file: "+err.Error())
		return
	}

	resolved, err := s.registry.Resolve(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resolved)
}

// handleList returns secret metadata (no values) for a given prefix.
// GET /v1/list?prefix=myapp/
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}
	prefix := r.URL.Query().Get("prefix")

	// List from all registered providers — first provider wins on duplicates.
	// For now surface only what the registry exposes via the local provider.
	secrets, err := s.registry.List(r.Context(), prefix)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, secrets)
}

// handleExternalSecrets implements the External Secrets Operator webhook protocol.
// GET /externalsecrets/<key>
// Returns: {"value": "<secret value>"}
func (s *Server) handleExternalSecrets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "GET only")
		return
	}

	key := strings.TrimPrefix(r.URL.Path, "/externalsecrets/")
	if key == "" {
		writeError(w, http.StatusBadRequest, "key required in path")
		return
	}

	val, err := s.registry.Get(r.Context(), key)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
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
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(token), 0600)
}

func removeToken() error {
	return os.Remove(tokenPath())
}
