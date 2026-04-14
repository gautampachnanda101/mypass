# vaultx User Guide

Full reference for vaultx — the zero-trust secrets broker.

For a quick install-and-run guide see [../VAULTX_USER_GUIDE.md](../VAULTX_USER_GUIDE.md).

---

## Installation

### macOS and Linux (Homebrew)

```bash
brew tap gautampachnanda101/homebrew-tap
brew install vaultx
```

### Windows (Scoop)

```powershell
scoop bucket add vaultx https://github.com/gautampachnanda101/scoop-bucket
scoop install vaultx
```

### Build from source

```bash
git clone https://github.com/gautampachnanda101/vaultx
cd vaultx
go build -o vaultx ./cmd/vaultx
```

---

## Contextual help

Every command has a `--help` flag:

```bash
vaultx --help
vaultx run --help
vaultx k3d --help
vaultx docker --help
```

---

## First-time setup

### 1. Create the vault

```bash
vaultx init
```

Creates `~/.vaultx/vault.enc` and `~/.vaultx/config.toml`.

### 2. Store your first secrets

```bash
vaultx set myapp/db_password "s3cr3t"
vaultx set myapp/api_key "sk-live-abc123"
vaultx set myapp/jwt_secret "$(openssl rand -hex 32)"
```

### 3. Create a vaultx.env (commit this)

```bash
cat > vaultx.env <<EOF
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:local/myapp/api_key
JWT_SECRET=vault:local/myapp/jwt_secret
PORT=3000
EOF
```

### 4. Run your app

```bash
vaultx run -- npm start
```

---

## Vault lifecycle

```bash
vaultx init      # Create a new vault (one time per machine)
vaultx unlock    # Unlock for this session — prompts for master password
vaultx lock      # Clear encryption key from memory
```

The vault auto-locks when the process exits. The daemon keeps the key in memory
for its lifetime.

---

## Secret CRUD

```bash
vaultx set   myapp/db_password "s3cr3t"   # create or overwrite
vaultx get   myapp/db_password            # print value to stdout
vaultx list                               # list all paths (values masked)
vaultx list  myapp/                       # list paths under prefix
vaultx delete myapp/db_password           # delete
```

Paths are hierarchical with `/` as separator. Any string is valid:
`myapp/db_password`, `infra/prod/redis-url`, `team/shared/slack-token`.

---

## Running commands and injecting into shell

Resolve `vaultx.env` and exec a command with secrets in its environment:

```bash
vaultx run -- npm start
vaultx run -- ./server
vaultx run --env staging.env -- python manage.py runserver
```

Inject secrets into your current shell:

```bash
eval $(vaultx shell)
eval $(vaultx shell --env staging.env)
```

---

## Docker

```bash
# resolve vaultx.env and pass each secret as a --env KEY=VAL flag
vaultx docker run -- myapp:latest

# compose inherits resolved secrets from the child environment
vaultx docker compose -- up -d
vaultx docker compose -- up --build
```

Nothing is written to disk — no `--env-file`, no plaintext in `docker inspect`.

---

## Import / Export

### Import

Format is auto-detected from file extension and header:

```bash
vaultx import ~/Downloads/google-passwords.csv
vaultx import ~/Downloads/bitwarden-export.json
vaultx import ~/Downloads/1password-export.csv --format 1password
```

| `--format` | Source |
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
| `vaultx` | vaultx JSON (lossless round-trip) |

### Export

```bash
vaultx export -f vaultx   -o backup.json      # full backup — lossless round-trip
vaultx export -f bitwarden -o bw-backup.json
vaultx export -f csv      -o secrets.csv
```

Keep your backup encrypted and offline.

---

## Multi-provider configuration

`~/.vaultx/config.toml`:

```toml
[vault]
path = "~/.vaultx/vault.enc"
kdf  = "argon2id"

[[providers]]
id      = "local"
type    = "local"
default = true

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

Check all provider statuses:

```bash
vaultx providers
```

---

## vaultx.env reference

`vaultx.env` is a drop-in superset of `.env`. Every valid `.env` is a valid
`vaultx.env`.

```env
# Secret reference — resolved at runtime from the named provider
DB_PASSWORD=vault:local/myapp/db_password
STRIPE_KEY=vault:work/Payments/api-key

# Plain value — passed through unchanged
PORT=3000
NODE_ENV=production
```

Secret URI format: `vault:<provider-id>/<path>`

| URI | Provider |
| --- | --- |
| `vault:local/…` | Local encrypted vault (always available) |
| `vault:work/…` | 1Password (requires `op` CLI signed in) |
| `vault:prod/…` | HashiCorp Vault / AWS |
| `vault:myapp/key` (no prefix) | Default provider |

---

## Local HTTP daemon

```bash
vaultx serve             # default port 7474
vaultx serve --port 8080
```

The daemon writes a bearer token to `~/.vaultx/daemon.token` (mode 0600) at
startup. All endpoints except `/health` require it.

```bash
TOKEN=$(cat ~/.vaultx/daemon.token)

# Resolve a single secret
curl -H "X-Vaultx-Token: $TOKEN" \
  "http://localhost:7474/v1/secret?path=myapp/db_password"

# Resolve a full vaultx.env body
curl -H "X-Vaultx-Token: $TOKEN" \
     -H "Content-Type: text/plain" \
     --data-binary @vaultx.env \
  "http://localhost:7474/v1/resolve"

# List secrets under a prefix
curl -H "X-Vaultx-Token: $TOKEN" \
  "http://localhost:7474/v1/list?prefix=myapp/"
```

| Endpoint | Auth | Description |
| --- | --- | --- |
| `GET /health` | none | Liveness check |
| `GET /v1/secret?path=<p>` | token | Resolve one secret |
| `POST /v1/resolve` | token | Resolve vaultx.env body |
| `GET /v1/list?prefix=<p>` | token | List paths (values masked) |
| `GET /externalsecrets/<key>` | token | ESO webhook |

---

## Kubernetes / k3d

vaultx integrates with the External Secrets Operator via its webhook provider.

```bash
# 1. Start the daemon (keeps vault unlocked, serves the ESO webhook)
vaultx serve --port 7474

# 2. One-time cluster setup (helm install ESO + kubectl apply SecretStore)
vaultx k3d setup
vaultx k3d setup --cluster-wide           # ClusterSecretStore (all namespaces)
vaultx k3d setup --namespace my-ns        # specific namespace

# 3. After a daemon restart, refresh the token secret
vaultx k3d token

# 4. Check cluster health
vaultx k3d status
```

Apply secrets to your cluster:

```bash
kubectl apply -f k3d/externalsecret-example.yaml
```

---

## Security model

| Property | How |
| --- | --- |
| **Secrets never on disk** | AES-256-GCM ciphertext at rest; plaintext only in process memory |
| **Master password never stored** | Argon2id KDF → barrier key → wraps EK; zeroed on lock |
| **`vaultx.env` safe to commit** | References only, never values |
| **Provider tokens not in config** | Read from env vars or OS keychain |
| **Password rotation is instant** | Re-wraps EK only — entries are never re-encrypted |
| **Header tampering detected** | HMAC-SHA256(barrier key) on vault header |

### Local vault internals

```
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

---

## Full CLI reference

```
vaultx init                          Create a new local vault
vaultx unlock                        Unlock the vault for this session
vaultx lock                          Lock the vault (clear key from memory)

vaultx set <path> <value>            Store a secret in the local vault
vaultx get <path>                    Get a single secret value
vaultx delete <path>                 Delete a secret
vaultx list [prefix]                 List secrets (values masked)

vaultx run [--env file] -- <cmd>     Resolve vaultx.env and exec a command
vaultx shell [--env file]            Print export statements (eval $(vaultx shell))

vaultx import [--format f] <file>    Import from a password manager export
vaultx export [--format f] [-o file] Export to file

vaultx providers                     List configured providers + health

vaultx serve [--port N]              Start local HTTP daemon (default port 7474)

vaultx docker run -- <args>          docker run with secrets as --env flags
vaultx docker compose -- <args>      docker compose with secrets in child env

vaultx k3d setup [--namespace N] [--cluster-wide] [--port N]
                                     Install ESO + configure SecretStore
vaultx k3d token [--namespace N]     Refresh vaultx-token k8s secret
vaultx k3d status                    Show ESO / SecretStore / ExternalSecret status

vaultx version                       Print version
```

Global flags: `--config <path>`, `--env <vaultx.env path>`

---

## Troubleshooting

**Vault sealed after reboot** — Run `vaultx unlock`. The key is never persisted
across restarts by design.

**`vault:work/…` references not resolving** — Ensure `op` is installed and
signed in (`op signin`). Run `vaultx providers` to check health.

**Daemon token missing** — Run `vaultx serve`. A fresh token is written to
`~/.vaultx/daemon.token` on each start.

**Forgot master password** — There is no recovery path. Maintain a regular
offline backup: `vaultx export -f vaultx -o backup.json`.
