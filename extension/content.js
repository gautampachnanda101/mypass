// content.js — injected into web pages to handle autofill.

/**
 * Selectors for locating username / email fields.
 * Ordered from most to least specific.
 */
const USERNAME_SELECTORS = [
  "input[autocomplete='username']",
  "input[autocomplete='email']",
  "input[type='email']",
  "input[name*='user' i]",
  "input[name*='email' i]",
  "input[id*='user' i]",
  "input[id*='email' i]",
].join(", ");

/**
 * Fill username and password fields on the current page.
 * @param {string} username
 * @param {string} password
 */
function autofill(username, password) {
  const userField = document.querySelector(USERNAME_SELECTORS);
  const passField = document.querySelector("input[type='password']");

  if (userField) userField.value = username;
  if (passField) passField.value = password;
}

// Listen for autofill messages from the background service worker
chrome.runtime.onMessage.addListener((message) => {
  if (message.type === "autofill") {
    autofill(message.username, message.password);
  }
});
