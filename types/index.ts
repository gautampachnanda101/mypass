export interface VaultItem {
  id: string;
  name: string;
  username: string;
  /** AES-256-GCM ciphertext (base64) */
  encryptedPassword: string;
  /** AES-256-GCM IV (base64) */
  iv: string;
  url?: string;
  notes?: string;
  createdAt: number;
  updatedAt: number;
}

export interface VaultStore {
  items: VaultItem[];
  /** PBKDF2 salt (base64) — stored alongside the vault so the key can be re-derived */
  salt: string;
}

export interface KdfParams {
  kdfType: number; // 0 = PBKDF2, 1 = Argon2id
  kdfIterations: number;
  kdfMemory?: number;
  kdfParallelism?: number;
}
