"use client";

import React, { createContext, useContext, useState } from "react";

interface VaultContextValue {
  key: CryptoKey | null;
  setKey: (key: CryptoKey | null) => void;
}

const VaultContext = createContext<VaultContextValue>({
  key: null,
  setKey: () => undefined,
});

export function VaultProvider({ children }: { children: React.ReactNode }) {
  const [key, setKey] = useState<CryptoKey | null>(null);
  return (
    <VaultContext.Provider value={{ key, setKey }}>
      {children}
    </VaultContext.Provider>
  );
}

export function useVaultKey() {
  return useContext(VaultContext);
}
