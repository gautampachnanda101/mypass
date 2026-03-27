"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  listItems,
  addItem,
  deleteItem,
  getPassword,
} from "@/lib/vault";
import type { VaultItem } from "@/types";
import { useVaultKey } from "../components/VaultProvider";
import ImportPasswords from "../components/ImportPasswords";

export default function VaultPage() {
  const router = useRouter();
  const { key, setKey } = useVaultKey();
  const [items, setItems] = useState<VaultItem[]>([]);
  const [visiblePasswords, setVisiblePasswords] = useState<
    Record<string, string>
  >({});
  const [revealError, setRevealError] = useState("");
  const [deleteConfirm, setDeleteConfirm] = useState<string | null>(null);

  // New-item form state
  const [form, setForm] = useState({
    name: "",
    username: "",
    password: "",
    url: "",
  });
  const [formError, setFormError] = useState("");
  const [adding, setAdding] = useState(false);

  // ---------------------------------------------------------------------------
  // On mount: redirect to login if the vault key is not in context
  // ---------------------------------------------------------------------------
  useEffect(() => {
    if (!key) {
      // Key lost (e.g. page refresh) — redirect to login to re-derive it
      router.replace("/");
      return;
    }
    setItems(listItems());
  }, [key, router]);

  // ---------------------------------------------------------------------------
  // Add item
  // ---------------------------------------------------------------------------
  const handleAdd = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      setFormError("");
      if (!key) {
        setFormError("Vault is locked — please unlock first.");
        return;
      }
      setAdding(true);
      try {
        await addItem(key, { name: form.name, username: form.username, url: form.url }, form.password);
        setItems(listItems());
        setForm({ name: "", username: "", password: "", url: "" });
      } catch (err) {
        setFormError(err instanceof Error ? err.message : "Add failed.");
      } finally {
        setAdding(false);
      }
    },
    [key, form]
  );

  // ---------------------------------------------------------------------------
  // Reveal / hide password
  // ---------------------------------------------------------------------------
  const handleReveal = useCallback(
    async (item: VaultItem) => {
      setRevealError("");
      if (!key) return;
      if (visiblePasswords[item.id]) {
        setVisiblePasswords((p) => {
          const next = { ...p };
          delete next[item.id];
          return next;
        });
        return;
      }
      try {
        const plain = await getPassword(key, item.id);
        setVisiblePasswords((p) => ({ ...p, [item.id]: plain }));
      } catch {
        setRevealError(`Could not decrypt "${item.name}" — wrong master password?`);
      }
    },
    [key, visiblePasswords]
  );

  // ---------------------------------------------------------------------------
  // Delete item (inline confirmation)
  // ---------------------------------------------------------------------------
  const handleDeleteRequest = useCallback((id: string) => {
    setDeleteConfirm(id);
  }, []);

  const handleDeleteConfirm = useCallback(() => {
    if (!deleteConfirm) return;
    deleteItem(deleteConfirm);
    setItems(listItems());
    setDeleteConfirm(null);
  }, [deleteConfirm]);

  // ---------------------------------------------------------------------------
  // Lock
  // ---------------------------------------------------------------------------
  function handleLock() {
    setKey(null);
    router.push("/");
  }

  // ---------------------------------------------------------------------------
  // Render
  // ---------------------------------------------------------------------------
  return (
    <main style={styles.main}>
      <header style={styles.header}>
        <h1 style={styles.heading}>🔐 MyPass</h1>
        <button onClick={handleLock} style={styles.lockBtn}>
          Lock vault
        </button>
      </header>

      {/* Add item form */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>Add item</h2>
        <form onSubmit={handleAdd} style={styles.addForm}>
          <input
            placeholder="Name (e.g. GitHub)"
            value={form.name}
            onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
            required
            style={styles.input}
          />
          <input
            placeholder="Username / email"
            value={form.username}
            onChange={(e) =>
              setForm((f) => ({ ...f, username: e.target.value }))
            }
            required
            style={styles.input}
          />
          <input
            type="password"
            placeholder="Password"
            value={form.password}
            onChange={(e) =>
              setForm((f) => ({ ...f, password: e.target.value }))
            }
            required
            style={styles.input}
          />
          <input
            placeholder="URL (optional)"
            value={form.url}
            onChange={(e) => setForm((f) => ({ ...f, url: e.target.value }))}
            style={styles.input}
          />
          <button type="submit" disabled={adding} style={styles.addBtn}>
            {adding ? "Saving…" : "Add"}
          </button>
          {formError && <p style={styles.error}>{formError}</p>}
        </form>
      </section>

      {/* Import from other password managers */}
      {key && (
        <section style={styles.section}>
          <h2 style={styles.sectionTitle}>Import passwords</h2>
          <ImportPasswords
            cryptoKey={key}
            onImportDone={() => setItems(listItems())}
          />
        </section>
      )}

      {/* Reveal error banner */}
      {revealError && (
        <p style={{ ...styles.error, marginBottom: "0.75rem" }}>{revealError}</p>
      )}

      {/* Inline delete confirmation */}
      {deleteConfirm && (
        <div style={styles.confirmBanner}>
          <span>Delete this item? This cannot be undone.</span>
          <button onClick={handleDeleteConfirm} style={styles.confirmYes}>
            Delete
          </button>
          <button onClick={() => setDeleteConfirm(null)} style={styles.confirmNo}>
            Cancel
          </button>
        </div>
      )}

      {/* Item list */}
      <section style={styles.section}>
        <h2 style={styles.sectionTitle}>
          Vault items ({items.length})
        </h2>

        {items.length === 0 && (
          <p style={{ color: "#888", fontSize: "0.9rem" }}>
            No items yet. Add one above.
          </p>
        )}

        <ul style={styles.list}>
          {items.map((item) => (
            <li key={item.id} style={styles.listItem}>
              <div style={styles.itemMeta}>
                <strong>{item.name}</strong>
                <span style={styles.username}>{item.username}</span>
                {item.url && (
                  <a href={item.url} target="_blank" rel="noopener noreferrer" style={styles.link}>
                    {item.url}
                  </a>
                )}
              </div>

              <div style={styles.itemActions}>
                {visiblePasswords[item.id] && (
                  <code style={styles.passwordBadge}>
                    {visiblePasswords[item.id]}
                  </code>
                )}
                <button
                  onClick={() => handleReveal(item)}
                  style={styles.actionBtn}
                >
                  {visiblePasswords[item.id] ? "Hide" : "Reveal"}
                </button>
                <button
                  onClick={() => handleDeleteRequest(item.id)}
                  style={{ ...styles.actionBtn, color: "#d32f2f" }}
                >
                  Delete
                </button>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}

const styles: Record<string, React.CSSProperties> = {
  main: { maxWidth: 720, margin: "0 auto", padding: "1.5rem 1rem" },
  header: {
    display: "flex",
    alignItems: "center",
    justifyContent: "space-between",
    marginBottom: "1.5rem",
  },
  heading: { fontSize: "1.4rem" },
  lockBtn: {
    padding: "0.4rem 0.9rem",
    background: "#f44336",
    color: "#fff",
    border: "none",
    borderRadius: 6,
    cursor: "pointer",
    fontSize: "0.85rem",
  },
  section: {
    background: "#fff",
    borderRadius: 10,
    padding: "1.25rem",
    marginBottom: "1rem",
    boxShadow: "0 2px 8px rgba(0,0,0,.06)",
  },
  sectionTitle: {
    fontSize: "1rem",
    marginBottom: "0.75rem",
    color: "#333",
  },
  addForm: { display: "flex", flexDirection: "column", gap: "0.5rem" },
  input: {
    padding: "0.5rem 0.75rem",
    border: "1px solid #ddd",
    borderRadius: 6,
    fontSize: "0.95rem",
  },
  addBtn: {
    padding: "0.55rem",
    background: "#1a73e8",
    color: "#fff",
    border: "none",
    borderRadius: 6,
    cursor: "pointer",
    alignSelf: "flex-start",
    paddingLeft: "1.25rem",
    paddingRight: "1.25rem",
  },
  error: { color: "#d32f2f", fontSize: "0.85rem" },
  confirmBanner: {
    display: "flex",
    alignItems: "center",
    gap: "0.75rem",
    background: "#fff3e0",
    border: "1px solid #ffcc02",
    borderRadius: 8,
    padding: "0.75rem 1rem",
    marginBottom: "0.75rem",
    fontSize: "0.9rem",
  },
  confirmYes: {
    padding: "0.3rem 0.8rem",
    background: "#d32f2f",
    color: "#fff",
    border: "none",
    borderRadius: 5,
    cursor: "pointer",
    fontSize: "0.85rem",
  },
  confirmNo: {
    padding: "0.3rem 0.8rem",
    background: "transparent",
    border: "1px solid #aaa",
    borderRadius: 5,
    cursor: "pointer",
    fontSize: "0.85rem",
  },
  list: { listStyle: "none", display: "flex", flexDirection: "column", gap: "0.5rem" },
  listItem: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "flex-start",
    padding: "0.75rem",
    border: "1px solid #eee",
    borderRadius: 8,
    gap: "0.5rem",
  },
  itemMeta: { display: "flex", flexDirection: "column", gap: "0.15rem" },
  username: { fontSize: "0.85rem", color: "#666" },
  link: { fontSize: "0.8rem", color: "#1a73e8" },
  itemActions: {
    display: "flex",
    alignItems: "center",
    gap: "0.5rem",
    flexShrink: 0,
  },
  passwordBadge: {
    background: "#f0f4ff",
    padding: "0.2rem 0.5rem",
    borderRadius: 4,
    fontSize: "0.85rem",
  },
  actionBtn: {
    padding: "0.3rem 0.6rem",
    background: "transparent",
    border: "1px solid #ddd",
    borderRadius: 5,
    cursor: "pointer",
    fontSize: "0.82rem",
  },
};
