import { fileURLToPath } from 'node:url';
import { defineConfig, devices } from '@playwright/test';

// Screenshot checks of every Storybook story (A-2: at most 0.1 % of the pixels may differ from the
// approved baseline). Runs against the static build (`pnpm run storybook:build`) served by Vite.
//
// Baselines come from Chromium in the official Playwright image (mcr.microsoft.com/playwright,
// same version as @playwright/test): fonts and rendering then match on every machine and in CI.
// `task web:storybook:test` starts that container; `task web:storybook:update` rewrites the
// baselines after an intended change.

const PORT = 6007;
const ARTEFACTS = fileURLToPath(new URL('./node_modules/.playwright-storybook/', import.meta.url));

export default defineConfig({
  testDir: './e2e-storybook',
  snapshotPathTemplate: '{testDir}/__screenshots__/{arg}{ext}',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: 0,
  outputDir: `${ARTEFACTS}results`,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never', outputFolder: `${ARTEFACTS}report` }]]
    : 'list',
  timeout: 30_000,
  expect: {
    toHaveScreenshot: { maxDiffPixelRatio: 0.001, animations: 'disabled', caret: 'hide' },
  },
  use: {
    baseURL: `http://127.0.0.1:${PORT}`,
    viewport: { width: 1280, height: 720 },
    deviceScaleFactor: 1,
    locale: 'de-CH',
    timezoneId: 'Europe/Zurich',
    colorScheme: 'dark',
    // The carousel stays on its first slide; transitions are finished by the screenshot anyway.
    reducedMotion: 'reduce',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: `node_modules/.bin/vite preview --config .storybook/vite.config.ts --outDir storybook-static --host 127.0.0.1 --port ${PORT} --strictPort`,
    url: `http://127.0.0.1:${PORT}/index.json`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1280, height: 720 } },
    },
  ],
});
