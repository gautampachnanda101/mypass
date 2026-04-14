Audit this project for secret safety. Check:

1. **Committed secrets** — search git history and tracked files for patterns that look like real secret values: API keys (`sk-`, `pk_`, `ghp_`, `xoxb-`), passwords in config files, tokens in `.env` files that are not `.gitignore`d.

2. **vaultx.env hygiene** — if `vaultx.env` exists, verify every `vault:` reference follows the `vault:<provider>/<path>` format and that no raw values are present alongside references.

3. **Missing .gitignore entries** — check that `.env`, `.env.local`, `*.pem`, `*.key`, `vault.enc`, `daemon.token` are ignored.

4. **Hardcoded credentials in source** — grep for string literals that look like secrets in Go/TypeScript/Python/shell files.

5. **Docker and CI exposure** — check `docker-compose*.yml`, GitHub Actions workflows, and Dockerfiles for `--build-arg` or `ENV` instructions that embed secrets.

Report findings grouped by severity (critical / warning / info) with file paths and line numbers. For each critical finding, suggest the `vaultx set` command to move it into the vault and the `vault:` reference to replace it with.
