// Starts the Slouchcat binary for the Playwright suite with fresh temporary config and cache
// directories, so every run begins at the setup wizard. Playwright's `webServer` runs this script
// and stops it after the run; the temporary directories are removed on exit. The local podcast
// feed (feed-server.mjs, M6) runs in this process and stops with the server.
import { spawn } from 'node:child_process';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { startFeedServer } from './feed-server.mjs';

const binary = fileURLToPath(new URL('../../server/bin/slouchcat', import.meta.url));
const port = process.env.SLOUCHCAT_E2E_PORT ?? '8097';

if (!existsSync(binary)) {
  console.error(`slouchcat binary not found at ${binary}; run \`task build\` first.`);
  process.exit(1);
}

// Before the server: the global setup subscribes to the feed right after the server is up. Its
// port follows the app's (env.ts FEED_PORT), so runs on other ports do not collide.
const feed = await startFeedServer({
  port: Number(process.env.SLOUCHCAT_E2E_FEED_PORT ?? Number(port) - 2),
});

const root = mkdtempSync(join(tmpdir(), 'slouchcat-e2e-'));
const configDir = join(root, 'config');
const cacheDir = join(root, 'cache');

const server = spawn(
  binary,
  [
    'serve',
    '--listen',
    `:${port}`,
    '--config-dir',
    configDir,
    '--cache-dir',
    cacheDir,
    '--dev',
    '--log-format',
    'text',
  ],
  {
    stdio: 'inherit',
    env: {
      ...process.env,
      // No TMDB key in the e2e run: titles come from the file names.
      SLOUCHCAT_TMDB_API_KEY: '',
      // The specs that log in, try PINs or open invitation links give every test its own client
      // address (X-Forwarded-For, e2e/fixtures.ts), so the limit of 10 attempts per minute and
      // address does not couple tests to each other. The browser connects from the loopback.
      SLOUCHCAT_TRUSTED_PROXIES: 'private',
    },
  },
);

const cleanup = () => rmSync(root, { recursive: true, force: true });

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => server.kill(signal));
}
server.on('exit', (code) => {
  cleanup();
  void feed.close().finally(() => process.exit(code ?? 0));
});
