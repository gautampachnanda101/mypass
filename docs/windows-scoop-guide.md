# vaultx on Windows — Scoop Guide

Install and use vaultx on Windows via [Scoop](https://scoop.sh).

## Prerequisites

- Windows 10 (1903+) or Windows 11, 64-bit
- PowerShell 5.1 or PowerShell 7+
- Scoop installed:

```powershell
irm get.scoop.sh | iex
```

## Install

```powershell
scoop bucket add vaultx https://github.com/gautampachnanda101/scoop-bucket
scoop install vaultx
```

Verify:

```powershell
vaultx version
vaultx --help
```

## Upgrade

```powershell
scoop update vaultx
```

## Uninstall

```powershell
scoop uninstall vaultx
scoop bucket rm vaultx
```

---

## First-time setup

### 1. Create a vault

```powershell
vaultx init
```

### 2. Store secrets

```powershell
vaultx set myapp/db_password "s3cr3t"
vaultx set myapp/api_key "sk-live-abc123"
```

### 3. Create vaultx.env (commit this file)

```powershell
@"
DB_PASSWORD=vault:local/myapp/db_password
API_KEY=vault:local/myapp/api_key
PORT=3000
"@ | Out-File -Encoding utf8 vaultx.env
```

### 4. Run your app

```powershell
vaultx run -- npm start
```

---

## Daily use

```powershell
vaultx list                        # list all secrets (values masked)
vaultx get myapp/db_password       # print a single value
vaultx set myapp/new_key "value"   # store a secret

vaultx run -- .\server.exe         # inject secrets and run a process
vaultx docker run -- myapp:latest  # docker run with secrets as --env flags
```

---

## Notes

- The vault is stored at `%USERPROFILE%\.vaultx\vault.enc`.
- The master password is never stored — you will be prompted on each unlock.
- Run `vaultx lock` to clear the key from memory when stepping away.
- For 1Password integration, install the `op` CLI and sign in before running
  any command that references `vault:work/…`.
