# vaultx — Agent Context

This is **vaultx**: a secrets broker that gives developers the convenience of an env file
with the security of a zero-trust vault.

## What it is

- A Go binary (`vaultx`) that manages secrets from multiple providers
- A `vaultx.env` file format: `.env`-compatible, values may be `vault:provider/path` references
- `vaultx run -- <cmd>` resolves references and injects real values into the child process
- A local HTTP daemon for VS Code extension and browser extension
- Docker injection and k3d / External Secrets Operator support

## Key principle

**`vaultx.env` is safe to commit.** It contains only keys and `vault:` references, never values.
Secrets are resolved at runtime from the appropriate provider.

## Repository layout

```
cmd/vaultx/              CLI entrypoint (cobra)
internal/providers/      Provider interface + local/1password/hashicorp/aws/env adapters
internal/resolver/       vaultx.env parser + multi-provider fan-out
internal/injector/       process.go, docker.go, k8s.go
internal/daemon/         Local HTTP server
internal/store/          Local vault CRUD (wraps local provider)
web/                     Next.js frontend (go:embed)
vscode-extension/        VS Code extension (TypeScript)
browser-extension/       Chrome/Firefox Manifest V3
```

## Spec

Full product spec is in `SPEC.md`. Read it before making changes.

## Coding rules

- All secrets resolved in-memory; never written to disk or logged
- Provider tokens/credentials must come from env vars or OS keychain — never config files
- `vaultx.env` is a superset of `.env`: plain values pass through, `vault:` prefixed values are resolved
- Secret URI format: `vault:<provider-id>/<path>`
- Provider interface is in `internal/providers/provider.go` — all adapters implement it
- Argon2id for KDF (not PBKDF2); AES-256-GCM for encryption
- Config at `~/.vaultx/config.toml`; vault at `~/.vaultx/vault.enc`

## Commit discipline

- Small, focused commits — one logical change per commit
- Commit message format: `type(scope): description`
- No commit should break `go build ./...` or `go test ./...`
- Architecture changes must update `SPEC.md` in the same commit
