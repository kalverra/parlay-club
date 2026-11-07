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

// HTMX swaps in partials, so format whatever arrived and whatever was there
// before the swap.
document.addEventListener("htmx:afterSwap", (event) => {
  const target = (event.detail && event.detail.target) || document;
  formatTimes(target);
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

// Copies an invite link from its read-only input, then briefly says so.
async function copyInvite(button) {
  const source = document.querySelector(button.getAttribute("data-copy-target"));
  if (!source) {
    return;
  }

  try {
    await navigator.clipboard.writeText(source.value);
  } catch {
    source.select();
    document.execCommand("copy");
  }

  const label = button.textContent;
  button.textContent = "Copied";
  setTimeout(() => {
    button.textContent = label;
  }, 1500);
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
  closeGroupSwitcher(event);
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

document.addEventListener("visibilitychange", () => {
  if (document.hidden) {
    closeLive();
    return;
  }

  openLive();
  refreshLive();
});

window.addEventListener("focus", () => {
  openLive();
  refreshLive();
});
