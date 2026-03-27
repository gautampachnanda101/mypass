import { Vault, EncryptedVault, Credential } from "@/types";
import {
  deriveSessionKey,
  encryptWithKey,
  decryptWithKey,
  deriveMasterPasswordHash,
} from "./crypto";

const VAULT_KEY = "mypass_vault";
const AUTH_KEY = "mypass_auth";
/** Session key (derived from master password) stored for the browser session only. */
const SESSION_KEY = "mypass_sk";

export interface AuthData {
  /** PBKDF2 hash of the master password (for verification only). */
  passwordHash: string;
  /** Salt used to verify the master password (separate from encryption salt). */
  hashSalt: string;
  /** Salt used to derive the AES encryption key. */
  keySalt: string;
}

export function getEncryptedVault(): EncryptedVault | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(VAULT_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as EncryptedVault;
  } catch {
    return null;
  }
}

export function getAuthData(): AuthData | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(AUTH_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthData;
  } catch {
    return null;
  }
}

export function isVaultSetup(): boolean {
  if (typeof window === "undefined") return false;
  return !!localStorage.getItem(AUTH_KEY);
}

/** Retrieves the session key (derived key bytes) from sessionStorage. */
export function getSessionKey(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(SESSION_KEY);
}

/** Stores the session key (derived key bytes) in sessionStorage. */
export function setSessionKey(sessionKey: string): void {
  sessionStorage.setItem(SESSION_KEY, sessionKey);
}

/** Removes the session key, effectively locking the vault. */
export function clearSessionKey(): void {
  sessionStorage.removeItem(SESSION_KEY);
}

export async function saveVault(
  vault: Vault,
  sessionKey: string
): Promise<void> {
  const plaintext = JSON.stringify(vault);
  const encrypted = await encryptWithKey(plaintext, sessionKey);
  localStorage.setItem(VAULT_KEY, JSON.stringify(encrypted));
}

export async function loadVault(sessionKey: string): Promise<Vault> {
  const encrypted = getEncryptedVault();
  if (!encrypted) {
    return { credentials: [] };
  }
  const plaintext = await decryptWithKey(encrypted, sessionKey);
  return JSON.parse(plaintext) as Vault;
}

export async function setupMasterPassword(
  masterPassword: string
): Promise<string> {
  const hashSaltBytes = crypto.getRandomValues(new Uint8Array(16));
  const keySaltBytes = crypto.getRandomValues(new Uint8Array(16));
  const hashSalt = btoa(String.fromCharCode(...hashSaltBytes));
  const keySalt = btoa(String.fromCharCode(...keySaltBytes));

  const passwordHash = await deriveMasterPasswordHash(masterPassword, hashSalt);
  const sessionKey = await deriveSessionKey(masterPassword, keySalt);

  const authData: AuthData = { passwordHash, hashSalt, keySalt };
  localStorage.setItem(AUTH_KEY, JSON.stringify(authData));
  await saveVault({ credentials: [] }, sessionKey);
  return sessionKey;
}

/**
 * Verifies the master password and, if correct, returns the derived session key.
 * Returns null if the password is incorrect.
 */
export async function verifyMasterPassword(
  masterPassword: string
): Promise<string | null> {
  const authData = getAuthData();
  if (!authData) return null;

  const hash = await deriveMasterPasswordHash(masterPassword, authData.hashSalt);
  if (hash !== authData.passwordHash) return null;

  return deriveSessionKey(masterPassword, authData.keySalt);
}

export function createCredential(
  data: Omit<Credential, "id" | "createdAt" | "updatedAt">
): Credential {
  return {
    ...data,
    id: crypto.randomUUID(),
    createdAt: Date.now(),
    updatedAt: Date.now(),
  };
}

export function updateCredential(
  credentials: Credential[],
  id: string,
  data: Partial<Omit<Credential, "id" | "createdAt" | "updatedAt">>
): Credential[] {
  return credentials.map((c) =>
    c.id === id ? { ...c, ...data, updatedAt: Date.now() } : c
  );
}

export function deleteCredential(
  credentials: Credential[],
  id: string
): Credential[] {
  return credentials.filter((c) => c.id !== id);
}

export function clearVault(): void {
  if (typeof window === "undefined") return;
  localStorage.removeItem(VAULT_KEY);
  localStorage.removeItem(AUTH_KEY);
  clearSessionKey();
}

