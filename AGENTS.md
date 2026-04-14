# vaultx — Unified Agent Context (AGENTS)

This file is the canonical shared context for all coding assistants working in this repository.

## Priority rule

1. Follow this file first for repository-wide behavior.
2. Then follow assistant-specific guidance (`.github/copilot-instructions.md`, `CLAUDE.md`).
3. If guidance conflicts, prefer the stricter rule.

---

## What vaultx is

**vaultx** is a zero-trust secrets broker. `vaultx.env` is safe to commit — it contains only `vault:` references, never values. At runtime, `vaultx run` resolves each reference and injects the real secret into the child process. Nothing ever touches disk.

- Go binary (`vaultx`) with CLI, local HTTP daemon, Docker injection, and k3d / ESO support.
- `vaultx.env` format: drop-in superset of `.env`; plain values pass through, `vault:` prefixed values are resolved at runtime.
- Local vault: AES-256-GCM encrypted at rest, Argon2id KDF, key held in memory and zeroed on lock.
- Multi-provider: local vault, 1Password (`op` CLI), HashiCorp Vault, AWS Secrets Manager.

Full spec: `SPEC.md`. Read it before making changes.

---

## Repository layout

```text
cmd/vaultx/              CLI entrypoint (cobra) + command files
internal/config/         TOML config loader
internal/envfile/        vaultx.env parser
internal/providers/      Provider interface + local / 1password adapters
internal/resolver/       Multi-provider fan-out + vaultx.env resolution
internal/injector/       Docker injection (docker run + compose)
internal/daemon/         Local HTTP server (extensions + k3d webhook)
internal/importexport/   Import/export for 9 external formats
k3d/                     Kubernetes / ESO manifests
scripts/                 CI quality gate
.github/skills/          AI assistant skill definitions
.claude/commands/        Claude Code slash commands
```

---

## Coding rules

- Secret values must never appear in logs, error messages, or test fixtures — only paths and masked representations.
- Secret values resolved in-memory only; never written to disk.
- Provider tokens/credentials from env vars or OS keychain only — never config files.
- `vaultx.env` is a superset of `.env`: plain values pass through, `vault:` prefixed values are resolved.
- Secret URI format: `vault:<provider-id>/<path>`
- Argon2id for KDF (not PBKDF2); AES-256-GCM for encryption — do not change crypto primitives.
- Config at `~/.vaultx/config.toml`; vault at `~/.vaultx/vault.enc`

---

## Feature sync surfaces

When adding or changing a command or feature, update **all** relevant surfaces:

- CLI command behavior, `Long` help text, `Example`, and flags.
- HTTP daemon endpoints in `internal/daemon/` if applicable.
- `SPEC.md` for architectural changes.
- `docs/user-guide.md` and `VAULTX_USER_GUIDE.md` for user-visible behavior.
- `.github/skills/vaultx-tools/SKILL.md` for new CLI/API patterns.
- `.github/copilot-instructions.md` if dev workflow changes.

---

## Pre-commit and pre-push gate

Before any commit or push:

1. `go vet ./...`
2. `go test ./...`
3. `go build ./...`
4. Confirm `SPEC.md` reflects architecture changes (or note "no architecture impact").
5. Confirm help text and docs match current behavior.

---

## Release SDLC (required order)

1. **Commit** — include all code/docs/skill updates.
2. **Push** — push branch changes and confirm remote sync.
3. **Validate (pre-release)** — `./scripts/quality-gate.sh` locally and green CI.
4. **Release** — push a `v*` tag: `git tag v0.1.0-rc1 && git push origin v0.1.0-rc1`
5. **Validate (post-release)** — confirm `release` and `post-release-binary-regression` workflows succeeded.

---

## vaultx + promptx cross-tool integration

vaultx and promptx are designed to complement each other:

| Concern | Tool |
| --- | --- |
| Secrets storage and injection | **vaultx** |
| Memory, context, cross-tool handoff | **promptx** |

### Use vaultx to inject secrets into promptx

```bash
# Store promptx passkey in vaultx
vaultx set promptx/passkey "your-passkey"

# Run promptx with passkey injected — no plaintext in shell history
vaultx run -- promptx memory-query "architecture decisions" --repo . --limit 5
vaultx run -- promptx serve
vaultx run -- promptx memory-watch --repo . --interval 30
```

`vaultx.env` for a promptx project:

```env
PROMPTX_PASSKEY=vault:local/promptx/passkey
```

### Use promptx to remember vaultx decisions

```bash
# Recall past provider and secret path decisions
promptx memory-query "vaultx provider config" --repo . --limit 5
promptx ask "which vault paths does this project use?" --repo . --limit 6

# Log significant vaultx changes to memory
promptx memory-write "Added vault:prod/ provider for AWS Secrets Manager" \
  --repo . --tags vaultx,infra,decision --force-store
```

### Skills available

| Skill file | Purpose |
| --- | --- |
| `.github/skills/vaultx-tools/SKILL.md` | vaultx CLI and HTTP daemon reference |
| `.github/skills/vaultx-promptx/SKILL.md` | Cross-tool integration patterns |

### Claude Code slash commands

| Command | Purpose |
| --- | --- |
| `/vaultx-env` | Generate a `vaultx.env` for this project |
| `/vaultx-audit` | Audit the project for committed or exposed secrets |
| `/vaultx-provider` | Configure a new secret provider |
| `/vaultx-docker` | Migrate Docker setup to vaultx injection |
| `/vaultx-k3d` | Set up or troubleshoot k3d / ESO integration |
| `/vaultx-import` | Import credentials from a password manager |

### MCP and IDE config

- **Claude Code**: `.claude/settings.json` — promptx MCP server + PostToolUse memory hook
- **Cursor**: `.cursor/mcp.json` — promptx MCP server

Both configs wire promptx MCP into the IDE so `chat_helper`, `memory_query`, `ask`, and `resume` are available as native tool calls during vaultx development sessions.

---

## Commit discipline

- Small, focused commits — one logical change per commit.
- Commit message format: `type(scope): description`
- No commit should break `go build ./...` or `go test ./...`.
