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
 * Set a value on an input and dispatch native `input` + `change` events so
 * that React, Vue, Angular and other SPA frameworks register the change.
 * @param {HTMLInputElement} el
 * @param {string} value
 */
function setNativeValue(el, value) {
  const nativeInputValueSetter = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    "value"
  )?.set;
  if (nativeInputValueSetter) {
    nativeInputValueSetter.call(el, value);
  } else {
    el.value = value;
  }
  el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent(new Event("change", { bubbles: true }));
}

/**
 * Fill username and password fields on the current page.
 * @param {string} username
 * @param {string} password
 */
function autofill(username, password) {
  const userField = document.querySelector(USERNAME_SELECTORS);
  const passField = document.querySelector("input[type='password']");

  if (userField) setNativeValue(userField, username);
  if (passField) setNativeValue(passField, password);
}

// Listen for autofill messages from the background service worker
chrome.runtime.onMessage.addListener((message) => {
  if (message.type === "autofill") {
    autofill(message.username, message.password);
  }
});
