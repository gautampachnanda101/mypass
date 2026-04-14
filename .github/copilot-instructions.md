# Copilot Instructions for vaultx

Use this repository's vaultx workflows when helping with development tasks.

## Canonical shared context
- Treat `AGENTS.md` as the repository-wide canonical context for all coding assistants.
- Apply `AGENTS.md` rules first, then the guidance in this file.

## Default development workflow

1. Read `AGENTS.md` and `SPEC.md` before proposing changes.
2. Run `go test ./...` and `go build ./...` to validate before suggesting completion.
3. Architecture changes must update `SPEC.md` in the same commit.
4. Secret values must never appear in logs, error messages, or test fixtures.

## Secret safety rules (non-negotiable)

- Never log or print secret values — only paths and masked representations.
- Never write secret values to files, even temp files.
- Resolved secret values live in process memory only (`syscall.Exec` pattern in `cmd_run.go`).
- Provider tokens/credentials must come from env vars or OS keychain, never config files.
- AES-256-GCM with Argon2id KDF — do not downgrade or change the crypto primitives.

## Using vaultx during development

```bash
# Store dev secrets
vaultx set myapp/db_password "dev-password"
vaultx set myapp/api_key "sk-dev-abc123"

# Run tests with injected secrets
vaultx run -- go test ./...

# Run the app with all secrets resolved
vaultx run -- go run ./cmd/vaultx serve
```

## Using promptx alongside vaultx

- Use `promptx memory-query` to recall past architecture decisions before proposing changes.
- Log significant vaultx changes to promptx memory for cross-session context.

```bash
promptx memory-query "vaultx provider interface" --repo . --limit 5
promptx memory-write "Changed: added AWS provider skeleton" --repo . --tags vaultx,aws
```

## Feature sync rule

For every new command or feature, update all relevant surfaces before completion:

- CLI command behavior, help text (`Long`, `Example`), and flags.
- HTTP daemon endpoints in `internal/daemon/` if applicable.
- `SPEC.md` for architectural changes.
- `docs/user-guide.md` and `VAULTX_USER_GUIDE.md` for user-visible behavior.
- `.github/skills/vaultx-tools/SKILL.md` for new CLI patterns.

## Pre-commit gate

Before any commit or push:

1. `go vet ./...`
2. `go test ./...`
3. `go build ./...`
4. Confirm `SPEC.md` reflects any architecture changes (or note "no architecture impact").

## Release SDLC order

1. Commit.
2. Push.
3. Validate: `./scripts/quality-gate.sh` + green CI.
4. Release: push a `v*` tag to trigger the release workflow.
5. Validate post-release: confirm `release` and `post-release-binary-regression` workflows succeeded.
