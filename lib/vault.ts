/**
 * Vault CRUD backed by localStorage.
 *
 * The vault is stored as:
 *   localStorage["mypass_vault"] = JSON.stringify(VaultStore)
 *
 * Passwords inside each item are AES-256-GCM encrypted — the raw
 * CryptoKey lives only in memory and is never persisted.
 */

import { VaultItem, VaultStore } from "@/types";
import {
  bufferToBase64,
  base64ToBuffer,
  deriveKey,
  encrypt,
  decrypt,
  generateSalt,
} from "@/lib/crypto";

const STORAGE_KEY = "mypass_vault";

// ---------------------------------------------------------------------------
// Low-level storage helpers
// ---------------------------------------------------------------------------

function loadRaw(): VaultStore | null {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as VaultStore;
  } catch {
    return null;
  }
}

function saveRaw(store: VaultStore): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(store));
}

// ---------------------------------------------------------------------------
// Key management
// ---------------------------------------------------------------------------

/**
 * Bootstrap a brand-new vault for a master password.
 * Generates a fresh salt and returns the derived CryptoKey.
 */
export async function initVault(masterPassword: string): Promise<CryptoKey> {
  const salt = generateSalt();
  const key = await deriveKey(masterPassword, salt);
  saveRaw({ items: [], salt: bufferToBase64(salt) });
  return key;
}

/**
 * Re-derive the CryptoKey from the persisted salt + master password.
 * Throws if no vault has been initialised yet.
 */
export async function unlockVault(masterPassword: string): Promise<CryptoKey> {
  const store = loadRaw();
  if (!store) throw new Error("No vault found. Create one first.");
  const salt = base64ToBuffer(store.salt);
  return deriveKey(masterPassword, salt);
}

export function vaultExists(): boolean {
  return loadRaw() !== null;
}

// ---------------------------------------------------------------------------
// CRUD
// ---------------------------------------------------------------------------

export function listItems(): VaultItem[] {
  return loadRaw()?.items ?? [];
}

/**
 * Add a new vault item, encrypting the password with the in-memory key.
 */
export async function addItem(
  key: CryptoKey,
  item: Omit<VaultItem, "id" | "encryptedPassword" | "iv" | "createdAt" | "updatedAt">,
  plainPassword: string
): Promise<VaultItem> {
  const store = loadRaw();
  if (!store) throw new Error("Vault is locked or does not exist.");

  const { ciphertext, iv } = await encrypt(plainPassword, key);
  const now = Date.now();
  const newItem: VaultItem = {
    ...item,
    id: crypto.randomUUID(),
    encryptedPassword: ciphertext,
    iv,
    createdAt: now,
    updatedAt: now,
  };

  store.items.push(newItem);
  saveRaw(store);
  return newItem;
}

/**
 * Decrypt a single item's password using the in-memory key.
 */
export async function getPassword(
  key: CryptoKey,
  itemId: string
): Promise<string> {
  const items = listItems();
  const item = items.find((i) => i.id === itemId);
  if (!item) throw new Error(`Item ${itemId} not found.`);
  return decrypt(item.encryptedPassword, item.iv, key);
}

/**
 * Update an existing vault item. Pass a new plainPassword to re-encrypt,
 * or omit it to keep the existing encrypted value.
 */
export async function updateItem(
  key: CryptoKey,
  itemId: string,
  updates: Partial<Omit<VaultItem, "id" | "encryptedPassword" | "iv" | "createdAt" | "updatedAt">>,
  newPlainPassword?: string
): Promise<void> {
  const store = loadRaw();
  if (!store) throw new Error("Vault is locked or does not exist.");

  const idx = store.items.findIndex((i) => i.id === itemId);
  if (idx === -1) throw new Error(`Item ${itemId} not found.`);

  const updated = { ...store.items[idx], ...updates, updatedAt: Date.now() };

  if (newPlainPassword !== undefined) {
    const { ciphertext, iv } = await encrypt(newPlainPassword, key);
    updated.encryptedPassword = ciphertext;
    updated.iv = iv;
  }

  store.items[idx] = updated;
  saveRaw(store);
}

/**
 * Permanently remove a vault item.
 */
export function deleteItem(itemId: string): void {
  const store = loadRaw();
  if (!store) return;
  store.items = store.items.filter((i) => i.id !== itemId);
  saveRaw(store);
}

/**
 * Wipe the entire vault from localStorage.
 */
export function destroyVault(): void {
  if (typeof window !== "undefined") {
    localStorage.removeItem(STORAGE_KEY);
  }
}
