"use client";

/**
 * ImportPasswords — CSV import from common password managers.
 *
 * Auto-detects format by inspecting CSV headers:
 *   Google Password Manager : name, url, username, password
 *   Samsung Pass            : title, url, id, password
 *   McAfee True Key         : kind, name, url, username, password
 *   LastPass                : url, username, password, name  (plus extra cols)
 *   1Password               : Title, Url, Username, Password  (case-insensitive)
 *   Bitwarden               : login_uri, login_username, login_password, name
 *   Generic fallback        : any CSV with recognisable field names
 */

import { useCallback, useRef, useState } from "react";
import { addItem } from "@/lib/vault";

interface ParsedRow {
  name: string;
  username: string;
  password: string;
  url?: string;
}

type ImportStatus = "idle" | "parsing" | "importing" | "done" | "error";

interface ImportResult {
  imported: number;
  skipped: number;
  errors: string[];
}

// ---------------------------------------------------------------------------
// CSV parser
// ---------------------------------------------------------------------------

/** Split a single CSV line respecting double-quoted fields. */
function splitCsvLine(line: string): string[] {
  const fields: string[] = [];
  let current = "";
  let inQuotes = false;

  for (let i = 0; i < line.length; i++) {
    const ch = line[i];
    if (ch === '"') {
      if (inQuotes && line[i + 1] === '"') {
        current += '"';
        i++;
      } else {
        inQuotes = !inQuotes;
      }
    } else if (ch === "," && !inQuotes) {
      fields.push(current.trim());
      current = "";
    } else {
      current += ch;
    }
  }
  fields.push(current.trim());
  return fields;
}

/** Detect which column index maps to each logical field. */
function detectColumns(headers: string[]): {
  name: number;
  username: number;
  password: number;
  url: number;
} | null {
  const h = headers.map((x) => x.toLowerCase().replace(/[^a-z0-9]/g, "_"));

  const find = (...candidates: string[]) => {
    for (const c of candidates) {
      const idx = h.indexOf(c);
      if (idx !== -1) return idx;
    }
    // partial match fallback
    for (const c of candidates) {
      const idx = h.findIndex((x) => x.includes(c));
      if (idx !== -1) return idx;
    }
    return -1;
  };

  const name = find("name", "title", "account_name", "site_name");
  const username = find(
    "username",
    "login_username",
    "id",
    "login_id",
    "email",
    "user"
  );
  const password = find("password", "login_password", "pass");
  const url = find("url", "login_uri", "website", "web_site", "uri");

  if (name === -1 || username === -1 || password === -1) return null;
  return { name, username, password, url };
}

/** Parse CSV text into rows, skipping empty / header lines. */
function parseCsv(text: string): ParsedRow[] {
  const lines = text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);

  if (lines.length < 2) return [];

  const headers = splitCsvLine(lines[0]);
  const cols = detectColumns(headers);
  if (!cols) {
    throw new Error(
      "Unrecognised CSV format. Expected columns: name/title, username/email, password, url."
    );
  }

  const rows: ParsedRow[] = [];
  for (let i = 1; i < lines.length; i++) {
    const cells = splitCsvLine(lines[i]);
    const name = cells[cols.name] ?? "";
    const username = cells[cols.username] ?? "";
    const password = cells[cols.password] ?? "";
    const url = cols.url >= 0 ? (cells[cols.url] ?? "") : "";

    // Skip clearly empty rows (e.g. McAfee section headers like "kind=note")
    if (!name && !username && !password) continue;
    // Skip McAfee non-login rows (kind column exists and value is not "login")
    const kindIdx = headers
      .map((x) => x.toLowerCase())
      .indexOf("kind");
    if (kindIdx >= 0) {
      const kind = (cells[kindIdx] ?? "").toLowerCase();
      if (kind && kind !== "login") continue;
    }

    rows.push({ name, username, password, url: url || undefined });
  }
  return rows;
}

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

export interface ImportPasswordsProps {
  cryptoKey: CryptoKey;
  onImportDone: () => void;
}

export default function ImportPasswords({
  cryptoKey,
  onImportDone,
}: ImportPasswordsProps) {
  const fileRef = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<ImportStatus>("idle");
  const [result, setResult] = useState<ImportResult | null>(null);
  const [parseError, setParseError] = useState("");

  const handleFile = useCallback(
    async (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;

      setParseError("");
      setResult(null);
      setStatus("parsing");

      let rows: ParsedRow[];
      try {
        const text = await file.text();
        rows = parseCsv(text);
      } catch (err) {
        setParseError(err instanceof Error ? err.message : "Parse failed.");
        setStatus("error");
        return;
      }

      if (rows.length === 0) {
        setParseError("No importable rows found in the file.");
        setStatus("error");
        return;
      }

      setStatus("importing");
      let imported = 0;
      let skipped = 0;
      const errors: string[] = [];

      for (const row of rows) {
        try {
          await addItem(
            cryptoKey,
            { name: row.name || "Unnamed", username: row.username, url: row.url },
            row.password
          );
          imported++;
        } catch (err) {
          skipped++;
          errors.push(
            `${row.name || row.url}: ${err instanceof Error ? err.message : "unknown error"}`
          );
        }
      }

      setResult({ imported, skipped, errors });
      setStatus("done");

      // Reset file input so the same file can be re-imported if needed
      if (fileRef.current) fileRef.current.value = "";

      onImportDone();
    },
    [cryptoKey, onImportDone]
  );

  return (
    <div style={styles.wrapper}>
      <p style={styles.hint}>
        Supports CSV exports from{" "}
        <strong>Google, Samsung, McAfee True Key, LastPass, 1Password, Bitwarden</strong>
        {" "}and any CSV with <em>name/title</em>, <em>username/email</em>, <em>password</em> columns.
      </p>

      <label style={styles.fileLabel}>
        <span style={styles.fileLabelText}>
          {status === "importing" ? "Importing…" : "Choose CSV file"}
        </span>
        <input
          ref={fileRef}
          type="file"
          accept=".csv,text/csv"
          onChange={handleFile}
          disabled={status === "importing" || status === "parsing"}
          style={{ display: "none" }}
        />
      </label>

      {parseError && <p style={styles.error}>{parseError}</p>}

      {status === "done" && result && (
        <div style={styles.resultBox}>
          <p style={styles.resultOk}>
            ✓ Imported <strong>{result.imported}</strong> item
            {result.imported !== 1 ? "s" : ""}.
            {result.skipped > 0 && ` ${result.skipped} skipped.`}
          </p>
          {result.errors.length > 0 && (
            <ul style={styles.errorList}>
              {result.errors.map((e, i) => (
                <li key={i} style={styles.errorItem}>
                  {e}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  wrapper: { display: "flex", flexDirection: "column", gap: "0.6rem" },
  hint: { fontSize: "0.85rem", color: "#555", lineHeight: 1.5 },
  fileLabel: {
    display: "inline-block",
    cursor: "pointer",
    alignSelf: "flex-start",
  },
  fileLabelText: {
    display: "inline-block",
    padding: "0.5rem 1.1rem",
    background: "#1a73e8",
    color: "#fff",
    borderRadius: 6,
    fontSize: "0.9rem",
    cursor: "pointer",
  },
  error: { color: "#d32f2f", fontSize: "0.85rem" },
  resultBox: { fontSize: "0.85rem" },
  resultOk: { color: "#2e7d32", marginBottom: "0.3rem" },
  errorList: { paddingLeft: "1.25rem", margin: 0 },
  errorItem: { color: "#b71c1c", marginTop: "0.2rem" },
};
