const PBKDF2_ITERATIONS = 600_000;
const KEY_LENGTH = 256;

function bufferToBase64(buffer: ArrayBuffer): string {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)));
}

function base64ToBuffer(base64: string): ArrayBuffer {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer as ArrayBuffer;
}

/**
 * Derives a 256-bit AES key from the master password and the stored salt.
 * Returns the raw key bytes as a base64 string suitable for session storage.
 * Storing key bytes rather than the plaintext password limits the impact of
 * a compromised session: an attacker cannot reverse the key to the password
 * or re-hash it without repeating the full PBKDF2 computation.
 */
export async function deriveSessionKey(
  masterPassword: string,
  saltBase64: string
): Promise<string> {
  const salt = base64ToBuffer(saltBase64);
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(masterPassword),
    "PBKDF2",
    false,
    ["deriveKey"]
  );
  const key = await crypto.subtle.deriveKey(
    {
      name: "PBKDF2",
      salt,
      iterations: PBKDF2_ITERATIONS,
      hash: "SHA-256",
    },
    keyMaterial,
    { name: "AES-GCM", length: KEY_LENGTH },
    true,
    ["encrypt", "decrypt"]
  );
  const raw = await crypto.subtle.exportKey("raw", key);
  return bufferToBase64(raw);
}

async function importKey(sessionKeyBase64: string): Promise<CryptoKey> {
  const raw = base64ToBuffer(sessionKeyBase64);
  return crypto.subtle.importKey("raw", raw, { name: "AES-GCM", length: KEY_LENGTH }, false, [
    "encrypt",
    "decrypt",
  ]);
}

export async function encryptWithKey(
  plaintext: string,
  sessionKeyBase64: string
): Promise<{ iv: string; data: string }> {
  const ivBytes = crypto.getRandomValues(new Uint8Array(12));
  const iv = ivBytes.buffer as ArrayBuffer;
  const key = await importKey(sessionKeyBase64);
  const encrypted = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv },
    key,
    new TextEncoder().encode(plaintext)
  );
  return {
    iv: bufferToBase64(iv),
    data: bufferToBase64(encrypted),
  };
}

export async function decryptWithKey(
  encryptedData: { iv: string; data: string },
  sessionKeyBase64: string
): Promise<string> {
  const iv = base64ToBuffer(encryptedData.iv);
  const data = base64ToBuffer(encryptedData.data);
  const key = await importKey(sessionKeyBase64);
  const decrypted = await crypto.subtle.decrypt({ name: "AES-GCM", iv }, key, data);
  return new TextDecoder().decode(decrypted);
}

/**
 * Derives a PBKDF2 verification hash from the master password and a dedicated
 * hash salt (separate from the encryption salt, so the same password input
 * produces an independent hash that cannot be directly used to derive the key).
 */
export async function deriveMasterPasswordHash(
  masterPassword: string,
  hashSaltBase64: string
): Promise<string> {
  const salt = base64ToBuffer(hashSaltBase64);
  const keyMaterial = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(masterPassword),
    "PBKDF2",
    false,
    ["deriveBits"]
  );
  const bits = await crypto.subtle.deriveBits(
    { name: "PBKDF2", salt, iterations: PBKDF2_ITERATIONS, hash: "SHA-256" },
    keyMaterial,
    256
  );
  return bufferToBase64(bits);
}

export function generatePassword(
  length: number,
  useUppercase: boolean,
  useLowercase: boolean,
  useNumbers: boolean,
  useSymbols: boolean
): string {
  const uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
  const lowercase = "abcdefghijklmnopqrstuvwxyz";
  const numbers = "0123456789";
  const symbols = "!@#$%^&*()-_=+[]{}|;:,.<>?";

  let charset = "";
  const required: string[] = [];

  const randomIndex = (str: string): number => {
    const buf = new Uint8Array(1);
    crypto.getRandomValues(buf);
    return buf[0] % str.length;
  };

  if (useUppercase) {
    charset += uppercase;
    required.push(uppercase[randomIndex(uppercase)]);
  }
  if (useLowercase) {
    charset += lowercase;
    required.push(lowercase[randomIndex(lowercase)]);
  }
  if (useNumbers) {
    charset += numbers;
    required.push(numbers[randomIndex(numbers)]);
  }
  if (useSymbols) {
    charset += symbols;
    required.push(symbols[randomIndex(symbols)]);
  }

  if (charset.length === 0) {
    charset = lowercase;
    required.push(lowercase[randomIndex(lowercase)]);
  }

  const array = new Uint8Array(length);
  crypto.getRandomValues(array);
  const passwordChars = Array.from(
    array,
    (byte) => charset[byte % charset.length]
  );

  for (let i = 0; i < required.length && i < length; i++) {
    const pos = new Uint8Array(1);
    crypto.getRandomValues(pos);
    passwordChars[pos[0] % length] = required[i];
  }

  return passwordChars.join("");
}
