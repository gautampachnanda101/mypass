# vaultx — Specification

**The convenience of an env file. The power of a zero-trust vault.**

A `vaultx.env` file is safe to commit. It contains only secret *references*, never values.
At runtime, `vaultx run` resolves each reference from the appropriate provider and injects
the real value into the process environment — nothing touches disk.

---

## Core Concept

```bash
# vaultx.env  ← commit this, it contains no secrets
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:1password/Work/Stripe/credential
JWT_SECRET=vault:aws/myapp/jwt_secret
REDIS_URL=vault:hashicorp/prod/redis-url
PORT=3000                               # plain values pass through unchanged
```

```bash
# developer workflow
vaultx run -- npm start                 # resolve + inject, nothing on disk
vaultx run -- docker-compose up         # same for compose
vaultx shell                            # inject into current shell session
vaultx get myapp/db_password            # print a single resolved secret
vaultx copy myapp/api_key               # copy to clipboard, clear after 30s
```

Secret references use the URI format: `vault:<provider>/<path>/<key>`

- `vault:local/...`       — local encrypted file vault (`~/.vaultx/vault.enc`)
- `vault:1password/...`   — 1Password via `op` CLI
- `vault:hashicorp/...`   — HashiCorp Vault HTTP API
- `vault:aws/...`         — AWS Secrets Manager
- `vault:env/...`         — pass-through from current environment (escape hatch)

Plain values (no `vault:` prefix) pass through as-is. This means `vaultx.env` is a
superset of `.env` — all existing `.env` files are valid `vaultx.env` files.

---

## Architecture

```
vaultx/
├── cmd/vaultx/              # CLI entrypoint (cobra)
├── internal/
│   ├── providers/           # Provider interface + adapters
│   │   ├── provider.go      # interface: Get, List, Health
│   │   ├── local/           # Encrypted file vault (Argon2id + AES-256-GCM)
│   │   ├── onepassword/     # 1Password via op CLI
│   │   ├── hashicorp/       # HashiCorp Vault HTTP API
│   │   ├── aws/             # AWS Secrets Manager SDK
│   │   └── env/             # Environment pass-through
│   ├── resolver/            # Parse vaultx.env + fan-out to providers
│   ├── injector/
│   │   ├── process.go       # vaultx run — exec with injected env
│   │   ├── docker.go        # Docker API injection (no env files)
│   │   └── k8s.go           # External Secrets webhook backend
│   ├── daemon/              # Local HTTP server (VS Code / browser ext)
│   └── store/               # Local vault CRUD (wraps local provider)
├── web/                     # Next.js UI (go:embed)
├── vscode-extension/        # VS Code extension (TypeScript)
└── browser-extension/       # Chrome/Firefox Manifest V3
```

---

## Provider Interface

```go
type Secret struct {
    Key      string
    Value    string
    Version  string
    UpdatedAt time.Time
}

type Provider interface {
    ID()     string                                         // e.g. "local", "1password"
    Get(ctx context.Context, path string) (Secret, error)
    List(ctx context.Context, prefix string) ([]Secret, error)
    Health(ctx context.Context) error
}
```

---

## vaultx.env Format

A strict superset of `.env`:

```
# Comments supported
KEY=plain_value                         # literal, no resolution
KEY=vault:<provider>/<path>             # secret reference
KEY=${OTHER_KEY}                        # env var interpolation (existing envs)
KEY=vault:local/myapp/db               # local vault
KEY=vault:1password/Vault Name/item    # 1Password (spaces OK)
```

Resolution order per key:
1. If `vault:` prefix — call named provider
2. If `${VAR}` — interpolate from process env
3. Otherwise — literal value

The file is looked up in order: `./vaultx.env`, `./.vaultx.env`, `~/.vaultx/default.env`

---

## CLI Commands

```
vaultx run [--env file] -- <cmd> [args...]   Resolve vaultx.env and exec cmd
vaultx shell [--env file]                    Print export statements (eval $(vaultx shell))
vaultx get <path>                            Get a single secret value
vaultx set <path> <value>                    Store in local vault
vaultx delete <path>                         Delete from local vault
vaultx list [prefix]                         List secrets (values masked)
vaultx copy <path>                           Copy to clipboard, clear after 30s
vaultx gen [--length N] [--symbols]          Generate and store a password
vaultx import <file.csv>                     Import from CSV (1password/bitwarden/etc)
vaultx providers                             List configured providers + health
vaultx serve [--port N]                      Start local HTTP daemon
vaultx unlock                                Unlock vault (cache master key for session)
vaultx lock                                  Lock vault (clear cached key)
vaultx docker run [--env file] -- <args>     Docker run with secrets injected via API
```

---

## Config File

`~/.vaultx/config.toml`

```toml
[vault]
path = "~/.vaultx/vault.enc"
kdf  = "argon2id"        # argon2id | pbkdf2

[[providers]]
id   = "local"
type = "local"
default = true           # used when no provider prefix given

[[providers]]
id   = "work"
type = "onepassword"
account = "my.1password.com"
vault = "Work"

[[providers]]
id   = "prod"
type = "hashicorp"
address = "https://vault.example.com"
token_env = "VAULT_TOKEN"            # token sourced from env, never config

[[providers]]
id   = "aws"
type = "aws"
region = "eu-west-2"
role_arn = "arn:aws:iam::123456789:role/vaultx"   # assume-role, no static creds
```

---

## k3d / External Secrets Integration

vaultx daemon exposes an External Secrets Operator-compatible webhook endpoint.

```yaml
# secretstore.yaml
apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vaultx
spec:
  provider:
    webhook:
      url: "http://host.k3d.internal:7474/externalsecrets/{{ .remoteRef.key }}"
      result:
        jsonPath: "$.value"
```

```yaml
# externalsecret.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
spec:
  secretStoreRef:
    name: vaultx
    kind: SecretStore
  data:
    - secretKey: DB_PASSWORD
      remoteRef:
        key: myapp/db_password          # resolved via vaultx multi-vault
```

No secrets in manifests. No env files in containers. Vault access controls who can pull.

---

## HTTP Daemon & Web UI

`vaultx serve` starts a local HTTP server on `127.0.0.1` (default port `7474`).

### Endpoints

| Endpoint | Method | Auth | Purpose |
|---|---|---|---|
| `GET /health` | GET | None | Liveness check |
| `POST /auth/touchid` | POST | None | macOS Touch ID authentication |
| `GET /v1/secret?path=<path>` | GET | Token | Resolve single secret |
| `POST /v1/resolve` | POST | Token | Resolve vaultx.env body |
| `GET /v1/list?prefix=<prefix>` | GET | Token | List secrets (values masked) |
| `GET /v1/audit?limit=<n>` | GET | Token | Retrieve audit log (default 100, max 1000) |
| `GET /externalsecrets/<key>` | GET | Token | ESO webhook endpoint |
| `GET /` | GET | None | Web UI (Touch ID auth required) |

### Web UI

The daemon includes an embedded web UI accessible at `http://127.0.0.1:7474/`.

**Authentication Flow:**
1. Browser loads the web UI (no auth required for initial page load)
2. Touch ID authentication modal appears automatically
3. User authenticates via macOS Touch ID sensor
4. Browser receives session token, stored in `sessionStorage`
5. All subsequent API calls use the token via `X-Vaultx-Token` header

**Features:**
- **Secrets Tab**: Browse, search, and view all secrets across providers
- **Resolve Tab**: Paste `vaultx.env` content and resolve all references in-browser
- **Audit Log Tab**: View security events (auth failures, rate limits, path validation)

**Security:**
- Touch ID required for browser access (macOS only; CLI uses master password)
- Session token stored in browser memory only (`sessionStorage`)
- Rate limiting: 10 requests/second, burst 50
- Request timeouts: 10 seconds maximum
- Path validation: blocks traversal attacks (`.., /, \, null bytes`)
- Sanitized errors: never leak secret values in HTTP responses
- Audit logging: all security-relevant events logged to stderr

### CLI Access

```bash
# Read token from daemon
TOKEN=$(cat ~/.vaultx/daemon.token)

# Call API directly
curl -H "X-Vaultx-Token: $TOKEN" \
  http://localhost:7474/v1/secret?path=myapp/db
```

Session token written to `~/.vaultx/daemon.token` (mode `0600`) at startup.

---

## Docker Injection

```bash
# instead of:
docker run --env-file .env myapp

# use:
vaultx docker run -- myapp              # reads vaultx.env, injects via Docker API
```

vaultx calls `docker run` with `--env KEY=VALUE` for each resolved secret.
The values are passed as args to the Docker CLI subprocess — they never touch the
filesystem and are not visible in `docker inspect` (unlike `--env-file`).

For docker-compose:
```bash
vaultx run -- docker-compose up         # compose inherits the injected env
```

---

## Security Properties

| Property | How |
|---|---|
| Secrets never on disk (unencrypted) | Vault stored as AES-256-GCM ciphertext; runtime values in process memory only |
| Master password never stored | Argon2id → derive key; key held in memory, cleared on lock |
| Memory protection | Sensitive keys locked in RAM via mlock() (Unix/macOS) / VirtualLock (Windows) |
| `vaultx.env` safe to commit | Contains only references (`vault:provider/path`), never values |
| Provider credentials not in config | Tokens sourced from env vars or OS keychain, never written to config file |
| Defense in depth | Rate limiting (10 req/s), lockout after 5 failures, auto-lock after 15min idle |
| Multi-factor authentication | TOTP (RFC 6238) with QR code setup and recovery codes |
| Audit trail | Daemon logs every access/unlock/failure; export JSON/CSV; forward to syslog |
| Encrypted backups | Auto-backup on mutations; Shamir secret sharing for distributed recovery |
| Rotation | Update in vault once; all `vaultx run` invocations pick it up immediately |

---

## Defense-in-Depth Security Policies

vaultx implements layered security controls beyond encryption:

### Unlock Rate Limiting

- **Max 10 unlock attempts per minute** across CLI and daemon
- **Lockout after 5 failed attempts** — vault locked for 30 minutes
- State persisted to `~/.vaultx/policy.json` across process restarts
- Protects against brute-force password guessing

### Auto-Lock

- **Vault auto-locks after 15 minutes of inactivity**
- Activity = any vault operation (get, set, delete, list, run)
- Timer reset on each operation
- Manual lock via `vaultx lock` stops timer

### Memory Protection

- **Encryption keys locked in RAM** via `mlock()` (Unix/macOS) or `VirtualLock` (Windows)
- Prevents secrets from being swapped to disk
- Finalizers ensure cleanup on garbage collection
- Daemon session tokens stored in OS keychain (macOS Keychain, future: Linux Secret Service, Windows Credential Manager)

---

## Multi-Factor Authentication (TOTP)

Enable TOTP-based 2FA for vault unlocking:

```bash
# Enable MFA (generates TOTP secret + QR code + recovery codes)
vaultx mfa enable

# Unlock flow: password + 6-digit authenticator code
vaultx unlock
# Master password: **********
# Authenticator code: 123456

# Disable MFA
vaultx mfa disable

# View remaining recovery codes
vaultx mfa recovery-codes
```

### TOTP Setup Flow

1. `vaultx mfa enable` generates:
   - TOTP secret (32-byte base32-encoded)
   - QR code (ASCII art in terminal, or data URL in web UI)
   - 10 recovery codes (format: `XXXX-XXXX`, single-use)

2. Scan QR code with authenticator app (Google Authenticator, Authy, 1Password, etc.)

3. Subsequent unlocks require:
   - Master password
   - 6-digit TOTP code from authenticator app
   - Or: one recovery code (consumed on use)

### Recovery Codes

- **10 codes generated** during MFA setup
- **Format**: `ABCD-EFGH` (alphanumeric, no ambiguous chars: 0, O, 1, I)
- **Single-use**: Each code is invalidated after use
- **View remaining codes**: `vaultx mfa recovery-codes` (requires unlock)
- **Regenerate**: Disable and re-enable MFA to generate fresh recovery codes

---

## Audit Logging

All security-relevant events are logged:

### Events Logged

- Vault unlocks (success/failure)
- Secret access (GET /v1/secret, vaultx get, etc.)
- Secret mutations (set, delete)
- MFA validation (success/failure)
- Rate limit violations
- Lockout triggers

### Audit Export

```bash
# Export to JSON
vaultx audit --format json --output audit.json

# Export to CSV
vaultx audit --format csv --output audit.csv

# View in terminal (table format)
vaultx audit --limit 50
```

### Syslog Integration

Forward audit logs to syslog servers (local or remote):

```bash
# Local syslog
vaultx serve --syslog-network local

# Remote syslog (TCP)
vaultx serve --syslog-network tcp --syslog-address syslog.example.com:514

# Remote syslog (UDP)
vaultx serve --syslog-network udp --syslog-address 192.168.1.100:514
```

- **Facility**: `LOG_AUTH`
- **Tag**: `vaultx`
- **Levels**: Info (successful operations), Error (failures)

---

## Encrypted Backups with Shamir Secret Sharing

### Auto-Backup

vaultx automatically creates encrypted backups after every vault mutation:

```bash
# Backups created on:
vaultx set myapp/key value    # After set
vaultx delete myapp/key       # After delete

# Backups stored in ~/.vaultx/backups/
# Format: vault-2025-05-15T10-30-00.enc
```

### Manual Backup

```bash
# Create backup manually
vaultx backup create

# List all backups
vaultx backup list
```

### Restore from Backup

Restore with master password:

```bash
vaultx backup restore vault-2025-05-15T10-30-00.enc
```

### Shamir Secret Sharing

Split the backup encryption key into N shares, requiring M to restore:

```bash
# Split backup key into 5 shares, requiring 3 to restore
vaultx backup split --shares 5 --threshold 3

# Outputs:
#   ~/.vaultx/backup-shares/share-1.json
#   ~/.vaultx/backup-shares/share-2.json
#   ...
#   ~/.vaultx/backup-shares/share-5.json

# Distribute shares to different locations/people

# Restore using shares (any 3 of the 5)
vaultx backup restore --shares share-1.json,share-3.json,share-4.json vault-2025-05-15T10-30-00.enc
```

### Use Cases

- **Disaster recovery**: Store Shamir shares with different team members
- **Escrow**: Split backup key and deposit shares with legal/compliance
- **Geographic distribution**: Store shares in different physical locations (fire, theft protection)
- **M-of-N governance**: Require M approvals from N stakeholders to recover vault

### Encryption

- Backups encrypted with **AES-256-GCM**
- Key derived from master password via **Argon2id** (64 MiB, 3 iterations, 4 threads)
- Separate KDF salt for backup keys (distinct from vault encryption)
- Shamir implementation from `github.com/hashicorp/vault/shamir` (battle-tested)

---

## Non-Goals (v1)

- Secret sharing / team sync (use 1Password or Vault for that)
- Secret versioning / history (provider-dependent)
- Windows support (v2)
