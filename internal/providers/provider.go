package providers

import (
	"context"
	"fmt"
	"time"
)

// Secret is a resolved secret value from any provider.
type Secret struct {
	Key       string
	Value     string
	Version   string
	Provider  string
	UpdatedAt time.Time
}

// Provider is the interface every secret backend must implement.
type Provider interface {
	// ID returns the provider's configured identifier (e.g. "local", "work", "prod").
	ID() string

	// Get resolves a single secret by path. Path format is provider-specific
	// but conventionally uses slash-separated segments: "myapp/db_password".
	Get(ctx context.Context, path string) (Secret, error)

	// List returns all secrets with the given prefix. Pass "" for all secrets.
	List(ctx context.Context, prefix string) ([]Secret, error)

	// Health checks whether the provider is reachable and the vault is unlocked.
	Health(ctx context.Context) error
}

// ErrNotFound is returned by Get when the secret does not exist.
type ErrNotFound struct {
	Provider string
	Path     string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("secret not found: %s/%s", e.Provider, e.Path)
}

// ErrLocked is returned when the local vault is locked.
type ErrLocked struct {
	Provider string
}

func (e *ErrLocked) Error() string {
	return fmt.Sprintf("vault is locked: %s (run: vaultx unlock)", e.Provider)
}

// ErrUnavailable is returned when a remote provider cannot be reached.
type ErrUnavailable struct {
	Provider string
	Cause    error
}

func (e *ErrUnavailable) Error() string {
	return fmt.Sprintf("provider unavailable: %s: %v", e.Provider, e.Cause)
}
