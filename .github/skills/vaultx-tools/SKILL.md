---
name: vaultx-tools
description: Use vaultx CLI and HTTP daemon to manage secrets, resolve vault references, and inject credentials into processes, Docker, and Kubernetes — without writing plaintext to disk.
---

# vaultx Tools Skill

## When to use
- You need to store, retrieve, or rotate secrets in the local encrypted vault.
- You need to create or audit a `vaultx.env` file for a project.
- You need to run a command with secrets injected from the vault.
- You need to import credentials from a password manager.
- You need to resolve secrets programmatically via the HTTP daemon.
- You need to inject secrets into Docker or Kubernetes workloads.

## Vault lifecycle

```bash
vaultx init                          # create vault (one time per machine)
vaultx unlock                        # unlock for this session
vaultx lock                          # clear key from memory
```

## Secret management

```bash
vaultx set <path> <value>            # store a secret
vaultx get <path>                    # retrieve a value
vaultx list [prefix]                 # list paths (values masked)
vaultx delete <path>                 # delete a secret
```

## Inject into processes

```bash
vaultx run -- <cmd>                  # resolve vaultx.env and exec
vaultx run --env staging.env -- <cmd>
eval $(vaultx shell)                 # inject into current shell
```

## Docker and Kubernetes

```bash
vaultx docker run -- <image>:<tag>
vaultx docker compose -- up -d
vaultx serve --port 7474             # start daemon (needed for k3d)
vaultx k3d setup                     # install ESO + SecretStore
vaultx k3d token                     # refresh k8s token secret
vaultx k3d status                    # check ESO health
```

## Import / Export

```bash
vaultx import ~/Downloads/bitwarden-export.json
vaultx import ~/Downloads/google-passwords.csv
vaultx import ~/Downloads/1password-export.csv --format 1password
vaultx export -f vaultx -o backup.json   # full lossless backup
vaultx export -f bitwarden -o bw.json
vaultx export -f csv -o secrets.csv
```

## HTTP daemon API

```bash
# Start daemon — writes token to ~/.vaultx/daemon.token
vaultx serve

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

## vaultx.env format

```env
# vault: references — resolved at runtime
DB_PASSWORD=vault:local/myapp/db_password
STRIPE_KEY=vault:work/Payments/api-key      # 1Password
REDIS_URL=vault:prod/myapp/redis-url         # HashiCorp / AWS

# Plain values — passed through unchanged
PORT=3000
NODE_ENV=production
```

## Provider health

```bash
vaultx providers    # check all configured providers
```

## Output expectations
- `vaultx get` prints the raw value to stdout — safe to capture in scripts.
- `vaultx list` masks values — safe to share/log.
- All diagnostic messages go to stderr; secret values go to stdout only.
- The daemon returns JSON: `{"value": "<secret>"}` for `/v1/secret`.
