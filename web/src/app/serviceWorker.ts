// Idle period before the registration; keeps the precache off the link while the page still loads.
const IDLE_TIMEOUT_MS = 5000;

/**
 * Registers the service worker (vite-plugin-pwa, generateSW with auto update) once the page has
 * loaded and the browser is idle. Not an inline script in the shell: the start JS follows the hero
 * image (vite.config.ts), and a registration on `load` could precache the whole app shell while the
 * start JS and the posters are still downloading (P-10). Dev builds have no worker.
 */
export function registerServiceWorker() {
  if (!import.meta.env.PROD || !('serviceWorker' in navigator)) return;
  const register = () => {
    void navigator.serviceWorker.register('/sw.js', { scope: '/' });
    if (navigator.serviceWorker.controller) return;
    // First start: the page loaded its language before the worker controlled it.
    navigator.serviceWorker.addEventListener('controllerchange', cacheLoadedLocales, {
      once: true,
    });
  };
  const whenIdle = () => {
    // Safari has no requestIdleCallback.
    if (typeof window.requestIdleCallback === 'function') {
      window.requestIdleCallback(register, { timeout: IDLE_TIMEOUT_MS });
    } else {
      window.setTimeout(register, 0);
    }
  };
  if (document.readyState === 'complete') whenIdle();
  else window.addEventListener('load', whenIdle, { once: true });
}

/** A locale chunk of the build (vite.config.ts, localeChunk). */
const LOCALE_CHUNK = /\/assets\/locale-[^./]+\.[^./]+\.js$/;

/**
 * Fetches the locale chunks this page has loaded once more, now through the worker: only de, de-DE
 * and en are precached (concept 22 section 4.4), the others are kept by a CacheFirst rule when
 * fetched under the worker. Without this a French device would start offline without French after
 * its first visit. The browser's HTTP cache usually answers.
 */
export function cacheLoadedLocales(): void {
  for (const entry of performance.getEntriesByType('resource')) {
    if (LOCALE_CHUNK.test(new URL(entry.name).pathname))
      void fetch(entry.name).catch(() => undefined);
  }
}
