import { defineConfig, devices } from '@playwright/test';
import { ARTEFACTS, BASE_URL, EXTERNAL_BASE_URL, PORT, STORAGE_STATE } from './e2e/env';

// WebKit needs system libraries (libavif16 and friends, `sudo pnpm exec playwright install-deps`)
// that a dev machine may lack; its project runs only when SLOUCHCAT_E2E_WEBKIT=1.
const withWebkit = process.env.SLOUCHCAT_E2E_WEBKIT === '1';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // One server, one database: specs that change data (progress) must not race each other.
  //
  // CI splits the suite with `--shard` instead (ADR 0013). A shard is a whole run of its own on a
  // runner of its own -- own server, own database, own global setup -- so this stays 1 inside a
  // shard and nothing races. Playwright keeps a spec file whole while sharding (with
  // fullyParallel off, a file is one group), so the order within a file is the order it always had.
  // What a shard does not have is the data of the spec files that landed in the other shards:
  // every spec must create what it needs. On one machine the shards must still run one after the
  // other, because they all want the same port and the same artefact folder.
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  outputDir: `${ARTEFACTS}results`,
  // In CI every shard writes a blob report -- the machine-readable form that keeps the traces and
  // the screenshots -- and the gate job merges all of them into the one HTML report that a red run
  // uploads (`playwright merge-reports`). A single HTML report per shard would be six reports with
  // a sixth of the run in each.
  reporter: process.env.CI
    ? [['list'], ['blob', { outputDir: `${ARTEFACTS}blob` }]]
    : 'list',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  globalSetup: './e2e/global-setup.ts',
  // A-9: scans the server log of the run for secrets while the server still runs.
  globalTeardown: './e2e/global-teardown.ts',
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
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        // Chromium runs muted in Playwright, Firefox does not: the sine tones of the test media
        // would beep through the speakers during every run. Volume 0 still needs an audio output:
        // without one Firefox fails the audio sink of an audio-only title (MediaError 3
        // "OnMediaSinkAudioError") and hls.js re-attaches the element over and over. A machine
        // without a sound card therefore needs a null sink (CI workflow, docs/dev/SETUP.md).
        launchOptions: { firefoxUserPrefs: { 'media.volume_scale': '0.0' } },
      },
      // Flight mode cannot be emulated with a service worker in Playwright's Firefox: setOffline
      // does not reach the worker's own requests (the app still gets its session), and a route
      // that cuts the API makes Firefox bypass the worker for the page (NS_ERROR_OFFLINE). The
      // cold start in flight mode (C-1) therefore runs in Chromium; the offline specs that run
      // here prove the worker's control and its precache (prepareOffline, offline playback).
      testIgnore: ['**/offline-cold-start.spec.ts'],
    },
    ...(withWebkit ? [{ name: 'webkit', use: { ...devices['Desktop Safari'] } }] : []),
  ],
});
