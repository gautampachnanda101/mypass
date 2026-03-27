"use client";

import { useState } from "react";
import { Credential } from "@/types";

interface CredentialCardProps {
  credential: Credential;
  onEdit: () => void;
  onDelete: () => void;
}

export default function CredentialCard({
  credential,
  onEdit,
  onDelete,
}: CredentialCardProps) {
  const [showPassword, setShowPassword] = useState(false);
  const [copiedField, setCopiedField] = useState<string | null>(null);

  const copyToClipboard = async (text: string, field: string) => {
    await navigator.clipboard.writeText(text);
    setCopiedField(field);
    setTimeout(() => setCopiedField(null), 2000);
  };

  const getInitial = (name: string) =>
    name.charAt(0).toUpperCase();

  const colors = [
    "bg-blue-500", "bg-purple-500", "bg-green-500", "bg-rose-500",
    "bg-orange-500", "bg-teal-500", "bg-indigo-500", "bg-pink-500",
  ];
  const colorIndex =
    credential.name.charCodeAt(0) % colors.length;

  return (
    <div className="bg-white border border-gray-200 rounded-xl p-4 hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between gap-3">
        <div className="flex items-center gap-3 min-w-0">
          <div
            className={`flex-shrink-0 w-10 h-10 ${colors[colorIndex]} rounded-lg flex items-center justify-center text-white font-semibold text-sm`}
          >
            {getInitial(credential.name)}
          </div>
          <div className="min-w-0">
            <h3 className="font-semibold text-gray-900 truncate">{credential.name}</h3>
            {credential.url && (
              <a
                href={credential.url}
                target="_blank"
                rel="noopener noreferrer"
                className="text-xs text-blue-500 hover:underline truncate block"
              >
                {credential.url}
              </a>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1 flex-shrink-0">
          <button
            onClick={onEdit}
            className="p-1.5 text-gray-400 hover:text-blue-600 hover:bg-blue-50 rounded-lg transition-colors"
            title="Edit"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
          </button>
          <button
            onClick={onDelete}
            className="p-1.5 text-gray-400 hover:text-red-600 hover:bg-red-50 rounded-lg transition-colors"
            title="Delete"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          </button>
        </div>
      </div>

      <div className="mt-3 space-y-2">
        <div className="flex items-center justify-between bg-gray-50 rounded-lg px-3 py-2">
          <div className="min-w-0">
            <p className="text-xs text-gray-500">Username</p>
            <p className="text-sm text-gray-800 truncate">{credential.username}</p>
          </div>
          <button
            onClick={() => copyToClipboard(credential.username, "username")}
            className="ml-2 p-1 text-gray-400 hover:text-gray-600 flex-shrink-0"
            title="Copy username"
          >
            {copiedField === "username" ? (
              <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
            )}
          </button>
        </div>

        <div className="flex items-center justify-between bg-gray-50 rounded-lg px-3 py-2">
          <div className="min-w-0 flex-1">
            <p className="text-xs text-gray-500">Password</p>
            <p className="text-sm text-gray-800 font-mono truncate">
              {showPassword ? credential.password : "•".repeat(Math.min(credential.password.length, 16))}
            </p>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0 ml-2">
            <button
              onClick={() => setShowPassword((v) => !v)}
              className="p-1 text-gray-400 hover:text-gray-600"
              title={showPassword ? "Hide" : "Show"}
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
            <button
              onClick={() => copyToClipboard(credential.password, "password")}
              className="p-1 text-gray-400 hover:text-gray-600"
              title="Copy password"
            >
              {copiedField === "password" ? (
                <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              )}
            </button>
          </div>
        </div>

        {credential.notes && (
          <div className="bg-gray-50 rounded-lg px-3 py-2">
            <p className="text-xs text-gray-500">Notes</p>
            <p className="text-sm text-gray-700 whitespace-pre-wrap break-words">{credential.notes}</p>
          </div>
        )}
      </div>
    </div>
  );
}
