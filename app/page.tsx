"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { initVault, unlockVault, vaultExists } from "@/lib/vault";
import { useVaultKey } from "./components/VaultProvider";

export default function LoginPage() {
  const router = useRouter();
  const { setKey } = useVaultKey();
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const isNew = !vaultExists();

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);

    try {
      const derivedKey = isNew
        ? await initVault(password)
        : await unlockVault(password);

      // Hold the CryptoKey in React context (memory only — never written to storage)
      setKey(derivedKey);
      router.push("/vault");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unlock failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main style={styles.main}>
      <div style={styles.card}>
        <h1 style={styles.heading}>🔐 MyPass</h1>
        <p style={styles.subtitle}>
          {isNew ? "Create your vault" : "Unlock your vault"}
        </p>

        <form onSubmit={handleSubmit} style={styles.form}>
          <input
            type="password"
            placeholder="Master password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            autoFocus
            style={styles.input}
          />
          <button type="submit" disabled={loading} style={styles.button}>
            {loading ? "Working…" : isNew ? "Create vault" : "Unlock"}
          </button>
        </form>

        {error && <p style={styles.error}>{error}</p>}
      </div>
    </main>
  );
}

const styles: Record<string, React.CSSProperties> = {
  main: {
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    minHeight: "100vh",
  },
  card: {
    background: "#fff",
    borderRadius: 12,
    padding: "2rem",
    width: "100%",
    maxWidth: 360,
    boxShadow: "0 4px 24px rgba(0,0,0,.08)",
  },
  heading: { fontSize: "1.6rem", marginBottom: "0.25rem" },
  subtitle: { color: "#666", fontSize: "0.9rem", marginBottom: "1.5rem" },
  form: { display: "flex", flexDirection: "column", gap: "0.75rem" },
  input: {
    padding: "0.6rem 0.75rem",
    border: "1px solid #ddd",
    borderRadius: 6,
    fontSize: "1rem",
  },
  button: {
    padding: "0.65rem",
    background: "#1a73e8",
    color: "#fff",
    border: "none",
    borderRadius: 6,
    fontSize: "1rem",
    cursor: "pointer",
  },
  error: { marginTop: "0.75rem", color: "#d32f2f", fontSize: "0.9rem" },
};
