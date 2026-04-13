// Package onepassword implements a vaultx Provider backed by the 1Password CLI (op).
//
// The op CLI handles authentication — vaultx never touches 1Password credentials.
// Users must have op installed and be signed in (op signin / biometric unlock).
//
// Reference format in vaultx.env:
//
//	SECRET=vault:work/Vault Name/item title           # field: password (default)
//	SECRET=vault:work/Vault Name/item title/field     # explicit field
//	SECRET=vault:work/Vault Name/item uuid/field      # by item UUID
//
// Path parsing: <vault>/<item>[/<field>]
// If <field> is omitted, the "password" field is returned.
package onepassword

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gautampachnanda101/vaultx/internal/providers"
)

const (
	defaultField   = "password"
	flagAccount    = "--account"
	flagVault      = "--vault"
	flagField      = "--field"
	flagFormat     = "--format"
	flagFormatJSON = "json"
)

// execFn is the function type used to exec op. Swappable in tests.
type execFn func(ctx context.Context, args ...string) ([]byte, error)

// Provider resolves secrets from 1Password via the op CLI.
type Provider struct {
	id      string
	account string // op account shorthand (e.g. "my.1password.com")
	vault   string // default vault name/UUID (optional — path can include vault)
	exec    execFn // injectable for tests; nil means use execOp
}

// New creates a 1Password provider.
//
//   - id:      registry identifier (e.g. "work")
//   - account: op account shorthand; empty uses the default signed-in account
//   - vault:   default vault name/UUID; can be overridden per-path
func New(id, account, vault string) *Provider {
	return &Provider{id: id, account: account, vault: vault}
}

func (p *Provider) ID() string { return p.id }

// Health checks that op is installed and the account is accessible.
func (p *Provider) Health(ctx context.Context) error {
	args := []string{"account", "list", flagFormat + "=" + flagFormatJSON}
	if p.account != "" {
		args = append(args, flagAccount, p.account)
	}
	if _, err := p.do(ctx, args...); err != nil {
		return &providers.ErrUnavailable{Provider: p.id, Cause: fmt.Errorf("op not available: %w", err)}
	}
	return nil
}

// Get resolves a single secret. Path format: <vault>/<item>[/<field>]
func (p *Provider) Get(ctx context.Context, path string) (providers.Secret, error) {
	vaultName, item, field := p.parsePath(path)

	args := []string{
		"item", "get", item,
		flagVault, vaultName,
		flagField, field,
		flagFormat, flagFormatJSON,
	}
	if p.account != "" {
		args = append(args, flagAccount, p.account)
	}

	out, err := p.do(ctx, args...)
	if err != nil {
		if isNotFound(err, out) {
			return providers.Secret{}, &providers.ErrNotFound{Provider: p.id, Path: path}
		}
		return providers.Secret{}, &providers.ErrUnavailable{Provider: p.id, Cause: err}
	}

	value, updatedAt, err := extractField(out, field)
	if err != nil {
		return providers.Secret{}, fmt.Errorf("op parse response for %s: %w", path, err)
	}

	return providers.Secret{
		Key:       path,
		Value:     value,
		Provider:  p.id,
		UpdatedAt: updatedAt,
	}, nil
}

// List returns metadata for all items in the configured vault (values not included).
func (p *Provider) List(ctx context.Context, prefix string) ([]providers.Secret, error) {
	vaultArg := p.vaultOrDefault("")

	args := []string{"item", "list", flagVault, vaultArg, flagFormat, flagFormatJSON}
	if p.account != "" {
		args = append(args, flagAccount, p.account)
	}

	out, err := p.do(ctx, args...)
	if err != nil {
		return nil, &providers.ErrUnavailable{Provider: p.id, Cause: err}
	}

	var items []struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("parse op item list: %w", err)
	}

	var secrets []providers.Secret
	for _, it := range items {
		key := vaultArg + "/" + it.Title
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		secrets = append(secrets, providers.Secret{
			Key:       key,
			Provider:  p.id,
			UpdatedAt: it.UpdatedAt,
		})
	}
	return secrets, nil
}

// do dispatches to the injected execFn or the real op binary.
func (p *Provider) do(ctx context.Context, args ...string) ([]byte, error) {
	if p.exec != nil {
		return p.exec(ctx, args...)
	}
	return execOp(ctx, args...)
}

// execOp runs the real op binary.
func execOp(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "op", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		n := len(args)
		if n > 2 {
			n = 2
		}
		return stderr.Bytes(), fmt.Errorf("op %s: %w — %s",
			strings.Join(args[:n], " "),
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return stdout.Bytes(), nil
}

// parsePath splits a path into (vault, item, field).
//
//	"Vault/Item"           → vault=Vault, item=Item, field=password
//	"Vault/Item/fieldname" → vault=Vault, item=Item, field=fieldname
//	"Item"                 → vault=p.vault (or "Private"), item=Item, field=password
func (p *Provider) parsePath(path string) (vault, item, field string) {
	parts := strings.SplitN(path, "/", 3)
	switch len(parts) {
	case 1:
		return p.vaultOrDefault(""), parts[0], defaultField
	case 2:
		return p.vaultOrDefault(parts[0]), parts[1], defaultField
	default:
		return parts[0], parts[1], parts[2]
	}
}

func (p *Provider) vaultOrDefault(seg string) string {
	if seg != "" {
		return seg
	}
	if p.vault != "" {
		return p.vault
	}
	return "Private"
}

// extractField pulls the field value and updated timestamp from op JSON output.
func extractField(data []byte, field string) (string, time.Time, error) {
	// Try single-field response first (--field returns {"value":"..."}).
	var single struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(data, &single); err == nil && single.Value != "" {
		return single.Value, time.Time{}, nil
	}

	// Full item response — scan fields array.
	var item struct {
		UpdatedAt time.Time `json:"updated_at"`
		Fields    []struct {
			Label string `json:"label"`
			ID    string `json:"id"`
			Value string `json:"value"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &item); err != nil {
		return "", time.Time{}, fmt.Errorf("unmarshal op response: %w", err)
	}

	for _, f := range item.Fields {
		if strings.EqualFold(f.Label, field) || strings.EqualFold(f.ID, field) {
			return f.Value, item.UpdatedAt, nil
		}
	}
	return "", time.Time{}, fmt.Errorf("field %q not found in item", field)
}

// isNotFound checks op stderr/error output for a "not found" error pattern.
func isNotFound(err error, stderr []byte) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(string(stderr) + err.Error())
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "isn't an item") ||
		strings.Contains(msg, "no item")
}
