Help configure a new secret provider for vaultx. Ask the user which provider they want to add if not specified: local (default), 1Password, HashiCorp Vault, or AWS Secrets Manager.

Then:

1. Show the `~/.vaultx/config.toml` snippet for that provider.
2. Explain any prerequisites (e.g. `op` CLI for 1Password, `VAULT_TOKEN` env var for HashiCorp).
3. Show example `vault:<provider-id>/...` reference syntax for `vaultx.env`.
4. Run `vaultx providers` to check current health (if the vault is unlocked).
5. If the user wants 1Password: show how to sign in with `op signin` and verify with `op vault list`.
6. If the user wants HashiCorp: show how to set `VAULT_TOKEN` and test with `vault kv get`.
7. If the user wants AWS: show the IAM role ARN pattern and how to verify with `aws secretsmanager list-secrets`.

Config template:

```toml
# ~/.vaultx/config.toml

[vault]
path = "~/.vaultx/vault.enc"
kdf  = "argon2id"

[[providers]]
id      = "local"
type    = "local"
default = true

# 1Password
[[providers]]
id      = "work"
type    = "onepassword"
account = "my.1password.com"
vault   = "Work"

# HashiCorp Vault
[[providers]]
id        = "prod"
type      = "hashicorp"
address   = "https://vault.example.com"
token_env = "VAULT_TOKEN"

# AWS Secrets Manager
[[providers]]
id       = "aws"
type     = "aws"
region   = "eu-west-2"
role_arn = "arn:aws:iam::123456789:role/vaultx"
```
