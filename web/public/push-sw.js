// Web Push in the service worker (M8, A-6). The generated worker (vite-plugin-pwa, generateSW)
// loads this file with importScripts (vite.config.ts, workbox.importScripts); it is plain JS outside
// the bundle, so it stays small. The server sends `{title, body, url}` (notify, design 3.2); the
// notification opens `url` on this origin when clicked. When the push service replaces the
// subscription, the worker subscribes again and registers the new one with the server. Tested in
// src/app/pushWorker.test.ts.

/** Where the app registers a subscription (POST /me/push-subscriptions, cookie session). */
const SUBSCRIPTIONS_URL = '/api/v1/me/push-subscriptions';

/** What a push message shows: title, text and the path a click opens (this origin only). */
function pushNotification(text) {
  let data = null;
  try {
    data = JSON.parse(text);
  } catch {
    data = null;
  }
  const message = data && typeof data === 'object' ? data : { body: text };
  const title = typeof message.title === 'string' && message.title ? message.title : 'Slouchcat';
  const body = typeof message.body === 'string' ? message.body : '';
  let path = '/';
  if (typeof message.url === 'string' && message.url) {
    try {
      // An absolute link (the server's public URL) opens on the origin the app runs on.
      const url = new URL(message.url, self.location.origin);
      path = url.pathname + url.search + url.hash;
    } catch {
      path = '/';
    }
  }
  return { title, options: { body, icon: '/icons/icon-192.png', data: { url: path } } };
}

self.addEventListener('push', (event) => {
  const { title, options } = pushNotification(event.data ? event.data.text() : '');
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const path = (event.notification.data && event.notification.data.url) || '/';
  const target = new URL(path, self.location.origin).href;
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((windows) => {
      // A window of the app: show it and open the link there; else (or when it cannot
      // navigate) a new window.
      const open = windows.find((client) => client.url.startsWith(self.location.origin));
      if (!open) return clients.openWindow(target);
      return open
        .focus()
        .then(() => (open.url === target ? undefined : open.navigate(target)))
        .catch(() => clients.openWindow(target));
    }),
  );
});

/**
 * The push service replaced or dropped the subscription (Firefox does this): subscribe again with
 * the old one's server key, unless the browser already did, and register the new one with the
 * server like the settings page does (same origin, the session cookie, the CSRF header). Best
 * effort: without a session (or for a profile that may not change channels) nothing is stored, and
 * the server forgets the old endpoint at its next 410.
 */
function renewSubscription(oldSubscription, newSubscription) {
  const key =
    oldSubscription && oldSubscription.options && oldSubscription.options.applicationServerKey;
  const next = newSubscription
    ? Promise.resolve(newSubscription)
    : key
      ? self.registration.pushManager.subscribe({
          userVisibleOnly: true,
          applicationServerKey: key,
        })
      : Promise.resolve(null);
  return next
    .then((subscription) => {
      if (!subscription) return undefined;
      const json = subscription.toJSON();
      if (!json.endpoint || !json.keys || !json.keys.p256dh || !json.keys.auth) return undefined;
      return fetch(SUBSCRIPTIONS_URL, {
        method: 'POST',
        credentials: 'same-origin',
        headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'Slouchcat' },
        body: JSON.stringify({
          endpoint: json.endpoint,
          keys: { p256dh: json.keys.p256dh, auth: json.keys.auth },
        }),
      });
    })
    .catch(() => undefined);
}

self.addEventListener('pushsubscriptionchange', (event) => {
  event.waitUntil(renewSubscription(event.oldSubscription, event.newSubscription));
});
