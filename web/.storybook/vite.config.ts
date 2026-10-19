import { fileURLToPath, URL } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

// Vite config for Storybook only (main.ts): React and the '@' alias, as in web/vite.config.ts.
export default defineConfig({
  plugins: [react()],
  // Story artwork comes from .storybook/public (staticDirs); the app's icons are not needed.
  publicDir: false,
  // The preview bundles all stories into one iframe chunk; its size does not reach users.
  build: { chunkSizeWarningLimit: 2000 },
  resolve: {
    alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) },
  },
});
