import { defineConfig, devices } from '@playwright/test';
import { ARTEFACTS, BASE_URL, EXTERNAL_BASE_URL, PORT, STORAGE_STATE } from './e2e/env';

// WebKit needs system libraries (libavif16 and friends, `sudo pnpm exec playwright install-deps`)
// that a dev machine may lack; its project runs only when SLOUCHCAT_E2E_WEBKIT=1.
const withWebkit = process.env.SLOUCHCAT_E2E_WEBKIT === '1';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // One server, one database: specs that change data (progress) must not race each other.
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  outputDir: `${ARTEFACTS}results`,
  reporter: process.env.CI
    ? [['list'], ['html', { open: 'never', outputFolder: `${ARTEFACTS}report` }]]
    : 'list',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  globalSetup: './e2e/global-setup.ts',
  use: {
    baseURL: BASE_URL,
    storageState: STORAGE_STATE,
    locale: 'de-CH',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: EXTERNAL_BASE_URL
    ? undefined
    : {
        command: 'node e2e/start-server.mjs',
        url: `${BASE_URL}/api/v1/healthz`,
        reuseExistingServer: false,
        timeout: 60_000,
        stdout: 'pipe',
        stderr: 'pipe',
        env: { SLOUCHCAT_E2E_PORT: String(PORT) },
      },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    { name: 'firefox', use: { ...devices['Desktop Firefox'] } },
    ...(withWebkit ? [{ name: 'webkit', use: { ...devices['Desktop Safari'] } }] : []),
  ],
});
