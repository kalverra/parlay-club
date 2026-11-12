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

document.addEventListener("htmx:beforeSwap", () => {
  const pane = document.querySelector("[data-keep-scroll]");
  keptScrollTop = pane ? pane.scrollTop : null;
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

document.addEventListener("click", (event) => {
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
  closeGroupSwitcher(event);
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
  const error = formForEvent(event)?.querySelector("[data-form-error]");
  if (error) {
    error.textContent = "";
    error.classList.add("hidden");
  }
});

document.body.addEventListener("htmx:responseError", (event) => {
  const xhr = event.detail && event.detail.xhr;
  if (!xhr || xhr.status !== 422) {
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

// The top bar's slip switcher is a plain select; a choice loads that slip.
document.addEventListener("change", (event) => {
  if (event.target && event.target.id === "slip-switcher") {
    const slipID = event.target.value;
    if (slipID) {
      window.location.assign(`/slips/${encodeURIComponent(slipID)}`);
    }
  }
});

// Live updates. The slip page marks its body with data-live-url. One
// EventSource per page carries only signals, so the page refetches the
// partials it shows. The server says bye at its deadline, and the tab closes
// the stream while hidden because iOS Safari drops background connections.
let liveURL = "";
let liveSource = null;

function openLive() {
  if (!liveURL || document.hidden || liveSource) {
    return;
  }

  liveSource = new EventSource(liveURL);

  liveSource.addEventListener("slip-updated", () => {
    htmx.trigger(document.body, "slip-updated");
  });

  liveSource.addEventListener("board-updated", () => {
    htmx.trigger(document.body, "board-updated");
  });

  liveSource.addEventListener("bye", closeLive);
}

function closeLive() {
  if (!liveSource) {
    return;
  }

  liveSource.close();
  liveSource = null;
}

// A reopen may have missed events while the tab was hidden or the stream was
// past its deadline, so refetch both partials.
function refreshLive() {
  if (!liveURL) {
    return;
  }

  htmx.trigger(document.body, "slip-updated");
  htmx.trigger(document.body, "board-updated");
}

const liveRoot = document.querySelector("[data-live-url]");
if (liveRoot) {
  liveURL = liveRoot.getAttribute("data-live-url") || "";
  openLive();
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

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    closeLive();
    return;
  }

  openLive();
  refreshLive();
  refreshOnVisible();
});

window.addEventListener("focus", () => {
  openLive();
  refreshLive();
  refreshOnVisible();
});
