# mypass
Free password manager from people who want security without cost

A self-hosted password manager stack combining **Vaultwarden** (Bitwarden-compatible backend), **Caddy** (auto-HTTPS reverse proxy), and a **custom Chrome extension** for autofill.

---

## Repository layout

```
mypass/
├── docker-compose.yml   # Production-ready Docker stack
├── Caddyfile            # Caddy reverse-proxy / TLS config
└── extension/           # Minimal Chrome extension (Manifest V3)
    ├── manifest.json
    ├── background.js
    ├── popup.html
    ├── popup.js
    └── content.js
```

---

## 1 — Docker stack (Vaultwarden + HTTPS)

### Prerequisites
- Docker ≥ 20 and Docker Compose ≥ 2
- Add `vault.local` to `/etc/hosts`:
  ```
  127.0.0.1 vault.local
  ```

### Start the stack
```bash
docker-compose up -d
```

This starts:
| Service | Role |
|---------|------|
| `vaultwarden` | Bitwarden-compatible API + web vault |
| `caddy` | Automatic HTTPS reverse proxy |

The web vault is accessible at **https://vault.local** once the stack is running.

### Environment variables

Copy `.env.example` to `.env` and fill in your values:

```bash
cp .env.example .env
# then edit .env
```

| Variable | Description |
|----------|-------------|
| `DOMAIN` | Public URL of the vault (default `https://vault.local`) |
| `SIGNUPS_ALLOWED` | Set to `true` to allow new registrations |
| `ADMIN_TOKEN` | Strong random token for `/admin` panel — **required** |

Generate a secure admin token:
```bash
openssl rand -base64 48
```

> **Security note:** Never commit `.env` to source control. The `.gitignore` already excludes it.

---

## 2 — Fork & strip the Bitwarden extension (fast path)

If you want battle-tested crypto without writing it yourself:

```bash
git clone https://github.com/bitwarden/clients
cd clients/apps/browser
```

**Strip** (safe to remove):
- Marketing UI
- Account-creation flows
- Analytics / telemetry
- Extra vault views

**Keep**:
- `crypto.service.ts` — AES encryption, Argon2 key derivation
- `cipher.service.ts` — vault JSON parsing
- Autofill engine
- Vault sync logic

Then replace the popup UI with your own and point the API URL at your Vaultwarden instance.

---

## 3 — Minimal Chrome extension (from scratch)

The `extension/` directory contains a clean Manifest V3 base.

### Load unpacked in Chrome
1. Open `chrome://extensions`
2. Enable **Developer mode**
3. Click **Load unpacked** → select the `extension/` folder
4. Set your vault email in extension storage (one-time setup):
   ```js
   // Paste in the Chrome DevTools console while the extension popup is open
   chrome.storage.local.set({ vaultEmail: "you@example.com" });
   ```

### What's included

| File | Purpose |
|------|---------|
| `manifest.json` | Extension metadata and permissions |
| `background.js` | Service worker — routes autofill messages to content scripts |
| `popup.html` | Extension popup UI |
| `popup.js` | Unlock flow + vault item list |
| `content.js` | Page-level autofill injection |

### ⚠️ Crypto is a placeholder

`popup.js` calls `/api/accounts/prelogin` to fetch KDF parameters but **does not yet perform real key derivation or decryption**. Before using this in production you must implement:

1. **Argon2 / PBKDF2** master-key derivation using the KDF params returned by prelogin
2. **Authentication** against `/api/accounts/token`
3. **AES-256-CBC / AES-256-GCM** vault decryption

The cleanest path is to extract `crypto.service.ts` and `cipher.service.ts` from the Bitwarden browser extension and bundle them with this extension.

---

## Security rules (non-negotiable)

| Rule | |
|------|-|
| Never store the master password | ❌ |
| Never write the decrypted vault to disk | ❌ |
| Keep derived keys in memory only | ✅ |
| Auto-lock after inactivity | ✅ |
| Use HTTPS always | ✅ |
| Clear clipboard after copy | ✅ |

---

## How everything connects

```
[ Chrome Extension ]
        │  decrypts locally
        ▼
[ Vaultwarden API ]  ←─── returns encrypted vault JSON
        │
        ▼
[ Caddy (TLS termination) ]
```

### Optional: add your own API layer

```
Extension → Node/Express API → Vaultwarden
```

Useful for: faster search, custom tagging, CLI integration, audit logs.
