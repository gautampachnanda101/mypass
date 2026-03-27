"use client";

import { useState } from "react";
import { Credential } from "@/types";
import PasswordGenerator from "./PasswordGenerator";

interface CredentialFormProps {
  initial?: Partial<Credential>;
  onSave: (data: Omit<Credential, "id" | "createdAt" | "updatedAt">) => void;
  onCancel: () => void;
}

export default function CredentialForm({
  initial,
  onSave,
  onCancel,
}: CredentialFormProps) {
  const [name, setName] = useState(initial?.name ?? "");
  const [username, setUsername] = useState(initial?.username ?? "");
  const [password, setPassword] = useState(initial?.password ?? "");
  const [url, setUrl] = useState(initial?.url ?? "");
  const [notes, setNotes] = useState(initial?.notes ?? "");
  const [showPassword, setShowPassword] = useState(false);
  const [showGenerator, setShowGenerator] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const validate = () => {
    const e: Record<string, string> = {};
    if (!name.trim()) e.name = "Name is required.";
    if (!username.trim()) e.username = "Username is required.";
    if (!password.trim()) e.password = "Password is required.";
    setErrors(e);
    return Object.keys(e).length === 0;
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    onSave({ name: name.trim(), username: username.trim(), password, url: url.trim(), notes: notes.trim() });
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Name / Site <span className="text-red-500">*</span>
        </label>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. GitHub, Gmail"
          className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900"
        />
        {errors.name && <p className="text-red-500 text-xs mt-1">{errors.name}</p>}
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Username / Email <span className="text-red-500">*</span>
        </label>
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="e.g. user@example.com"
          className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900"
        />
        {errors.username && <p className="text-red-500 text-xs mt-1">{errors.username}</p>}
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Password <span className="text-red-500">*</span>
        </label>
        <div className="flex gap-2">
          <div className="relative flex-1">
            <input
              type={showPassword ? "text" : "password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter or generate a password"
              className="w-full px-3 py-2 pr-10 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 font-mono"
            />
            <button
              type="button"
              onClick={() => setShowPassword((v) => !v)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600"
            >
              {showPassword ? (
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
              ) : (
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              )}
            </button>
          </div>
          <button
            type="button"
            onClick={() => setShowGenerator((v) => !v)}
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 transition-colors text-gray-700"
          >
            Generate
          </button>
        </div>
        {errors.password && <p className="text-red-500 text-xs mt-1">{errors.password}</p>}
      </div>

      {showGenerator && (
        <div className="border border-gray-200 rounded-lg p-4 bg-gray-50">
          <p className="text-sm font-medium text-gray-700 mb-3">Password Generator</p>
          <PasswordGenerator
            onSelect={(pw) => {
              setPassword(pw);
              setShowGenerator(false);
            }}
          />
        </div>
      )}

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          URL (optional)
        </label>
        <input
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://example.com"
          className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900"
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">
          Notes (optional)
        </label>
        <textarea
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          placeholder="Any additional notes..."
          rows={3}
          className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 text-gray-900 resize-none"
        />
      </div>

      <div className="flex gap-3 pt-2">
        <button
          type="button"
          onClick={onCancel}
          className="flex-1 px-4 py-2 border border-gray-300 rounded-lg text-gray-700 text-sm font-medium hover:bg-gray-50 transition-colors"
        >
          Cancel
        </button>
        <button
          type="submit"
          className="flex-1 px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
        >
          {initial?.id ? "Save Changes" : "Add Credential"}
        </button>
      </div>
    </form>
  );
}
