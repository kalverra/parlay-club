// Formats <time datetime="..."> elements in the browser's local zone.
//
// Every page that shows a time renders it as a <time> element with a UTC ISO
// datetime, so the server never has to guess the reader's zone. This runs on
// first load and after every HTMX swap, which is what inserts new times.

const localTimeFormat = new Intl.DateTimeFormat(undefined, {
  weekday: "short",
  hour: "numeric",
  minute: "2-digit",
});

function formatTimeElement(element) {
  const iso = element.getAttribute("datetime");
  if (!iso) {
    return;
  }

  const when = new Date(iso);
  if (Number.isNaN(when.getTime())) {
    return;
  }

  element.textContent = localTimeFormat.format(when);
}

function formatTimes(root) {
  if (!root || !root.querySelectorAll) {
    return;
  }

  for (const element of root.querySelectorAll("time[datetime]")) {
    formatTimeElement(element);
  }
}

// The pick slate scrolls on its own, and replacing #pick-body would throw it
// back to the top on every game switch. Save the pane marked for it before an
// HTMX swap and put the position back after.
let keptScrollTop = null;

document.addEventListener("htmx:beforeSwap", (event) => {
  const pane = document.querySelector("[data-keep-scroll]");
  keptScrollTop = pane ? pane.scrollTop : null;

  if (formForEvent(event)?.id === "bug-report-form" && event.detail?.xhr?.status === 400) {
    const errorSlot = document.getElementById("bug-report-error");
    if (errorSlot) {
      event.detail.shouldSwap = true;
      event.detail.target = errorSlot;
      event.detail.isError = false;
    }
  }
});

// HTMX swaps in partials, so format whatever arrived and whatever was there
// before the swap.
document.addEventListener("htmx:afterSwap", (event) => {
  const target = (event.detail && event.detail.target) || document;
  formatTimes(target);

  const pane = document.querySelector("[data-keep-scroll]");
  if (pane && keptScrollTop !== null) {
    pane.scrollTop = keptScrollTop;
  }
  keptScrollTop = null;

  if (target && target.id === "leg-sheet-body") {
    openLegSheet();
  }
  if (target && target.id === "pick-swap-body") {
    openSwapSheet();
  }
  if (target && target.id === "pick-tray-body") {
    openDockSheet();
  }
});

document.addEventListener("htmx:oobAfterSwap", (event) => {
  const target = (event.detail && event.detail.target) || document;
  formatTimes(target);
});

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => formatTimes(document));
} else {
  formatTimes(document);
}

// Image load errors do not bubble, so listen in capture phase to hide any
// failed team logo or player portrait and reveal its sibling fallback badge.
document.addEventListener(
  "error",
  (event) => {
    const img = event.target;
    if (!(img instanceof HTMLImageElement) || !img.matches("img[data-fallback-img]")) {
      return;
    }
    img.classList.add("hidden");
    const wrapper = img.closest("[data-media]");
    if (!wrapper) {
      return;
    }
    const badge = wrapper.querySelector("[data-fallback-badge]");
    if (badge) {
      badge.classList.remove("hidden");
      badge.classList.add("flex");
    }
  },
  true,
);

// Copies text with the clipboard API, falling back to a hidden textarea for
// browsers that refuse it.
async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.className = "sr-only";
    document.body.append(area);
    area.select();
    let copied = false;
    try {
      copied = document.execCommand("copy");
    } catch {
      copied = false;
    }
    area.remove();
    return copied;
  }
}

// Copies an invite link from its read-only input, then briefly says so.
async function copyInvite(button) {
  const source = document.querySelector(button.getAttribute("data-copy-target"));
  if (!source) {
    return;
  }

  if (!(await copyText(source.value))) {
    return;
  }

  sayCopied(button);
}

// Flips a button's label to "Copied" for a moment, then puts it back.
function sayCopied(button, label = "Copied") {
  const original = button.textContent;
  button.textContent = label;
  setTimeout(() => {
    button.textContent = original;
  }, 1500);
}

// A nudge shares a short message with the person the slip waits on. The message
// reads in the sender's zone and carries an absolute link, so the server sends
// only its parts.
function nudgeMessage(button) {
  const slip = button.getAttribute("data-nudge-slip") || "our slip";
  let message = `Waiting on you for ${slip}.`;

  const close = button.getAttribute("data-nudge-close");
  if (close) {
    const when = new Date(close);
    if (!Number.isNaN(when.getTime())) {
      message += ` Picks close ${localTimeFormat.format(when)}.`;
    }
  }

  const path = button.getAttribute("data-nudge-path") || "";
  return `${message} ${new URL(path, window.location.origin).href}`;
}

// Opens the share sheet when the browser has one, otherwise copies the nudge.
async function shareNudge(button) {
  const message = nudgeMessage(button);

  if (navigator.share) {
    try {
      await navigator.share({ text: message });
    } catch {
      // Dismissing the share sheet is not a failure worth a second prompt.
    }
    return;
  }

  if (await copyText(message)) {
    sayCopied(button, "Nudge copied");
  }
}

// Closes the top bar's group sheet when a tap lands outside it.
function closeGroupSwitcher(event) {
  for (const switcher of document.querySelectorAll("details[data-group-switcher][open]")) {
    if (!switcher.contains(event.target)) {
      switcher.open = false;
    }
  }
}

// Closes an open grade breakdown when a tap lands outside it, so two popovers
// never stack awkwardly on a phone.
function closeHelpPopovers(event) {
  for (const popover of document.querySelectorAll("details[data-help-popover][open]")) {
    if (!popover.contains(event.target)) {
      popover.open = false;
    }
  }
}

// Theme cycling and pc_theme cookie persistence.
const themeCatalog = [
  { id: "paper-turf", label: "Paper & Turf", shortLabel: "Paper", colorScheme: "dark", themeColor: "#0f1e16" },
  { id: "redzone", label: "RedZone", shortLabel: "RedZone", colorScheme: "dark", themeColor: "#090a0c" },
  { id: "almanac", label: "90s Almanac", shortLabel: "Almanac", colorScheme: "light", themeColor: "#fff8e7" },
];

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme.id;

  const schemeMeta = document.querySelector('meta[name="color-scheme"]');
  if (schemeMeta) {
    schemeMeta.setAttribute("content", theme.colorScheme);
  }
  const colorMeta = document.querySelector('meta[name="theme-color"]');
  if (colorMeta) {
    colorMeta.setAttribute("content", theme.themeColor);
  }

  for (const button of document.querySelectorAll("[data-theme-cycle]")) {
    button.setAttribute("title", `Theme: ${theme.label}`);
    button.setAttribute("aria-label", `Switch theme (${theme.label})`);
    const labelEl = button.querySelector("[data-theme-label]");
    if (labelEl) {
      labelEl.textContent = theme.shortLabel;
    }
  }

  let cookie = `pc_theme=${encodeURIComponent(theme.id)}; Path=/; Max-Age=31536000; SameSite=Lax`;
  if (window.location.protocol === "https:") {
    cookie += "; Secure";
  }
  document.cookie = cookie;
}

function cycleTheme() {
  const currentID = document.documentElement.dataset.theme || "paper-turf";
  const idx = themeCatalog.findIndex((t) => t.id === currentID);
  const next = themeCatalog[(idx + 1) % themeCatalog.length];
  applyTheme(next);
}

document.addEventListener("click", (event) => {
  const themeButton = event.target.closest("[data-theme-cycle]");
  if (themeButton) {
    cycleTheme();
    return;
  }
  const copyButton = event.target.closest("[data-copy-target]");
  if (copyButton) {
    copyInvite(copyButton);
    return;
  }
  const nudgeButton = event.target.closest("[data-nudge]");
  if (nudgeButton) {
    shareNudge(nudgeButton);
    return;
  }
  if (event.target.closest("[data-close-leg-sheet]")) {
    closeLegSheet();
    return;
  }
  if (event.target.closest("[data-close-swap-sheet]")) {
    closeSwapSheet();
    return;
  }
  if (event.target.closest("[data-close-dock]")) {
    closeDockSheet();
    return;
  }
  if (event.target.closest("[data-bug-dialog-close]")) {
    closeBugDialog();
    return;
  }
  closeGroupSwitcher(event);
  closeHelpPopovers(event);
});

// The leg form opens in a bottom sheet. The sheet shell lives on the slip page
// and HTMX fills its body, so open it only after the fetched form lands.
const legSheet = document.getElementById("leg-sheet");
const legSheetBody = document.getElementById("leg-sheet-body");

function openLegSheet() {
  if (legSheet && !legSheet.open) {
    legSheet.showModal();
  }
}

function closeLegSheet() {
  if (legSheet && legSheet.open) {
    legSheet.close();
  }
}

function clearLegSheet() {
  if (legSheetBody) {
    legSheetBody.replaceChildren();
  }
}

if (legSheet) {
  legSheet.addEventListener("close", clearLegSheet);
  document.body.addEventListener("leg-saved", closeLegSheet);
}

const swapSheet = document.getElementById("pick-swap");
const swapSheetBody = document.getElementById("pick-swap-body");

function openSwapSheet() {
  if (swapSheet && !swapSheet.open) {
    swapSheet.showModal();
  }
}

function closeSwapSheet() {
  if (swapSheet && swapSheet.open) {
    swapSheet.close();
  }
}

if (swapSheet) {
  swapSheet.addEventListener("close", () => {
    if (swapSheetBody) {
      swapSheetBody.replaceChildren();
    }
  });
  document.body.addEventListener("tray-updated", closeSwapSheet);
}

// The pick dock opens the viewer's picks in a sheet. Its body is fetched from
// /slips/{id}/tray, and the tray inside keeps itself fresh on the same signals
// as the dock, so a leg can be removed right there.
const dockSheet = document.getElementById("pick-tray-sheet");
const dockSheetBody = document.getElementById("pick-tray-body");

function openDockSheet() {
  if (dockSheet && !dockSheet.open) {
    dockSheet.showModal();
  }
}

function closeDockSheet() {
  if (dockSheet && dockSheet.open) {
    dockSheet.close();
  }
}

if (dockSheet) {
  dockSheet.addEventListener("close", () => {
    if (dockSheetBody) {
      dockSheetBody.replaceChildren();
    }
  });
}

// HTMX leaves a 4xx response in place by default. Rule refusals answer 422
// with a short message, so put that message in the form that sent the request.
function formForEvent(event) {
  const source = event.detail && event.detail.elt;
  return source && source.closest ? source.closest("form") : null;
}

function showFormError(form, message) {
  const error = form && form.querySelector("[data-form-error]");
  if (!error) {
    return false;
  }

  error.textContent = message;
  error.classList.remove("hidden");
  return true;
}

document.body.addEventListener("htmx:beforeRequest", (event) => {
  if (formForEvent(event)?.id === "bug-report-form") {
    const bugError = document.getElementById("bug-report-error");
    if (bugError) {
      bugError.replaceChildren();
    }
  }

  const error = formForEvent(event)?.querySelector("[data-form-error]");
  if (error) {
    error.textContent = "";
    error.classList.add("hidden");
  }
});

document.body.addEventListener("htmx:responseError", (event) => {
  const xhr = event.detail && event.detail.xhr;
  if (!xhr) {
    return;
  }

  if (formForEvent(event)?.id === "bug-report-form" && xhr.status !== 400) {
    const bugError = document.getElementById("bug-report-error");
    if (bugError) {
      const alert = document.createElement("div");
      alert.setAttribute("role", "alert");
      alert.className = "rounded-lg border border-rose-800 bg-rose-950/60 p-3 text-sm text-rose-200";
      alert.textContent = "Could not submit bug report. Please try again.";
      bugError.replaceChildren(alert);
    }
    return;
  }

  if (xhr.status !== 422) {
    return;
  }

  const message = (xhr.responseText || "").trim() || "That didn't work.";
  if (!showFormError(formForEvent(event), message)) {
    showToast(message);
  }
});

// A small toast covers refusals that do not happen inside a form with an
// error slot, like lock in or unlock.
let toastTimer = null;

function showToast(message) {
  let toast = document.getElementById("app-toast");
  if (!toast) {
    toast = document.createElement("p");
    toast.id = "app-toast";
    toast.setAttribute("role", "status");
    toast.className =
      "fixed inset-x-3 bottom-20 z-40 mx-auto max-w-sm rounded-lg border border-rose-700 bg-rose-950 px-3 py-2 text-sm text-rose-200 shadow-xl";
    document.body.append(toast);
  }

  toast.textContent = message;
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toast.remove(), 4000);
}

// Live updates. The slip page marks its body with data-live-url. One
// EventSource per page carries only signals, so the page refetches the
// partials it shows. The server says bye at its deadline, and the client closes
// the stream when hidden, unloaded, or idle for 3m, then reopens and refreshes
// on interaction, focus, or return to visibility.
let liveURL = "";
let liveSource = null;
let boardUpdateTimer = null;
let liveIdleTimer = null;
let liveIdlePaused = false;
const boardUpdateDebounceMs = 150;
const liveIdleTimeoutMs = 3 * 60 * 1000;
const liveActivityEvents = ["pointerdown", "keydown", "scroll", "touchstart", "htmx:beforeRequest"];

function triggerBoardUpdated() {
  if (!window.htmx || !document.body) {
    return;
  }

  clearTimeout(boardUpdateTimer);
  boardUpdateTimer = setTimeout(() => {
    boardUpdateTimer = null;
    htmx.trigger(document.body, "board-updated");
  }, boardUpdateDebounceMs);
}

function clearLiveIdleTimer() {
  clearTimeout(liveIdleTimer);
  liveIdleTimer = null;
}

function resetLiveIdleTimer() {
  clearLiveIdleTimer();
  if (!liveURL || document.hidden || liveIdlePaused) {
    return;
  }

  liveIdleTimer = setTimeout(pauseLive, liveIdleTimeoutMs);
}

function openLive() {
  if (!liveURL || document.hidden || liveSource) {
    return;
  }

  liveIdlePaused = false;
  liveSource = new EventSource(liveURL);

  liveSource.addEventListener("slip-updated", () => {
    htmx.trigger(document.body, "slip-updated");
  });

  liveSource.addEventListener("board-updated", triggerBoardUpdated);

  liveSource.addEventListener("bye", pauseLive);
}

function closeLive() {
  if (!liveSource) {
    return;
  }

  liveSource.close();
  liveSource = null;
}

function pauseLive() {
  clearLiveIdleTimer();
  closeLive();
  if (liveURL && !document.hidden) {
    liveIdlePaused = true;
  }
}

// A reopen may have missed events while the tab was hidden, idle-paused, or
// past its server deadline, so refetch both partials.
function refreshLive() {
  if (!liveURL || !window.htmx || !document.body) {
    return;
  }

  htmx.trigger(document.body, "slip-updated");
  triggerBoardUpdated();
}

function resumeLive() {
  if (!liveURL || document.hidden) {
    return;
  }

  liveIdlePaused = false;
  openLive();
  refreshLive();
  resetLiveIdleTimer();
}

function handleLiveActivity() {
  if (!liveURL || document.hidden) {
    return;
  }

  if (liveIdlePaused) {
    resumeLive();
    return;
  }

  resetLiveIdleTimer();
}

const liveRoot = document.querySelector("[data-live-url]");
if (liveRoot) {
  liveURL = liveRoot.getAttribute("data-live-url") || "";
  if (liveURL) {
    openLive();
    resetLiveIdleTimer();
    for (const eventName of liveActivityEvents) {
      document.addEventListener(eventName, handleLiveActivity, { capture: true, passive: true });
    }
  }
}

// A glance page like the group list carries data-refresh-on-visible instead of
// a live stream, so it costs nothing while hidden. Refetch it when the tab
// comes back into view.
function refreshOnVisible() {
  if (document.hidden) {
    return;
  }

  for (const element of document.querySelectorAll("[data-refresh-on-visible]")) {
    if (window.htmx) {
      htmx.trigger(element, "refresh-on-visible");
    }
  }
}

function teardownLive() {
  clearTimeout(boardUpdateTimer);
  boardUpdateTimer = null;
  clearLiveIdleTimer();
  liveIdlePaused = false;
  closeLive();
}

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    teardownLive();
    return;
  }

  resumeLive();
  refreshOnVisible();
});

window.addEventListener("focus", () => {
  resumeLive();
  refreshOnVisible();
});

window.addEventListener("pagehide", teardownLive);

// Bug report modal: captures the current viewport with html2canvas (downscaled
// to at most 1280px wide as JPEG), populates the preview and diagnostic fields,
// and resets back to the initial form when closed.
const bugTrigger = document.getElementById("bug-report-trigger");
const bugDialog = document.getElementById("bug-report-dialog");
const bugReportBody = document.getElementById("bug-report-body");
const initialBugBodyHTML = bugReportBody ? bugReportBody.innerHTML : "";
const maxBugScreenshotWidth = 1280;
const maxBugScreenshotDataURLLength = 1300000;
let html2canvasPromise = null;

function loadHtml2Canvas() {
  if (typeof window.html2canvas === "function") {
    return Promise.resolve(window.html2canvas);
  }
  if (html2canvasPromise) {
    return html2canvasPromise;
  }

  const src = bugTrigger?.getAttribute("data-html2canvas-src");
  if (!src) {
    return Promise.resolve(null);
  }

  html2canvasPromise = new Promise((resolve) => {
    const script = document.createElement("script");
    script.src = src;
    script.async = true;
    script.onload = () => resolve(typeof window.html2canvas === "function" ? window.html2canvas : null);
    script.onerror = () => {
      html2canvasPromise = null;
      resolve(null);
    };
    document.head.append(script);
  });
  return html2canvasPromise;
}

function scaleBugCanvas(canvas) {
  if (!canvas || !canvas.width || !canvas.height) {
    return canvas;
  }
  if (canvas.width <= maxBugScreenshotWidth) {
    return canvas;
  }

  const ratio = maxBugScreenshotWidth / canvas.width;
  const scaled = document.createElement("canvas");
  scaled.width = maxBugScreenshotWidth;
  scaled.height = Math.max(1, Math.round(canvas.height * ratio));

  const ctx = scaled.getContext("2d");
  if (!ctx) {
    return canvas;
  }
  ctx.drawImage(canvas, 0, 0, scaled.width, scaled.height);
  return scaled;
}

async function captureBugScreenshot() {
  const capture = await loadHtml2Canvas();
  if (typeof capture !== "function") {
    return "";
  }

  try {
    const canvas = await capture(document.body, {
      useCORS: true,
      logging: false,
      x: window.scrollX,
      y: window.scrollY,
      width: window.innerWidth,
      height: window.innerHeight,
    });
    const output = scaleBugCanvas(canvas);
    let dataURL = output.toDataURL("image/jpeg", 0.8);
    if (dataURL.length > maxBugScreenshotDataURLLength) {
      dataURL = output.toDataURL("image/jpeg", 0.5);
    }
    if (!dataURL.startsWith("data:image/jpeg;base64,") || dataURL.length > maxBugScreenshotDataURLLength) {
      return "";
    }
    return dataURL;
  } catch {
    return "";
  }
}

function renderBugPreview(dataURL) {
  const slot = document.getElementById("bug-preview-slot");
  if (!slot) {
    return;
  }
  if (!dataURL) {
    slot.replaceChildren();
    slot.classList.add("hidden");
    return;
  }

  const img = document.createElement("img");
  img.id = "bug-preview";
  img.alt = "Screenshot preview";
  img.className = "max-h-48 w-full object-contain";
  img.src = dataURL;
  slot.replaceChildren(img);
  slot.classList.remove("hidden");
}

function populateBugDiagnostics(dataURL) {
  const screenshotInput = document.getElementById("bug-input-screenshot");
  const pageURLInput = document.getElementById("bug-input-page-url");
  const screenSizeInput = document.getElementById("bug-input-screen-size");
  const userAgentInput = document.getElementById("bug-input-user-agent");

  if (screenshotInput) {
    screenshotInput.value = dataURL;
  }
  if (pageURLInput) {
    pageURLInput.value = window.location.href;
  }
  if (screenSizeInput) {
    screenSizeInput.value = `${window.innerWidth}x${window.innerHeight}`;
  }
  if (userAgentInput) {
    userAgentInput.value = navigator.userAgent || "";
  }
  renderBugPreview(dataURL);
}

function resetBugDialog() {
  if (bugReportBody && initialBugBodyHTML) {
    bugReportBody.innerHTML = initialBugBodyHTML;
    if (window.htmx) {
      window.htmx.process(bugReportBody);
    }
  }
  renderBugPreview("");
}

function closeBugDialog() {
  if (bugDialog && bugDialog.open) {
    bugDialog.close();
  }
}

async function openBugDialog() {
  if (!bugDialog || bugDialog.open) {
    return;
  }
  if (bugTrigger) {
    bugTrigger.disabled = true;
  }
  try {
    const dataURL = await captureBugScreenshot();
    populateBugDiagnostics(dataURL);
    if (!bugDialog.open) {
      bugDialog.showModal();
    }
  } finally {
    if (bugTrigger) {
      bugTrigger.disabled = false;
    }
  }
}

if (bugTrigger && bugDialog) {
  bugTrigger.addEventListener("click", openBugDialog);
  bugDialog.addEventListener("close", resetBugDialog);
}

// Slip creation form: mode toggle (one-off vs recurring), weekday shortcut
// chips, and live selected-game count.
function updateSelectedGamesCount(form) {
  const counter = form.querySelector("[data-selected-games-count]");
  if (!counter) {
    return;
  }
  const checked = form.querySelectorAll('input[name="game_id"][data-game-day]:checked').length;
  counter.textContent = `${checked} ${checked === 1 ? "game" : "games"} selected`;
}

document.addEventListener("click", (event) => {
  const target = event.target instanceof Element ? event.target : null;
  if (!target) {
    return;
  }

  const modeBtn = target.closest("[data-slip-mode-btn]");
  if (modeBtn) {
    const form = modeBtn.closest("[data-slip-create-form]");
    if (!form) {
      return;
    }
    const mode = modeBtn.getAttribute("data-slip-mode-btn");
    const input = form.querySelector("[data-slip-mode-input]");
    if (input && mode) {
      input.value = mode;
    }
    for (const btn of form.querySelectorAll("[data-slip-mode-btn]")) {
      const active = btn.getAttribute("data-slip-mode-btn") === mode;
      btn.setAttribute("aria-pressed", active ? "true" : "false");
      btn.classList.toggle("bg-accent", active);
      btn.classList.toggle("text-accent-ink", active);
      btn.classList.toggle("text-ink-muted", !active);
    }
    for (const panel of form.querySelectorAll("[data-slip-mode-panel]")) {
      panel.hidden = panel.getAttribute("data-slip-mode-panel") !== mode;
    }
    return;
  }

  const shortcutBtn = target.closest("[data-slate-shortcut]");
  if (shortcutBtn) {
    const form = shortcutBtn.closest("[data-slip-create-form]");
    if (!form) {
      return;
    }
    const key = shortcutBtn.getAttribute("data-slate-shortcut");
    const boxes = form.querySelectorAll('input[name="game_id"][data-game-day]');
    for (const box of boxes) {
      if (key === "all") {
        box.checked = true;
      } else if (key === "clear") {
        box.checked = false;
      } else {
        box.checked = box.getAttribute("data-game-day") === key;
      }
    }
    updateSelectedGamesCount(form);
  }
});

document.addEventListener("change", (event) => {
  const input = event.target instanceof HTMLInputElement ? event.target : null;
  if (!input || !input.matches('input[name="game_id"][data-game-day]')) {
    return;
  }
  const form = input.closest("[data-slip-create-form]");
  if (form) {
    updateSelectedGamesCount(form);
  }
});
