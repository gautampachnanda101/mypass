# vaultx User Guide

The convenience of an env file. The power of a zero-trust vault.

`vaultx` resolves `vault:` secret references at runtime and injects real values
into processes, Docker containers, and Kubernetes pods — nothing ever touches disk.

---

## Install

### macOS and Linux (Homebrew)

```bash
brew tap gautampachnanda101/homebrew-tap
brew install vaultx
```

Upgrade:

```bash
brew upgrade vaultx
```

### Windows (Scoop)

```powershell
scoop bucket add vaultx https://github.com/gautampachnanda101/scoop-bucket
scoop install vaultx
```

Upgrade:

```powershell
scoop update vaultx
```

### Direct download

Download release archives and `checksums.txt` from `gautampachnanda101/homebrew-tap` releases.

Verify the install:

```bash
vaultx --help
vaultx version
```

---

## Quick start

```bash
# 1. Create a new local vault (one time)
vaultx init

# 2. Store secrets
vaultx set myapp/db_password "s3cr3t"
vaultx set myapp/api_key "sk-live-abc123"

# 3. Create vaultx.env — this file is safe to commit
cat > vaultx.env <<EOF
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:local/myapp/api_key
PORT=3000
EOF

# 4. Run your app — secrets injected, nothing written to disk
vaultx run -- npm start
```

---

## How it works

`vaultx.env` is a drop-in replacement for `.env`. Plain values pass through
unchanged. Values prefixed with `vault:` are resolved at runtime from the
configured provider.

```
vault:<provider-id>/<path>
```

| Reference | Provider |
| --- | --- |
| `vault:local/myapp/key` | Local encrypted file vault |
| `vault:work/Vault/stripe` | 1Password (via `op` CLI) |
| `vault:myapp/key` (no prefix) | Default provider |

Plain values (no `vault:` prefix) are passed through as-is — any existing
`.env` file is a valid `vaultx.env` file.

---

## Vault lifecycle

```bash
vaultx init          # Create a new vault at ~/.vaultx/vault.enc
vaultx unlock        # Unlock for this session (prompts for master password)
vaultx lock          # Clear the key from memory
```

The vault is locked automatically when the daemon exits.

---

## Secret management

```bash
vaultx set myapp/db_password "s3cr3t"     # Store a secret
vaultx get myapp/db_password              # Retrieve a secret value
vaultx list                               # List all secrets (values masked)
vaultx list myapp/                        # List secrets under a prefix
vaultx delete myapp/db_password           # Delete a secret
```

---

## Running commands with secrets

Resolve `vaultx.env` and inject into a child process:

```bash
vaultx run -- npm start
vaultx run -- python manage.py runserver
vaultx run --env staging.env -- ./server
```

Inject into your current shell session:

```bash
eval $(vaultx shell)
eval $(vaultx shell --env staging.env)
```

---

## Docker

Instead of passing secrets in an env file visible to `docker inspect`:

```bash
# Before:
docker run --env-file .env myapp            # secrets on disk, visible in docker inspect

# After:
vaultx docker run -- myapp:latest           # --env KEY=VAL args only, nothing on disk
vaultx docker compose -- up -d             # docker compose inherits resolved env
```

---

## Import / Export

Import from any major password manager — format is auto-detected:

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
| `vaultx` | vaultx JSON (lossless round-trip) |

Export to file:

```bash
vaultx export -f bitwarden -o bw-backup.json
vaultx export -f vaultx   -o backup.json     # full backup
vaultx export -f csv      -o secrets.csv
```

---

## Multi-provider setup

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
```

With this config your `vaultx.env` can reference secrets from multiple providers:

```env
DB_PASS=vault:local/myapp/db       # local vault
STRIPE=vault:work/Payments/api-key  # 1Password
PORT=3000                           # plain value — passed through
```

Currently implemented providers in the binary are `local` and `onepassword`.

Check provider health:

```bash
vaultx providers
```

---

## Local HTTP daemon

`vaultx serve` exposes a local-only API on 127.0.0.1 for the VS Code extension
and k3d webhook:

```bash
vaultx serve             # default port 7474
vaultx serve --port 8080
```

```bash
TOKEN=$(cat ~/.vaultx/daemon.token)
curl -H "X-Vaultx-Token: $TOKEN" http://localhost:7474/v1/secret?path=myapp/db
```

| Endpoint | Description |
| --- | --- |
| `GET /health` | Liveness check (no auth) |
| `GET /v1/secret?path=<path>` | Resolve a single secret |
| `POST /v1/resolve` | Resolve a vaultx.env body |
| `GET /v1/list?prefix=<prefix>` | List secrets (values masked) |
| `GET /externalsecrets/<key>` | ESO webhook endpoint |

---

## Kubernetes / k3d

```bash
# 1. Start the daemon
vaultx serve --port 7474

# 2. One-time cluster setup (installs ESO + configures SecretStore)
vaultx k3d setup

# 3. Refresh the token secret after a daemon restart
vaultx k3d token

# 4. Check status
vaultx k3d status
```

---

## Full CLI reference

```
vaultx init                          Create a new local vault
vaultx unlock                        Unlock the vault for this session
vaultx lock                          Lock the vault (clear key from memory)
vaultx doctor                        Check runtime dependencies and vault health

vaultx set <path> <value>            Store a secret in the local vault
vaultx get <path>                    Get a single secret value
vaultx delete <path>                 Delete a secret
vaultx list [prefix]                 List secrets (values masked)

vaultx run [--env file] -- <cmd>     Resolve vaultx.env and exec a command
vaultx shell [--env file]            Print export statements for eval

vaultx import [--format f] <file>    Import from a password manager export
vaultx export [--format f] [-o file] Export to file

vaultx providers                     List configured providers and health
vaultx docs                          Pretty-print the public user guide
vaultx completion [shell]            Install shell completion

vaultx serve [--port N] [--max-memory MiB]
                                     Start local HTTP daemon (default 7474)

vaultx docker run -- <args>          docker run with secrets as --env flags
vaultx docker compose -- <args>      docker compose with secrets in child env

vaultx k3d setup                     Install ESO + configure SecretStore
vaultx k3d token                     Refresh vaultx-token k8s secret
vaultx k3d status                    Show ESO / SecretStore / ExternalSecret status

vaultx version                       Print version
```

Global flags: `--config <path>`, `--env <vaultx.env path>`, `--color auto|always|never`, `--emoji auto|always|never`

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

---

## Troubleshooting

**Vault is sealed after a reboot**
Run `vaultx unlock` to re-enter your master password. The daemon does not
persist the key across restarts by design.

**`vault:work/…` references not resolving**
Ensure the `op` CLI is installed and you are signed in: `op signin`.
Check `vaultx providers` for health status.

**Daemon token missing**
Start the daemon with `vaultx serve` — it writes a fresh token to
`~/.vaultx/daemon.token` on each startup.

**Wrong master password**
There is no password recovery. Keep an offline backup via `vaultx export -f vaultx`.

---

## Resources

- Source and releases: `https://github.com/gautampachnanda101/vaultx`
- Issues: open a GitHub issue on the source repository
