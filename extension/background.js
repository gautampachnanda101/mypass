// background.js — service worker for My Vault extension
// Routes messages between popup and content scripts.

chrome.runtime.onInstalled.addListener(() => {
  console.log("My Vault extension installed.");
});

// Forward autofill requests from the popup to the active tab's content script.
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message.type === "autofill") {
    const { tabId, username, password } = message;
    chrome.tabs.sendMessage(tabId, { type: "autofill", username, password });
    sendResponse({ success: true });
  }
  return true; // keep message channel open for async response
});
