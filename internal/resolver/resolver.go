// Package resolver resolves vault: references from a vaultx.env file,
// fanning out to the appropriate provider for each reference.
//
// Reference format:  vault:<provider-id>/<path>
//
// If no provider-id is given (bare path after vaultPrefix), the default
// provider is used.
package resolver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gautampachnanda101/vaultx/internal/envfile"
	"github.com/gautampachnanda101/vaultx/internal/providers"
)

const vaultPrefix = "vault:"

// Registry holds a set of named providers.
type Registry struct {
	providers map[string]providers.Provider
	defaultID string
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]providers.Provider{}}
}

// Register adds a provider. If isDefault is true it is used for bare vault: refs.
func (r *Registry) Register(p providers.Provider, isDefault bool) {
	r.providers[p.ID()] = p
	if isDefault || r.defaultID == "" {
		r.defaultID = p.ID()
	}
}

// Resolve takes a parsed envfile and returns a map of KEY→resolved_value
// suitable for injecting into a process environment.
//
// Plain literals are passed through unchanged.
// ${VAR} entries are interpolated from the caller's environment.
// vault: references are resolved from the appropriate provider.
func (r *Registry) Resolve(ctx context.Context, f *envfile.File) (map[string]string, error) {
	out := make(map[string]string, len(f.Entries))

	for _, e := range f.Entries {
		switch e.Kind {
		case envfile.KindLiteral:
			out[e.Key] = e.Value

		case envfile.KindEnv:
			varName := strings.TrimSuffix(strings.TrimPrefix(e.Value, "${"), "}")
			out[e.Key] = os.Getenv(varName)

		case envfile.KindRef:
			val, err := r.resolveRef(ctx, e.Value)
			if err != nil {
				return nil, fmt.Errorf("line %d: %s: %w", e.Line, e.Key, err)
			}
			out[e.Key] = val
		}
	}

	return out, nil
}

// resolveRef resolves a single vault: reference string.
func (r *Registry) resolveRef(ctx context.Context, ref string) (string, error) {
	// Strip vaultPrefix prefix.
	tail := strings.TrimPrefix(ref, vaultPrefix)

	// Split into providerID and path.
	providerID, path, err := r.splitRef(tail)
	if err != nil {
		return "", err
	}

	p, ok := r.providers[providerID]
	if !ok {
		return "", fmt.Errorf("provider %q not configured (reference: %s)", providerID, ref)
	}

	secret, err := p.Get(ctx, path)
	if err != nil {
		return "", err
	}
	return secret.Value, nil
}

// splitRef splits "providerID/path/to/key" into (providerID, "path/to/key").
// The first slash-delimited segment is treated as a provider ID only if it
// matches a registered provider; otherwise the default provider is used and
// the entire tail is the path. This lets paths like "myapp/db" work against
// the default provider without needing an explicit "local/myapp/db" prefix.
func (r *Registry) splitRef(tail string) (providerID, path string, err error) {
	if r.defaultID == "" {
		return "", "", fmt.Errorf("no default provider configured")
	}
	slash := strings.IndexByte(tail, '/')
	if slash < 0 {
		return r.defaultID, tail, nil
	}
	candidate := tail[:slash]
	if _, ok := r.providers[candidate]; ok {
		return candidate, tail[slash+1:], nil
	}
	// First segment is not a known provider — treat full tail as path under default.
	return r.defaultID, tail, nil
}

// Get resolves a single vault: reference string (convenience wrapper).
func (r *Registry) Get(ctx context.Context, ref string) (string, error) {
	if !strings.HasPrefix(ref, vaultPrefix) {
		ref = vaultPrefix + ref
	}
	return r.resolveRef(ctx, ref)
}
