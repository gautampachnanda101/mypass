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
| `vaultx.env` safe to commit | Contains only references (`vault:provider/path`), never values |
| Provider credentials not in config | Tokens sourced from env vars or OS keychain, never written to config file |
| Audit trail (future) | Daemon logs every resolution: who, what, when |
| Rotation | Update in vault once; all `vaultx run` invocations pick it up immediately |

---

## Non-Goals (v1)

- Secret sharing / team sync (use 1Password or Vault for that)
- Secret versioning / history (provider-dependent)
- Web UI hosted externally (local daemon only)
- Windows support (v2)
