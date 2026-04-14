Help the user import credentials from their current password manager into vaultx.

1. Ask which password manager they're exporting from if not clear from context: Google Password Manager, 1Password, Bitwarden, LastPass, Samsung Pass, McAfee True Key, Dashlane, Keeper, or generic CSV.

2. Show the exact export steps for their password manager (UI path or CLI command).

3. Run the import:

```bash
# Auto-detected format
vaultx import ~/Downloads/export.csv
vaultx import ~/Downloads/bitwarden-export.json

# Explicit format
vaultx import ~/Downloads/export.csv --format 1password
vaultx import ~/Downloads/export.csv --format google
vaultx import ~/Downloads/export.csv --format lastpass
```

Supported formats: `google`, `1password`, `bitwarden`, `lastpass`, `samsung`, `mcafee`, `dashlane`, `keeper`, `csv`, `vaultx`

4. After import, verify with `vaultx list` and spot-check a few entries with `vaultx get`.

5. **Security reminder**: delete the export file after import — it contains plaintext passwords.

```bash
# Verify import
vaultx list
vaultx get <path>

# Delete export file
rm ~/Downloads/export.csv    # or .json
```

6. Take a backup in vaultx format:

```bash
vaultx export -f vaultx -o ~/vaultx-backup-$(date +%Y%m%d).json
# Store the backup file encrypted (e.g. in a safe location, not in git)
```
