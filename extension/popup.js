// popup.js — handles unlock flow and vault item display.
// ⚠️  Crypto is a placeholder — replace with real Argon2 + AES implementation.

const API = "https://vault.local/api";

document.getElementById("unlock").addEventListener("click", async () => {
  const password = document.getElementById("password").value.trim();
  const statusEl = document.getElementById("status");
  const listEl = document.getElementById("vault-list");

  if (!password) {
    statusEl.textContent = "Please enter your master password.";
    return;
  }

  statusEl.textContent = "Unlocking…";

  try {
    // Read saved email from extension storage (set on first run / settings page)
    const { vaultEmail } = await chrome.storage.local.get("vaultEmail");
    const email = vaultEmail || "";

    if (!email) {
      statusEl.textContent =
        "No account email configured. Please set it in extension storage.";
      return;
    }

    // Step 1: fetch KDF parameters for the account
    const preloginRes = await fetch(`${API}/accounts/prelogin`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    });

    if (!preloginRes.ok) {
      throw new Error(`Prelogin failed: ${preloginRes.status}`);
    }

    const kdfInfo = await preloginRes.json();
    console.log("KDF info received:", kdfInfo);

    // TODO: derive master key using Argon2 / PBKDF2 (kdfInfo.kdfType)
    // TODO: authenticate against /api/accounts/token
    // TODO: decrypt vault with derived key (AES-256-CBC / AES-256-GCM)

    statusEl.textContent = "Vault unlocked! (demo — add real crypto)";

    // Demo: render placeholder items
    listEl.innerHTML = "";
    const demoItems = [
      { name: "GitHub", username: "user@example.com" },
      { name: "Gmail", username: "user@gmail.com" },
    ];

    demoItems.forEach((item) => {
      const li = document.createElement("li");
      li.textContent = `${item.name} — ${item.username}`;
      li.addEventListener("click", () => autofillItem(item.username, ""));
      listEl.appendChild(li);
    });
  } catch (err) {
    statusEl.textContent = `Error: ${err.message}`;
    console.error(err);
  }
});

async function autofillItem(username, password) {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) return;

  chrome.tabs.sendMessage(tab.id, { type: "autofill", username, password });
}
