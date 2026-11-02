import { defineConfig, devices } from '@playwright/test';
import base from './playwright.config';

// Measurements of the administration pages (concept 12, section 3): not part of `task ci`. It reuses
// the e2e server and global setup of playwright.config.ts (fresh server, main account, libraries)
// and runs the specs in e2e-perf/ in Chromium only. Run it after `task build`, on a port of its own:
//
//   cd web && SLOUCHCAT_E2E_PORT=8197 node_modules/.bin/playwright test -c perf.config.ts
//
// The numbers go to docs/perf/verwaltungsseiten.md.
export default defineConfig({
  ...base,
  testDir: './e2e-perf',
  retries: 0,
  reporter: 'list',
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
