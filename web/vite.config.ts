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

export default defineConfig({
  plugins: [
    react(),
    themeColorMeta(),
    VitePWA({
      registerType: 'autoUpdate',
      injectRegister: 'auto',
      includeAssets: ['icon.svg'],
      manifest: {
        name: 'Slouchcat',
        short_name: 'Slouchcat',
        lang: 'de',
        start_url: '/',
        scope: '/',
        display: 'standalone',
        background_color: chromeColor,
        theme_color: chromeColor,
        icons: [{ src: 'icon.svg', sizes: 'any', type: 'image/svg+xml', purpose: 'any' }],
      },
      workbox: {
        // API calls always go to the network; only the app shell is precached.
        navigateFallbackDenylist: [/^\/api\//],
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
      '/api': 'http://localhost:8096',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
});
