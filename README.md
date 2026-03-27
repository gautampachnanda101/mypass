# MyPass

Free password manager for people who want security without cost.

## Features

- **AES-256 encryption** – All passwords are encrypted client-side using AES-GCM via the Web Crypto API. Your vault never leaves your device.
- **Master password protection** – A PBKDF2-derived key (600,000 iterations, SHA-256) protects your vault.
- **Password generator** – Generate strong random passwords with configurable length, charset, and strength indicator.
- **Credential management** – Add, edit, delete, and search stored credentials (site, username, password, URL, notes).
- **One-click copy** – Copy usernames or passwords to the clipboard instantly.
- **Zero dependencies for crypto** – Uses the browser-native `SubtleCrypto` API.
- **Completely local** – No server, no account, no telemetry. Data stored in `localStorage`.

## Getting Started

```bash
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) to use the app.

## Build

```bash
npm run build
npm start
```

## Tech Stack

- [Next.js](https://nextjs.org/) 16 (App Router)
- TypeScript
- Tailwind CSS
- Web Crypto API (AES-GCM + PBKDF2)

