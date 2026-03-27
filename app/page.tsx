"use client";

import { useState, useEffect, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  isVaultSetup,
  setupMasterPassword,
  verifyMasterPassword,
  setSessionKey,
} from "@/lib/vault";

export default function Home() {
  const router = useRouter();
  const [isSetup, setIsSetup] = useState<boolean | null>(null);
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setIsSetup(isVaultSetup());
  }, []);

  const handleSetup = useCallback(async () => {
    if (password.length < 8) {
      setError("Master password must be at least 8 characters.");
      return;
    }
    if (password !== confirmPassword) {
      setError("Passwords do not match.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const sessionKey = await setupMasterPassword(password);
      setSessionKey(sessionKey);
      router.push("/vault");
    } catch {
      setError("Failed to set up vault. Please try again.");
    } finally {
      setLoading(false);
    }
  }, [password, confirmPassword, router]);

  const handleLogin = useCallback(async () => {
    if (!password) {
      setError("Please enter your master password.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const sessionKey = await verifyMasterPassword(password);
      if (sessionKey) {
        setSessionKey(sessionKey);
        router.push("/vault");
      } else {
        setError("Incorrect master password.");
      }
    } catch {
      setError("Failed to unlock vault. Please try again.");
    } finally {
      setLoading(false);
    }
  }, [password, router]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      if (isSetup) {
        handleLogin();
      } else {
        handleSetup();
      }
    }
  };

  if (isSetup === null) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600" />
      </div>
    );
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-blue-50 to-indigo-100 px-4">
      <div className="bg-white rounded-2xl shadow-xl p-8 w-full max-w-md">
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-blue-600 rounded-2xl mb-4">
            <svg
              className="w-8 h-8 text-white"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
              />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-gray-900">MyPass</h1>
          <p className="text-gray-500 mt-1 text-sm">
            {isSetup
              ? "Enter your master password to unlock your vault"
              : "Create a master password to protect your vault"}
          </p>
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Master Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Enter master password"
              autoFocus
              className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent text-gray-900"
            />
          </div>

          {!isSetup && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Confirm Password
              </label>
              <input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Confirm master password"
                className="w-full px-4 py-2.5 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent text-gray-900"
              />
            </div>
          )}

          {error && (
            <div className="bg-red-50 border border-red-200 rounded-lg p-3">
              <p className="text-red-600 text-sm">{error}</p>
            </div>
          )}

          <button
            onClick={isSetup ? handleLogin : handleSetup}
            disabled={loading}
            className="w-full bg-blue-600 text-white py-2.5 px-4 rounded-lg font-medium hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <svg className="animate-spin h-4 w-4" viewBox="0 0 24 24" fill="none">
                  <circle
                    className="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    strokeWidth="4"
                  />
                  <path
                    className="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                  />
                </svg>
                {isSetup ? "Unlocking..." : "Setting up..."}
              </span>
            ) : isSetup ? (
              "Unlock Vault"
            ) : (
              "Create Vault"
            )}
          </button>
        </div>

        <p className="text-center text-xs text-gray-400 mt-6">
          Your vault is encrypted with AES-256 and stored locally.
          <br />
          We never have access to your passwords.
        </p>
      </div>
    </div>
  );
}

