Migrate this project's Docker setup from env-file-based secrets to vaultx injection.

1. Read `docker-compose*.yml` and any `Dockerfile` in the project.
2. Find all `env_file:`, `--env-file`, and `environment:` entries that reference `.env` files or hardcoded secrets.
3. For each secret found, propose the `vaultx set` command to store it and the `vault:local/...` reference to use in `vaultx.env`.
4. Rewrite the relevant `docker-compose.yml` service entries to remove `env_file:` references — `vaultx docker compose` injects secrets directly into the child environment so no `env_file:` is needed.
5. Show the before/after diff.
6. Show the commands to run:

```bash
# Store secrets
vaultx set myapp/db_password "..."

# Run with secrets injected (replaces: docker compose up -d)
vaultx docker compose -- up -d

# Or for a single container (replaces: docker run --env-file .env myimage)
vaultx docker run -- myimage:latest
```

7. Note the security improvement: secrets are passed as `--env KEY=VAL` args to the Docker daemon rather than as a file on disk, so they do not appear in `docker inspect` env_file entries.
