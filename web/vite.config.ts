import { fileURLToPath, URL } from 'node:url';
import { tokens } from '@slouchcat/design-tokens';
import react from '@vitejs/plugin-react';
import { defineConfig, type Plugin } from 'vite';
import { VitePWA } from 'vite-plugin-pwa';

// App chrome colours come from the design tokens, the single source for every colour.
const chromeColor = tokens.color.bg.base;

/** Adds the theme-color meta tag with the token value, so index.html carries no colour literal. */
function themeColorMeta(): Plugin {
  return {
    name: 'slouchcat:theme-color',
    transformIndexHtml: () => [
      { tag: 'meta', attrs: { name: 'theme-color', content: chromeColor }, injectTo: 'head' },
    ],
  };
}

// Hero image first, start JS second (P-10). The server writes the first hero image into the start
// page's shell (#hero-prepaint, api/preload.go). This inline loader, placed at the end of <body> by
// deferStartScript, appends the module scripts once that image is painted (two frames after its
// load event, like the carousel) and right away when there is no such image (login, other pages),
// the image failed or the document is hidden. The image and the JS then no longer share a slow
// link, so the hero is on screen earlier; the JS arrives about as late as before, since the link
// carries the same bytes either way. The timer is a safety net for a stalled image.
//
// All of them, not just one: the bundler emits a runtime chunk of its own next to the entry, and
// deferring only the first left the entry eager in the head. The LCP went from 1.802 s (M1a) to
// 2.702 s that way, over the 2.5 s of P-10, and nothing said so -- the plugin only complained when
// it found no tag at all. Module scripts run in the order they are inserted, so the runtime still
// comes before the entry.
const startLoader = (srcs: string[]) => `(function () {
  var img = document.querySelector('#hero-prepaint > img');
  var started = false;
  function start() {
    if (started) return;
    started = true;
    ${JSON.stringify(srcs)}.forEach(function (src) {
      var script = document.createElement('script');
      script.type = 'module';
      script.crossOrigin = 'anonymous';
      script.src = src;
      document.head.appendChild(script);
    });
  }
  function afterPaint() {
    requestAnimationFrame(function () {
      requestAnimationFrame(start);
    });
  }
  if (!img || document.hidden) return start();
  if (img.complete) return afterPaint();
  img.addEventListener('load', afterPaint);
  img.addEventListener('error', start);
  setTimeout(start, 2500);
})();`;

const startScriptTags = /<script type="module" crossorigin src="([^"]+)"><\/script>\n?/g;

/** Replaces every module script tag of the built shell with the inline loader (see startLoader). */
function deferStartScript(): Plugin {
  return {
    name: 'slouchcat:defer-start-script',
    apply: 'build',
    enforce: 'post',
    transformIndexHtml: {
      order: 'post',
      handler: (html) => {
        const matches = [...html.matchAll(startScriptTags)];
        if (matches.length === 0) throw new Error('index.html: no start script tag found');
        const out = html.replaceAll(startScriptTags, '');
        // The guard that was missing: a tag the pattern does not know would stay eager in the head
        // and quietly undo the whole point of this plugin.
        if (out.includes('<script type="module"')) {
          throw new Error('index.html: a module script tag is left in the shell');
        }
        return {
          html: out,
          tags: [
            { tag: 'script', children: startLoader(matches.map((m) => m[1])), injectTo: 'body' },
          ],
        };
      },
    },
  };
}

export default defineConfig({
  plugins: [
    react(),
    themeColorMeta(),
    deferStartScript(),
    VitePWA({
      registerType: 'autoUpdate',
      // The app registers the worker itself once it runs and the browser is idle (app/serviceWorker.ts).
      injectRegister: false,
      // The glob below already precaches icons; avoid listing them twice.
      includeManifestIcons: false,
      manifest: {
        name: 'Slouchcat',
        short_name: 'Slouchcat',
        lang: 'de',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: chromeColor,
        theme_color: chromeColor,
        // PNGs are generated from icon.svg by `pnpm run icons` (scripts/icons.mjs).
        icons: [
          { src: 'icons/icon-192.png', sizes: '192x192', type: 'image/png', purpose: 'any' },
          { src: 'icons/icon-512.png', sizes: '512x512', type: 'image/png', purpose: 'any' },
          {
            src: 'icons/icon-512-maskable.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable',
          },
          { src: 'icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' },
        ],
      },
      workbox: {
        // The worker takes over the open page at once, not only the next navigation: an app
        // opened once (and gone offline since) gets the shell and every chunk from the precache
        // (C-1). `autoUpdate` brings skipWaiting only.
        clientsClaim: true,
        // Precache the app shell only (built JS, CSS, HTML, icons); never media or API answers.
        // That includes every lazy chunk (the offline library, its player, the OPFS worker and
        // the locale files), so the app starts and plays downloads without network (C-1, M5).
        globPatterns: ['**/*.{js,css,html,svg,png,webmanifest}'],
        globIgnores: ['**/node_modules/**/*', 'sw.js', 'workbox-*.js', 'push-sw.js', 'spike/**'],
        // Web Push (M8): the `push` and `notificationclick` listeners of public/push-sw.js run in
        // this worker; generateSW loads the file, the browser checks it for updates like sw.js.
        importScripts: ['push-sw.js'],
        // Every navigation without network gets the shell, /offline and /offline/play/* included;
        // the router opens the offline library from there.
        navigateFallback: 'index.html',
        // The API and the offline spike page are never answered with the app shell.
        navigateFallbackDenylist: [/^\/api\//, /^\/spike\//],
        runtimeCaching: [
          {
            // Posters and backdrops: immutable per content hash (`v`), served with the session.
            urlPattern: ({ url, sameOrigin }) =>
              sameOrigin && /^\/api\/v1\/items\/[^/]+\/images\//.test(url.pathname),
            handler: 'CacheFirst',
            options: {
              cacheName: 'slouchcat-images',
              expiration: { maxEntries: 300, maxAgeSeconds: 30 * 24 * 60 * 60 },
              cacheableResponse: { statuses: [200] },
            },
          },
          {
            // Everything else under /api (JSON, streams) always goes to the network.
            urlPattern: ({ url, sameOrigin }) => sameOrigin && url.pathname.startsWith('/api/'),
            handler: 'NetworkOnly',
          },
        ],
      },
    }),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      // SLOUCHCAT_API points the dev server at another Go server (e.g. the e2e one on :8097).
      '/api': process.env.SLOUCHCAT_API ?? 'http://localhost:8096',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rolldownOptions: {
      output: {
        // Everything the start page loads statically goes into one JS and one CSS file instead of
        // an entry, a shared chunk and the runtime: fewer requests before the hero (P-10). The
        // form pages, the player and react-aria stay separate chunks.
        codeSplitting: { groups: [{ name: 'index', tags: ['$initial'] }] },
      },
    },
  },
});
