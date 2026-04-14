# vaultx roadmap

This file tracks future aspirations that are not implemented in the current binary.

## Current implemented scope

- Providers: `local`, `onepassword`
- Vault lifecycle: `init`, `unlock`, `lock`
- Secret CRUD: `set`, `get`, `list`, `delete`
- Injection: `run`, `shell`, `docker run`, `docker compose`
- Operations: `serve`, `k3d`, `import`, `export`, `providers`, `doctor`, `completion`, `version`

## Aspirational roadmap (not implemented yet)

- Additional providers beyond `local` and `onepassword`.
- Extra secret utility commands such as clipboard copy and generator workflows.
- Expanded audit and policy capabilities in the daemon.
- Broader platform and ecosystem integrations over time.

Any roadmap item above should be treated as non-available until code and command help ship in the binary.
