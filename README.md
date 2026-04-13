# vaultx

> The convenience of an env file. The power of a zero-trust vault.

A `vaultx.env` file is safe to commit. It contains only secret *references*, never values.
At runtime, `vaultx run` resolves each reference from the appropriate provider and injects
the real value into the process — nothing touches disk.

```bash
# vaultx.env  ← commit this
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:work/Vault/stripe-api-key    # from 1Password
JWT_SECRET=vault:aws/myapp/jwt_secret       # from AWS Secrets Manager
PORT=3000                                   # plain values pass through
```

```bash
vaultx run -- npm start          # resolve + inject, nothing on disk
vaultx docker run -- myapp:latest
vaultx docker compose -- up -d
eval $(vaultx shell)             # inject into current shell
```

---

## Install

```bash
go install github.com/gautampachnanda101/vaultx/cmd/vaultx@latest
```

Or build from source:

```bash
git clone https://github.com/gautampachnanda101/mypass
cd mypass
go build -o vaultx ./cmd/vaultx
```

---

## Quick start

```bash
# 1. Create a new local vault
vaultx init

# 2. Store some secrets
vaultx set myapp/db_password "s3cr3t"
vaultx set myapp/api_key "sk-live-abc123"

# 3. Create vaultx.env (commit this)
cat > vaultx.env <<EOF
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:local/myapp/api_key
PORT=3000
EOF

# 4. Run your app — secrets injected, nothing on disk
vaultx run -- npm start
```

---

## How it works

```text
vaultx.env (committed)          vaultx daemon / CLI         Your workload
──────────────────────          ───────────────────         ─────────────
DB_PASSWORD=vault:local/…  ──▶  resolve from vault  ──▶  DB_PASSWORD=s3cr3t
API_KEY=vault:work/…       ──▶  resolve from 1Pass  ──▶  API_KEY=tok3n
PORT=3000                  ──▶  pass through         ──▶  PORT=3000
```

Secret references use the URI format `vault:<provider>/<path>`:

| Prefix | Provider |
| --- | --- |
| `vault:local/…` | Local encrypted file vault |
| `vault:work/…` | 1Password (via `op` CLI) |
| `vault:prod/…` | HashiCorp Vault / AWS / custom |
| `vault:myapp/key` (no prefix) | Default provider |

Plain values (no `vault:` prefix) pass through unchanged — all existing `.env` files
are valid `vaultx.env` files.

---

## Security model

| Property | How |
| --- | --- |
| **Secrets never on disk** | Vault stored as AES-256-GCM ciphertext; runtime values in process memory only |
| **Master password never stored** | Argon2id → barrier key → wraps encryption key; key held in memory, zeroed on lock |
| **`vaultx.env` safe to commit** | Contains only references, never values |
| **Provider tokens never in config** | Read from env vars or OS keychain only |
| **Password rotation is instant** | Re-wraps encryption key only — entries unchanged |
| **Header tampering detected** | HMAC-SHA256(barrier key) on vault header |

### Local vault design (mirrors HashiCorp Vault barrier model)

```text
master password
      │
      ▼ Argon2id (t=3, m=64MiB, p=4)
  barrier key  ──▶  HMAC header (tamper detection)
      │
      ▼ AES-256-GCM
  encryption key (EK)  ──▶  stored wrapped in vault header
      │
      ▼ AES-256-GCM (unique nonce per entry)
  encrypted entries  ──▶  ~/.vaultx/vault.enc
```

Rotating the master password re-wraps the EK only — entries are never re-encrypted.

---

## CLI reference

```text
vaultx init                          Create a new local vault
vaultx unlock                        Unlock the vault for this session
vaultx lock                          Lock the vault (clear key from memory)

vaultx set <path> <value>            Store a secret in the local vault
vaultx get <path>                    Get a single secret value
vaultx delete <path>                 Delete a secret
vaultx list [prefix]                 List secrets (values masked)

vaultx run [--env file] -- <cmd>     Resolve vaultx.env and exec a command
vaultx shell [--env file]            Print export statements (eval $(vaultx shell))

vaultx import [--format f] <file>    Import from external password manager
vaultx export [--format f] [-o file] Export to file

vaultx providers                     List configured providers + health

vaultx serve [--port N]              Start local HTTP daemon (port 7474)

vaultx docker run -- <args>          docker run with secrets as --env flags
vaultx docker compose -- <args>      docker compose with secrets in child env

vaultx k3d setup                     Install ESO + configure SecretStore
vaultx k3d token                     Refresh vaultx-token k8s secret
vaultx k3d status                    Show ESO / SecretStore / ExternalSecret status
```

Global flags: `--config <path>`, `--env <vaultx.env path>`

---

## Import / Export

Import credentials from any major password manager — format is auto-detected:

```bash
vaultx import ~/Downloads/google-passwords.csv
vaultx import ~/Downloads/1password-export.csv --format 1password
vaultx import ~/Downloads/bitwarden-export.json
```

| Format | Source |
| --- | --- |
| `google` | Google Password Manager CSV |
| `1password` | 1Password CSV |
| `bitwarden` | Bitwarden / Vaultwarden JSON |
| `lastpass` | LastPass CSV |
| `samsung` | Samsung Pass CSV |
| `mcafee` | McAfee True Key CSV |
| `dashlane` | Dashlane CSV |
| `keeper` | Keeper CSV |
| `csv` | Generic CSV (name/username/password/url) |
| `vaultx` | vaultx JSON backup (lossless round-trip) |

Export:

```bash
vaultx export -f bitwarden -o bw-backup.json
vaultx export -f vaultx   -o backup.json     # full backup
vaultx export -f csv      -o secrets.csv
```

---

## Multi-provider configuration

`~/.vaultx/config.toml`

```toml
[vault]
path = "~/.vaultx/vault.enc"
kdf  = "argon2id"

# Local vault (always registered as default)
[[providers]]
id      = "local"
type    = "local"
default = true

# 1Password — requires op CLI installed and signed in
[[providers]]
id      = "work"
type    = "onepassword"
account = "my.1password.com"
vault   = "Work"

# HashiCorp Vault (coming soon)
[[providers]]
id        = "prod"
type      = "hashicorp"
address   = "https://vault.example.com"
token_env = "VAULT_TOKEN"

# AWS Secrets Manager (coming soon)
[[providers]]
id       = "aws"
type     = "aws"
region   = "eu-west-2"
role_arn = "arn:aws:iam::123456789:role/vaultx"
```

With this config:

```bash
# vaultx.env
DB_PASS=vault:local/myapp/db       # from local vault
STRIPE=vault:work/Payments/api-key  # from 1Password "Work" vault
REDIS=vault:prod/myapp/redis-url    # from HashiCorp Vault
```

---

## Docker

```bash
# Instead of:
docker run --env-file .env myapp       # secrets on disk, visible in docker inspect

# Use:
vaultx docker run -- myapp:latest      # --env KEY=VAL args only, nothing on disk
vaultx docker compose -- up -d        # compose inherits resolved env
```

---

## Kubernetes / k3d

vaultx integrates with the [External Secrets Operator](https://external-secrets.io)
via its webhook provider. The ESO calls the vaultx daemon to resolve secrets at
deploy time — no secrets in manifests or container env files.

```bash
# 1. Start the daemon (keeps vault unlocked, serves webhook)
vaultx serve --port 7474

# 2. One-time cluster setup (installs ESO + configures SecretStore)
vaultx k3d setup

# 3. Declare what secrets your app needs
kubectl apply -f k3d/externalsecret-example.yaml
```

```yaml
# k3d/externalsecret-example.yaml
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
spec:
  secretStoreRef:
    name: vaultx
    kind: SecretStore
  data:
    - secretKey: DB_PASSWORD
      remoteRef:
        key: myapp/db_password    # resolved via vaultx multi-vault
    - secretKey: API_KEY
      remoteRef:
        key: myapp/api_key
```

ESO creates a standard `k8s Secret` that pods mount normally. Secret values never
appear in manifests or container definitions.

---

## Local HTTP daemon

`vaultx serve` exposes a local-only HTTP API (127.0.0.1 only) for the VS Code
extension, browser extension, and k3d webhook.

```text
GET  /health                        liveness (no auth)
GET  /v1/secret?path=<path>         resolve a single secret
POST /v1/resolve                    resolve a vaultx.env body
GET  /v1/list?prefix=<prefix>       list secrets (values masked)
GET  /externalsecrets/<key>         ESO webhook endpoint
```

All endpoints except `/health` require `X-Vaultx-Token` header (or `?token=` param).
The token is written to `~/.vaultx/daemon.token` (mode 0600) at startup.

```bash
TOKEN=$(cat ~/.vaultx/daemon.token)
curl -H "X-Vaultx-Token: $TOKEN" http://localhost:7474/v1/secret?path=myapp/db
```

---

## Project layout

```text
cmd/vaultx/              CLI entrypoint + cobra commands
internal/
  config/               TOML config loader
  envfile/              vaultx.env parser
  providers/
    local/              Encrypted file vault (Argon2id + AES-256-GCM)
    onepassword/        1Password via op CLI
  resolver/             Multi-provider fan-out + vaultx.env resolution
  injector/             Docker injection (docker run + compose)
  daemon/               Local HTTP server (extensions + k3d webhook)
  importexport/         Import/export for 9 external formats
k3d/                    Kubernetes / ESO manifests
```

---

## Development

```bash
go test ./...           # 56 tests, ~20ms
go build ./cmd/vaultx
```

See [SPEC.md](SPEC.md) for the full product specification and [AGENTS.md](AGENTS.md)
for coding agent context.
