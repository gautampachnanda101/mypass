Read the current project's source files and existing `.env` or `vaultx.env` to understand what environment variables the application needs. Then:

1. List every environment variable the app reads (search for `os.Getenv`, `process.env`, `ENV[`, `dotenv`, etc.).
2. For each variable, propose a `vault:local/<project>/<key>` reference path using the project name derived from the directory or `go.mod`/`package.json`.
3. Write a `vaultx.env` file with:
   - All secret variables as `vault:local/...` references
   - Non-secret variables (PORT, NODE_ENV, LOG_LEVEL, etc.) as plain values
   - A comment above each `vault:` line explaining what the secret is for
4. Print the `vaultx set` commands the user needs to run to populate the vault.
5. Note: plain values (no `vault:` prefix) pass through unchanged — existing `.env` files are valid `vaultx.env` files.
