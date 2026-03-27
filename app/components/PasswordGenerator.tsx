"use client";

import { useState, useCallback } from "react";
import { generatePassword } from "@/lib/crypto";

interface PasswordGeneratorProps {
  onSelect?: (password: string) => void;
}

export default function PasswordGenerator({ onSelect }: PasswordGeneratorProps) {
  const [length, setLength] = useState(20);
  const [useUppercase, setUseUppercase] = useState(true);
  const [useLowercase, setUseLowercase] = useState(true);
  const [useNumbers, setUseNumbers] = useState(true);
  const [useSymbols, setUseSymbols] = useState(true);
  const [generated, setGenerated] = useState("");
  const [copied, setCopied] = useState(false);

  const handleGenerate = useCallback(() => {
    const pw = generatePassword(length, useUppercase, useLowercase, useNumbers, useSymbols);
    setGenerated(pw);
    setCopied(false);
  }, [length, useUppercase, useLowercase, useNumbers, useSymbols]);

  const handleCopy = async () => {
    if (!generated) return;
    await navigator.clipboard.writeText(generated);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const strengthScore = [useUppercase, useLowercase, useNumbers, useSymbols].filter(Boolean).length;
  const strengthLabel =
    strengthScore <= 1 ? "Weak" : strengthScore === 2 ? "Fair" : strengthScore === 3 ? "Good" : "Strong";
  const strengthColor =
    strengthScore <= 1 ? "bg-red-500" : strengthScore === 2 ? "bg-yellow-500" : strengthScore === 3 ? "bg-blue-500" : "bg-green-500";

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <input
          type="text"
          value={generated}
          readOnly
          placeholder="Click Generate to create a password"
          className="flex-1 px-3 py-2 border border-gray-300 rounded-lg bg-gray-50 font-mono text-sm text-gray-800 focus:outline-none"
        />
        <button
          type="button"
          onClick={handleCopy}
          disabled={!generated}
          className="px-3 py-2 border border-gray-300 rounded-lg text-sm hover:bg-gray-50 disabled:opacity-40 transition-colors"
          title="Copy"
        >
          {copied ? "✓" : "Copy"}
        </button>
        {onSelect && (
          <button
            type="button"
            onClick={() => generated && onSelect(generated)}
            disabled={!generated}
            className="px-3 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-700 disabled:opacity-40 transition-colors"
          >
            Use
          </button>
        )}
      </div>

      <div>
        <div className="flex justify-between text-sm text-gray-600 mb-1">
          <span>Length: {length}</span>
          <span className={`font-medium ${strengthScore <= 1 ? "text-red-500" : strengthScore === 2 ? "text-yellow-500" : strengthScore === 3 ? "text-blue-500" : "text-green-500"}`}>
            {strengthLabel}
          </span>
        </div>
        <input
          type="range"
          min={8}
          max={64}
          value={length}
          onChange={(e) => setLength(Number(e.target.value))}
          className="w-full accent-blue-600"
        />
        <div className="flex gap-1 mt-2">
          {[...Array(4)].map((_, i) => (
            <div key={i} className={`h-1 flex-1 rounded ${i < strengthScore ? strengthColor : "bg-gray-200"}`} />
          ))}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm">
        {[
          { label: "Uppercase (A–Z)", value: useUppercase, set: setUseUppercase },
          { label: "Lowercase (a–z)", value: useLowercase, set: setUseLowercase },
          { label: "Numbers (0–9)", value: useNumbers, set: setUseNumbers },
          { label: "Symbols (!@#…)", value: useSymbols, set: setUseSymbols },
        ].map(({ label, value, set }) => (
          <label key={label} className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={value}
              onChange={(e) => set(e.target.checked)}
              className="rounded accent-blue-600"
            />
            <span className="text-gray-700">{label}</span>
          </label>
        ))}
      </div>

      <button
        type="button"
        onClick={handleGenerate}
        className="w-full bg-gray-900 text-white py-2 rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
      >
        Generate Password
      </button>
    </div>
  );
}
