# mypass
Free password manager for people who want security without cost.

A fully self-hosted three-layer stack running on **dev-uk.uk** with automatic TLS:

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Web app** | Next.js 15 + TypeScript | Vault UI — create, view and manage credentials |
| **API / storage** | Vaultwarden (Bitwarden-compatible) | Encrypted vault sync & REST API |
| **Reverse proxy** | Caddy | Automatic HTTPS via Let's Encrypt |
| **Browser autofill** | Chrome extension (Manifest V3) | One-click autofill on any website |

---

## Repository layout

```
mypass/
├── app/                         # Next.js 15 web app
│   ├── layout.tsx
│   ├── globals.css
│   ├── page.tsx                 # Login / vault unlock
│   ├── vault/
│   │   └── page.tsx             # Vault dashboard (CRUD + import)
│   └── components/
│       ├── VaultProvider.tsx    # React context — holds CryptoKey in memory only
│       └── ImportPasswords.tsx  # CSV import from Google, Samsung, McAfee, etc.
├── lib/
│   ├── crypto.ts                # PBKDF2 key derivation + AES-256-GCM
│   └── vault.ts                 # localStorage CRUD (encrypted)
├── types/
│   └── index.ts                 # Shared TypeScript types
├── extension/                   # Chrome extension (Manifest V3)
│   ├── manifest.json
│   ├── background.js
│   ├── popup.html
│   ├── popup.js
│   └── content.js
├── docker-compose.yml           # Unified Docker stack (webapp + vaultwarden + caddy)
├── Caddyfile                    # Reverse-proxy routing + auto-TLS for dev-uk.uk
├── Dockerfile                   # Multi-stage Next.js build
└── .env.example                 # Required environment variables
```

---

## How everything connects

```
Browser / Chrome extension
        │
        ▼
  Caddy (:443 / TLS — auto-provisioned by Let's Encrypt for dev-uk.uk)
   │         │
   │ /        │ /api, /admin
   ▼          ▼
Next.js    Vaultwarden
(port 3000) (port 80)
```

- **`/`** → Next.js web app (MyPass UI)
- **`/api/*`** → Vaultwarden REST API
- **`/notifications/hub*`** → Vaultwarden WebSocket (live vault sync)
- **`/admin*`** → Vaultwarden admin panel

---

## Quick start (Docker)

### 1. Configure secrets

```bash
cp .env.example .env
# Edit .env — at minimum set ADMIN_TOKEN
openssl rand -base64 48   # use this as ADMIN_TOKEN
```

### 2. Make sure dev-uk.uk points to your server

Add an A record (or AAAA for IPv6) in your DNS console:

```
dev-uk.uk  →  <your server's public IP>
```

Caddy will automatically obtain a Let's Encrypt TLS certificate as soon as the
container starts — no manual certificate management needed.

> **Note:** ports 80 and 443 must be open in your firewall/security group for the
> ACME HTTP-01 challenge to succeed.

### 3. Start the full stack

```bash
docker-compose up -d
```

| Service | URL |
|---------|-----|
| MyPass web app | https://dev-uk.uk |
| Vaultwarden admin | https://dev-uk.uk/admin |

---

## Local development (Next.js)

```bash
npm install
npm run dev
# → http://localhost:3000
```

The app uses the **Web Crypto API** (built into every modern browser and Node ≥ 15):
- **Key derivation:** PBKDF2-SHA-256, 600 000 iterations
- **Encryption:** AES-256-GCM (authenticated encryption)
- **Storage:** encrypted ciphertexts in `localStorage`; the raw key lives in memory only

---

## Importing passwords from another password manager

The vault dashboard includes an **Import passwords** section. Click **Choose CSV file**
and pick your export — the importer auto-detects the format:

| Password manager | How to export |
|-----------------|--------------|
| **Google Password Manager** | passwords.google.com → Settings → Export passwords → CSV |
| **Samsung Pass** | Samsung Pass app → Settings → Import/Export → Export as CSV |
| **McAfee True Key** | True Key app → Settings → Export → CSV |
| **LastPass** | Account Options → Advanced → Export → LastPass CSV File |
| **1Password** | File → Export → All Items → CSV |
| **Bitwarden** | Tools → Export Vault → .csv |
| **Generic** | Any CSV with columns: `name`/`title`, `username`/`email`, `password`, `url` |

All passwords are **encrypted immediately** with your master key — the plaintext
is never stored.

---

## Chrome extension

### Load unpacked

1. Open `chrome://extensions`
2. Enable **Developer mode**
3. Click **Load unpacked** → select the `extension/` folder
4. Set your vault email (once):
   ```js
   // DevTools console while extension popup is open
   chrome.storage.local.set({ vaultEmail: "you@example.com" });
   ```

The extension connects to `https://dev-uk.uk/api` for vault operations.

> ⚠️ **Crypto stub:** `popup.js` fetches KDF params but real key derivation + vault decryption are marked `TODO`. See `lib/crypto.ts` for the reference implementation to port.

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ADMIN_TOKEN` | *(required)* | Vaultwarden `/admin` panel token |
| `VAULT_DOMAIN` | `https://dev-uk.uk` | Public URL (must match Caddyfile hostname) |
| `SIGNUPS_ALLOWED` | `false` | Allow new registrations |
| `WEBSOCKET_ENABLED` | `true` | Live vault sync notifications |

---

## Security rules

| Rule | |
|------|-|
| Never store the master password | ❌ |
| Never write the decrypted vault to disk | ❌ |
| Keep derived keys in memory only | ✅ |
| HTTPS always (Let's Encrypt on dev-uk.uk) | ✅ |
| Imported passwords encrypted before storage | ✅ |
