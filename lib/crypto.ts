/**
 * Cryptographic helpers for MyPass.
 *
 * Key derivation: PBKDF2-SHA-256 (600 000 iterations — OWASP 2023 minimum).
 * Encryption:     AES-256-GCM (authenticated, 96-bit IV, 128-bit tag).
 *
 * All operations use the Web Crypto API (available in both browsers and
 * Node ≥ 15 / Next.js edge runtime) — no third-party crypto library needed.
 */

const PBKDF2_ITERATIONS = 600_000;
const KEY_USAGE_ENCRYPT: KeyUsage[] = ["encrypt", "decrypt"];

// ---------------------------------------------------------------------------
// Key derivation
// ---------------------------------------------------------------------------

/**
 * Derive a 256-bit AES-GCM key from a master password + salt using PBKDF2.
 *
 * @param masterPassword  - plaintext master password (never stored)
 * @param salt            - random 16-byte Uint8Array; persist alongside the vault
 * @param iterations      - PBKDF2 iteration count (default: 600 000)
 */
export async function deriveKey(
  masterPassword: string,
  salt: Uint8Array,
  iterations = PBKDF2_ITERATIONS
): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    enc.encode(masterPassword),
    "PBKDF2",
    false,
    ["deriveKey"]
  );
  return crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt: salt as unknown as BufferSource,
      iterations,
      hash: "SHA-256",
    },
    keyMaterial,
    { name: "AES-GCM", length: 256 },
    false,
    KEY_USAGE_ENCRYPT
  );
}

/**
 * Generate a cryptographically random 16-byte salt.
 */
export function generateSalt(): Uint8Array {
  return crypto.getRandomValues(new Uint8Array(16));
}

// ---------------------------------------------------------------------------
// Encryption / decryption
// ---------------------------------------------------------------------------

/**
 * Encrypt a plaintext string with AES-256-GCM.
 *
 * @returns { ciphertext, iv } — both as base64 strings for JSON storage.
 */
export async function encrypt(
  plaintext: string,
  key: CryptoKey
): Promise<{ ciphertext: string; iv: string }> {
  const enc = new TextEncoder();
  const ivBytes = crypto.getRandomValues(new Uint8Array(12)); // 96-bit IV

  const ciphertextBuffer = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: ivBytes },
    key,
    enc.encode(plaintext)
  );

  return {
    ciphertext: bufferToBase64(ciphertextBuffer),
    iv: bufferToBase64(ivBytes),
  };
}

/**
 * Decrypt an AES-256-GCM ciphertext.
 *
 * @throws {DOMException} if the key is wrong or data is tampered.
 */
export async function decrypt(
  ciphertext: string,
  iv: string,
  key: CryptoKey
): Promise<string> {
  const dec = new TextDecoder();
  const plainBuffer = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: base64ToBuffer(iv) as unknown as BufferSource },
    key,
    base64ToBuffer(ciphertext) as unknown as BufferSource
  );
  return dec.decode(plainBuffer);
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

export function bufferToBase64(buffer: ArrayBuffer | Uint8Array): string {
  const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
  let binary = "";
  // Process in 8 KB chunks to avoid call-stack overflow on large buffers
  const CHUNK = 8192;
  for (let i = 0; i < bytes.length; i += CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
  }
  return btoa(binary);
}

export function base64ToBuffer(b64: string): Uint8Array {
  return Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
}
