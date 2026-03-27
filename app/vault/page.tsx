"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import { Credential } from "@/types";
import {
  loadVault,
  saveVault,
  createCredential,
  updateCredential,
  deleteCredential,
  clearVault,
  getSessionKey,
  clearSessionKey,
} from "@/lib/vault";
import CredentialCard from "@/app/components/CredentialCard";
import CredentialForm from "@/app/components/CredentialForm";

type Modal = { type: "add" } | { type: "edit"; credential: Credential } | { type: "delete"; credential: Credential } | { type: "generator" } | null;

export default function VaultPage() {
  const router = useRouter();
  const [credentials, setCredentials] = useState<Credential[]>([]);
  const [search, setSearch] = useState("");
  const [modal, setModal] = useState<Modal>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const getSessionKeyOrRedirect = useCallback((): string | null => {
    if (typeof window === "undefined") return null;
    return getSessionKey();
  }, []);

  useEffect(() => {
    const sk = getSessionKeyOrRedirect();
    if (!sk) {
      router.replace("/");
      return;
    }
    loadVault(sk)
      .then((vault) => {
        setCredentials(vault.credentials);
        setLoading(false);
      })
      .catch(() => {
        setError("Failed to load vault.");
        setLoading(false);
      });
  }, [getSessionKeyOrRedirect, router]);

  const persist = useCallback(
    async (updated: Credential[]) => {
      const sk = getSessionKey();
      if (!sk) {
        router.replace("/");
        return;
      }
      setSaving(true);
      try {
        await saveVault({ credentials: updated }, sk);
      } catch {
        setError("Failed to save vault.");
      } finally {
        setSaving(false);
      }
    },
    [router]
  );

  const handleAdd = useCallback(
    async (data: Omit<Credential, "id" | "createdAt" | "updatedAt">) => {
      const newCred = createCredential(data);
      const updated = [...credentials, newCred];
      setCredentials(updated);
      setModal(null);
      await persist(updated);
    },
    [credentials, persist]
  );

  const handleEdit = useCallback(
    async (id: string, data: Omit<Credential, "id" | "createdAt" | "updatedAt">) => {
      const updated = updateCredential(credentials, id, data);
      setCredentials(updated);
      setModal(null);
      await persist(updated);
    },
    [credentials, persist]
  );

  const handleDelete = useCallback(
    async (id: string) => {
      const updated = deleteCredential(credentials, id);
      setCredentials(updated);
      setModal(null);
      await persist(updated);
    },
    [credentials, persist]
  );

  const handleLock = () => {
    clearSessionKey();
    router.push("/");
  };

  const handleResetVault = () => {
    if (window.confirm("Are you sure you want to delete all data? This cannot be undone.")) {
      clearVault();
      sessionStorage.removeItem("mp");
      router.push("/");
    }
  };

  const filtered = credentials.filter(
    (c) =>
      c.name.toLowerCase().includes(search.toLowerCase()) ||
      c.username.toLowerCase().includes(search.toLowerCase()) ||
      c.url?.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="min-h-screen bg-gray-50">
      <header className="bg-white border-b border-gray-200 sticky top-0 z-10">
        <div className="max-w-4xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-blue-600 rounded-lg flex items-center justify-center">
              <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
            <h1 className="font-bold text-gray-900 text-lg">MyPass</h1>
            {saving && <span className="text-xs text-gray-400">Saving...</span>}
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setModal({ type: "add" })}
              className="flex items-center gap-1.5 bg-blue-600 text-white px-3 py-1.5 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
              </svg>
              Add
            </button>
            <button
              onClick={handleLock}
              className="flex items-center gap-1.5 border border-gray-300 text-gray-700 px-3 py-1.5 rounded-lg text-sm font-medium hover:bg-gray-50 transition-colors"
              title="Lock vault"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
              Lock
            </button>
          </div>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-6">
        {error && (
          <div className="mb-4 bg-red-50 border border-red-200 rounded-lg p-3 text-red-600 text-sm">
            {error}
          </div>
        )}

        <div className="relative mb-6">
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={`Search ${credentials.length} credential${credentials.length !== 1 ? "s" : ""}…`}
            className="w-full pl-9 pr-4 py-2.5 border border-gray-300 rounded-xl bg-white focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900"
          />
        </div>

        {loading ? (
          <div className="flex justify-center py-16">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
          </div>
        ) : filtered.length === 0 ? (
          <div className="text-center py-16">
            {credentials.length === 0 ? (
              <>
                <div className="inline-flex items-center justify-center w-16 h-16 bg-gray-100 rounded-2xl mb-4">
                  <svg className="w-8 h-8 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z" />
                  </svg>
                </div>
                <h3 className="text-gray-900 font-semibold mb-1">Your vault is empty</h3>
                <p className="text-gray-500 text-sm mb-4">Add your first credential to get started.</p>
                <button
                  onClick={() => setModal({ type: "add" })}
                  className="inline-flex items-center gap-2 bg-blue-600 text-white px-4 py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
                  </svg>
                  Add Credential
                </button>
              </>
            ) : (
              <>
                <p className="text-gray-500">No results for &quot;{search}&quot;</p>
                <button onClick={() => setSearch("")} className="text-blue-600 text-sm mt-2 hover:underline">
                  Clear search
                </button>
              </>
            )}
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {filtered.map((cred) => (
              <CredentialCard
                key={cred.id}
                credential={cred}
                onEdit={() => setModal({ type: "edit", credential: cred })}
                onDelete={() => setModal({ type: "delete", credential: cred })}
              />
            ))}
          </div>
        )}

        <div className="mt-8 text-center">
          <button
            onClick={handleResetVault}
            className="text-xs text-gray-400 hover:text-red-500 transition-colors"
          >
            Reset vault
          </button>
        </div>
      </main>

      {modal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 px-4">
          <div className="bg-white rounded-2xl shadow-2xl w-full max-w-md max-h-[90vh] overflow-y-auto">
            <div className="p-6">
              {(modal.type === "add" || modal.type === "edit") && (
                <>
                  <div className="flex items-center justify-between mb-4">
                    <h2 className="text-lg font-semibold text-gray-900">
                      {modal.type === "add" ? "Add Credential" : "Edit Credential"}
                    </h2>
                    <button onClick={() => setModal(null)} className="text-gray-400 hover:text-gray-600">
                      <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                      </svg>
                    </button>
                  </div>
                  <CredentialForm
                    initial={modal.type === "edit" ? modal.credential : undefined}
                    onSave={(data) => {
                      if (modal.type === "add") {
                        handleAdd(data);
                      } else {
                        handleEdit(modal.credential.id, data);
                      }
                    }}
                    onCancel={() => setModal(null)}
                  />
                </>
              )}

              {modal.type === "delete" && (
                <>
                  <h2 className="text-lg font-semibold text-gray-900 mb-2">Delete Credential</h2>
                  <p className="text-gray-500 text-sm mb-4">
                    Are you sure you want to delete <strong>{modal.credential.name}</strong>? This action cannot be undone.
                  </p>
                  <div className="flex gap-3">
                    <button
                      onClick={() => setModal(null)}
                      className="flex-1 px-4 py-2 border border-gray-300 rounded-lg text-gray-700 text-sm font-medium hover:bg-gray-50 transition-colors"
                    >
                      Cancel
                    </button>
                    <button
                      onClick={() => handleDelete(modal.credential.id)}
                      className="flex-1 px-4 py-2 bg-red-600 text-white rounded-lg text-sm font-medium hover:bg-red-700 transition-colors"
                    >
                      Delete
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
