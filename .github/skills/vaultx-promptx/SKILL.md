---
name: vaultx-promptx
description: Use vaultx and promptx together — vaultx provides zero-trust secret injection, promptx provides encrypted memory and cross-tool context handoff. Neither tool requires the other, but together they cover secrets + intelligence.
---

# vaultx + promptx Integration Skill

## Division of responsibility

| Concern | Tool |
| --- | --- |
| Store and inject secrets | **vaultx** |
| Remember decisions, context, chat history | **promptx** |
| Run commands with secrets | `vaultx run -- <cmd>` |
| Run commands with memory context | `promptx generate`, `promptx ask` |
| Run commands with **both** | `vaultx run -- promptx <cmd>` |

## Run promptx with secrets injected from vaultx

Store promptx secrets in vaultx, then run via `vaultx run`:

```bash
# Store promptx passkey in vaultx (one time)
vaultx set promptx/passkey "$(promptx passkey-change --print)"

# Run any promptx command with PROMPTX_PASSKEY injected
vaultx run -- promptx memory-query "architecture decisions" --repo . --limit 5
vaultx run -- promptx ask "what changed in auth?" --repo . --limit 6
vaultx run -- promptx generate "add retry logic" --context "uses Go stdlib"
vaultx run -- promptx chat-helper "explain this function" --repo . --include-memory
vaultx run -- promptx commits --repo . --limit 20 --pretty
```

`vaultx.env` for a project using promptx:

```env
PROMPTX_PASSKEY=vault:local/promptx/passkey
STRIPE_KEY=vault:work/Payments/api-key
DB_URL=vault:local/myapp/db_url
```

Then:
```bash
vaultx run -- promptx memory-watch --repo . --interval 30 --force-store
```

## Run vaultx operations with promptx memory context

Use promptx to recall past vault configurations, import decisions, and infra patterns:

```bash
# Recall past vaultx decisions
promptx memory-query "vaultx provider config" --repo . --limit 5
promptx ask "which secrets are configured for this project?" --repo . --limit 6
promptx ask "when did we add the 1Password provider?" --repo . --limit 8

# Log a vaultx decision to memory
promptx memory-write "Decision: use vault:work/ prefix for all Stripe keys, vault:local/ for DB passwords" \
  --repo . --type decision --tags vaultx,secrets,architecture --force-store

# Search for past import operations
promptx fuzzy-search "vaultx import bitwarden" --repo . --limit 5
```

## Cross-tool handoff with secrets

Resume work across tools while preserving secret references:

```bash
# Hand off from Claude Code to Cursor, carrying vaultx context
promptx switch \
  --from claude-code --to cursor \
  --repo . \
  --prompt "setting up vaultx providers for staging" \
  --response "configured vault:work/ for Stripe, vault:local/ for DB" \
  --model auto

# Resume in the next tool
promptx resume --from claude-code --to cursor --repo . --limit 10
```

## Promptx MCP / bridge with vaultx secrets available

When running promptx serve with secrets injected:

```bash
# Start daemon with all secrets in environment
vaultx run -- promptx serve

# Or inject only the passkey
PROMPTX_PASSKEY=$(vaultx get promptx/passkey) promptx serve
```

## Promptx CLI — quick reference

```bash
# Memory
promptx memory-query "<question>" --repo . --limit 10
promptx memory-write "<content>" --repo . --tags "tag1,tag2"
promptx memory-watch --repo . --interval 30 --force-store
promptx memory-watch --stop

# Search and ask
promptx fuzzy-search "<query>" --repo . --limit 8
promptx ask "<question>" --repo . --limit 6
promptx chat-helper "<question>" --repo . --include-memory --memory-limit 20

# History
promptx commits --repo . --limit 20 --pretty --group-by-assistant
promptx graph --repo . --window 200 --json
promptx insights --limit 100

# Cross-tool
promptx resume --from claude-code --to cursor --repo . --limit 20
promptx switch --from claude-code --to cursor --repo . --prompt "..." --response "..."
```

## Promptx Bridge — with vaultx secrets

```bash
echo '{"action":"chat_helper","passkey":"<pass>","repo":".","query":"which vault path holds the DB password?","limit":6,"include_memory":true}' | promptx bridge
echo '{"action":"memory_write","passkey":"<pass>","content":"vaultx: added vault:prod/ provider for AWS Secrets Manager","tags":"vaultx,infra"}' | promptx bridge
echo '{"action":"memory_query","passkey":"<pass>","query":"vaultx provider config","limit":10}' | promptx bridge
echo '{"action":"ask","passkey":"<pass>","repo":".","query":"what secrets does this project need?","limit":6}' | promptx bridge
```

## Promptx MCP — from any IDE

```json
{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"chat_helper","arguments":{"passkey":"<pass>","repo_path":".","question":"what vault references are used in this project?","limit":6,"include_memory":"true","memory_limit":20}}}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_write","arguments":{"passkey":"<pass>","content":"vaultx: rotated DB password, updated vault:local/myapp/db_password","tags":"vaultx,rotation"}}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_query","arguments":{"passkey":"<pass>","query":"vault secret paths this project uses","limit":10}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"resume","arguments":{"passkey":"<pass>","repo_path":".","from_tool":"claude-code","to_tool":"cursor","limit":20}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"ask","arguments":{"passkey":"<pass>","repo_path":".","question":"which secrets have been rotated recently?","limit":6}}}
```

## Output expectations
- Prefer `vaultx get` for scripts; `vaultx list` for audits (values always masked).
- Prefer promptx `memory_query` over re-asking questions the AI has already answered.
- Always log significant vault decisions (new provider, rotation, import) to promptx memory.
