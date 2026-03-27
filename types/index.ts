export interface Credential {
  id: string;
  name: string;
  username: string;
  password: string;
  url: string;
  notes: string;
  createdAt: number;
  updatedAt: number;
}

export interface Vault {
  credentials: Credential[];
}

/** Encrypted vault stored in localStorage (key is derived from master password + auth salt). */
export interface EncryptedVault {
  iv: string;
  data: string;
}
