// Sunday Parlay Club service worker.
//
// It exists so the app can install and receive Web Push. It deliberately has
// no fetch handler and no cache: pages still come straight from the network,
// and a stale worker can never serve an old page.

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("push", (event) => {
  let payload = {};
  try {
    payload = event.data ? event.data.json() : {};
  } catch {
    payload = {};
  }

  event.waitUntil(
    self.registration.showNotification(payload.title || "Sunday Parlay Club", {
      body: payload.body || "There's news on your slip.",
      tag: payload.tag || undefined,
      icon: "/static/icon-192.png",
      data: { url: payload.url || "/" },
    }),
  );
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();

  const path = (event.notification.data && event.notification.data.url) || "/";
  event.waitUntil(openApp(path));
});

// Focus a tab already showing the app, or open a new one. The navigation keeps
// the existing tab on the slip the notification is about.
async function openApp(path) {
  const target = new URL(path, self.location.origin).href;
  const windows = await self.clients.matchAll({ type: "window", includeUncontrolled: true });

  for (const client of windows) {
    if (new URL(client.url).origin !== self.location.origin) {
      continue;
    }

    await client.focus();
    if ("navigate" in client) {
      await client.navigate(target);
    }
    return;
  }

  await self.clients.openWindow(target);
}
