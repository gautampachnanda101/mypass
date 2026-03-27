# mypass
Free password manager for people who want security without cost.

A fully self-hosted three-layer stack:

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Web app** | Next.js 15 + TypeScript | Vault UI — create, view and manage credentials |
| **API / storage** | Vaultwarden (Bitwarden-compatible) | Encrypted vault sync & REST API |
| **Reverse proxy** | Caddy | Automatic HTTPS, routes requests to the right service |
| **Browser autofill** | Chrome extension (Manifest V3) | One-click autofill on any website |

---

## Repository layout

```
mypass/
├── app/                    # Next.js 15 web app
│   ├── layout.tsx
│   ├── globals.css
│   ├── page.tsx            # Login / vault unlock
│   └── vault/
│       └── page.tsx        # Vault dashboard (CRUD)
├── lib/
│   ├── crypto.ts           # PBKDF2 key derivation + AES-256-GCM
│   └── vault.ts            # localStorage CRUD (encrypted)
├── types/
│   └── index.ts            # Shared TypeScript types
├── extension/              # Chrome extension (Manifest V3)
│   ├── manifest.json
│   ├── background.js
│   ├── popup.html
│   ├── popup.js
│   └── content.js
├── docker-compose.yml      # Unified Docker stack (webapp + vaultwarden + caddy)
├── Caddyfile               # Reverse-proxy routing + auto-TLS
├── Dockerfile              # Multi-stage Next.js build
└── .env.example            # Required environment variables
```

---

## How everything connects

```
Browser / Chrome extension
        │
        ▼
  Caddy (:443 / TLS)
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

### 2. Add the local hostname

```bash
echo "127.0.0.1 vault.local" | sudo tee -a /etc/hosts
```

### 3. Start the full stack

```bash
docker-compose up -d
```

| Service | URL |
|---------|-----|
| MyPass web app | https://vault.local |
| Vaultwarden admin | https://vault.local/admin |

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

### What it does

The extension calls the Vaultwarden **prelogin** endpoint to fetch KDF parameters, then the popup lets you unlock and autofill credentials on any page.

> ⚠️ **Crypto stub:** `popup.js` fetches KDF params but real key derivation + vault decryption are marked `TODO`. See `lib/crypto.ts` for the reference implementation to port.

---

## Environment variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ADMIN_TOKEN` | *(required)* | Vaultwarden `/admin` panel token |
| `VAULT_DOMAIN` | `https://vault.local` | Public URL (must match Caddyfile) |
| `SIGNUPS_ALLOWED` | `false` | Allow new registrations |
| `WEBSOCKET_ENABLED` | `true` | Live vault sync notifications |

---

## Security rules

| Rule | |
|------|-|
| Never store the master password | ❌ |
| Never write the decrypted vault to disk | ❌ |
| Keep derived keys in memory only | ✅ |
| Auto-lock after inactivity | ✅ (TODO) |
| HTTPS always | ✅ |
| Clear clipboard after copy | ✅ (TODO) |
