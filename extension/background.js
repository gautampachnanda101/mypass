// background.js — service worker for My Vault extension

chrome.runtime.onInstalled.addListener(() => {
  console.log("My Vault extension installed.");
});
