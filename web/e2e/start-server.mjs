// Starts the Slouchcat binary for the Playwright suite with fresh temporary config and cache
// directories, so every run begins at the setup wizard. Playwright's `webServer` runs this script
// and stops it after the run; the temporary directories are removed on exit. The local podcast
// feed (feed-server.mjs, M6), the Trakt mock (trakt-server.mjs, M7) and the mocks of the
// notification channels and the update check (smtp-, push-, telegram- and update-server.mjs, M8)
// run in this process and stop with the server.
//
// The server's output goes to the console as before and into node_modules/.playwright/server.log
// (emptied at every start), which global-teardown.ts scans for secrets (A-9).
import { spawn } from 'node:child_process';
import {
  createWriteStream,
  existsSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { startFeedServer } from './feed-server.mjs';
import { startPushServer } from './push-server.mjs';
import { SMTP_FROM, SMTP_PASSWORD, SMTP_USER, startSmtpServer } from './smtp-server.mjs';
import { startTelegramServer, TELEGRAM_BOT_TOKEN } from './telegram-server.mjs';
import { startTraktServer, TRAKT_CLIENT_ID } from './trakt-server.mjs';
import { startUpdateServer } from './update-server.mjs';

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

// Trakt (E-7): three below the app's port (8094 next to 8097), like the feed two below.
const trakt = await startTraktServer({
  port: Number(process.env.SLOUCHCAT_E2E_TRAKT_PORT ?? Number(port) - 3),
});

// The channels and the update check (M8): SMTP four, push five, Telegram six and the release
// manifest seven below the app's port, the SMTP mock's mailbox (HTTP) eight below (env.ts).
const below = (name, offset) =>
  Number(process.env[`SLOUCHCAT_E2E_${name}_PORT`] ?? Number(port) - offset);
const smtp = await startSmtpServer({ port: below('SMTP', 4), httpPort: below('SMTP_HTTP', 8) });
const push = await startPushServer({ port: below('PUSH', 5) });
const telegram = await startTelegramServer({ port: below('TELEGRAM', 6) });
const update = await startUpdateServer({ port: below('UPDATE', 7) });

const logFile = fileURLToPath(new URL('../node_modules/.playwright/server.log', import.meta.url));
mkdirSync(dirname(logFile), { recursive: true });
// Emptied, then appended to: every write lands at the end, also next to other writers.
writeFileSync(logFile, '');
const log = createWriteStream(logFile, { flags: 'a' });

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
    // Debug records go into the log scan of global-teardown.ts too (A-9).
    '--log-level',
    'debug',
  ],
  {
    stdio: ['ignore', 'pipe', 'pipe'],
    env: {
      ...process.env,
      // No TMDB key in the e2e run: titles come from the file names.
      SLOUCHCAT_TMDB_API_KEY: '',
      // The specs that log in, try PINs or open invitation links give every test its own client
      // address (X-Forwarded-For, e2e/fixtures.ts), so the limit of 10 attempts per minute and
      // address does not couple tests to each other. The browser connects from the loopback.
      SLOUCHCAT_TRUSTED_PROXIES: 'private',
      // The public Trakt lists come from the local mock (trakt-server.mjs) with a fake client id.
      SLOUCHCAT_TRAKT_CLIENT_ID: TRAKT_CLIENT_ID,
      SLOUCHCAT_TRAKT_BASE_URL: `http://127.0.0.1:${trakt.port}`,
      // E-mail through the SMTP mock, plain text (fake credentials; the log scan looks for them).
      SLOUCHCAT_SMTP_HOST: '127.0.0.1',
      SLOUCHCAT_SMTP_PORT: String(smtp.port),
      SLOUCHCAT_SMTP_USER: SMTP_USER,
      SLOUCHCAT_SMTP_PASSWORD: SMTP_PASSWORD,
      SLOUCHCAT_SMTP_FROM: SMTP_FROM,
      SLOUCHCAT_SMTP_TLS: 'none',
      // The Telegram bot API and the release manifest of the mocks; the server honours both base
      // URLs in development mode (--dev) only. Push needs no setting: the endpoints come with the
      // subscriptions, which the specs take from the push mock.
      SLOUCHCAT_TELEGRAM_BOT_TOKEN: TELEGRAM_BOT_TOKEN,
      SLOUCHCAT_TELEGRAM_BASE_URL: `http://127.0.0.1:${telegram.port}`,
      SLOUCHCAT_UPDATE_MANIFEST_URL: `http://127.0.0.1:${update.port}/manifest.json`,
    },
  },
);

// To the console as before, and into the log file for the scan.
server.stdout.pipe(process.stdout);
server.stderr.pipe(process.stderr);
server.stdout.pipe(log, { end: false });
server.stderr.pipe(log, { end: false });

const cleanup = () => rmSync(root, { recursive: true, force: true });

for (const signal of ['SIGINT', 'SIGTERM']) {
  process.on(signal, () => server.kill(signal));
}
server.on('exit', (code) => {
  cleanup();
  log.end();
  void Promise.all([
    feed.close(),
    trakt.close(),
    smtp.close(),
    push.close(),
    telegram.close(),
    update.close(),
  ]).finally(() => process.exit(code ?? 0));
});
