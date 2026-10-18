import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['test/**/*.test.ts'],
    // Keep console.table output of the contrast test visible on green runs.
    silent: false,
  },
});
