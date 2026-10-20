// O-6 long-term test, desktop part (M5 step 0, closed in M7; protocol docs/perf/M5-o6-langzeittest.md).
//
//   node web/scripts/o6-longterm.mjs start   prepares a server on :8098 with the movie library, downloads
//                                            one film in Chromium and in Firefox, each with a browser
//                                            profile of its own that survives the 30 days, then stops
//   node web/scripts/o6-longterm.mjs check   with no server running (the browsers are really offline):
//                                            opens the app in both profiles, expects the offline library,
//                                            plays the download, jumps to 50 % and appends the result
//
// State lives outside the repository in $SLOUCHCAT_O6_DIR (default ~/.local/share/slouchcat-o6): the
// server's config and cache, the two browser profiles, the account (random password, file mode 0600)
// and o6-log.json with one entry per run. Needs `task build` and `task testdata:media` first.
import { chromium, firefox, request } from '@playwright/test';
import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = fileURLToPath(new URL('../../', import.meta.url));
const dir = process.env.SLOUCHCAT_O6_DIR ?? join(homedir(), '.local', 'share', 'slouchcat-o6');
const port = 8098;
const base = `http://localhost:${port}`;
const title = 'Slouchcat Test H264';
const csrf = { 'X-Requested-With': 'Slouchcat' };
const browsers = { chromium, firefox };
const accountFile = join(dir, 'account.json');
const logFile = join(dir, 'o6-log.json');

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

function log(entry) {
  const entries = existsSync(logFile) ? JSON.parse(readFileSync(logFile, 'utf8')) : [];
  entries.push({ at: new Date().toISOString(), ...entry });
  writeFileSync(logFile, `${JSON.stringify(entries, null, 2)}\n`);
  console.log(JSON.stringify(entry));
}

async function until(what, probe, timeoutMs) {
  const end = Date.now() + timeoutMs;
  for (;;) {
    const value = await probe();
    if (value) return value;
    if (Date.now() > end) throw new Error(`timed out: ${what}`);
    await sleep(1000);
  }
}

function startServer() {
  mkdirSync(join(dir, 'server', 'config'), { recursive: true });
  mkdirSync(join(dir, 'server', 'cache'), { recursive: true });
  const server = spawn(
    join(repo, 'server', 'bin', 'slouchcat'),
    [
      'serve',
      '--listen',
      `:${port}`,
      '--config-dir',
      join(dir, 'server', 'config'),
      '--cache-dir',
      join(dir, 'server', 'cache'),
      '--log-format',
      'text',
    ],
    { stdio: ['ignore', 'ignore', 'inherit'] },
  );
  return server;
}

async function account(api) {
  if (existsSync(accountFile)) return JSON.parse(readFileSync(accountFile, 'utf8'));
  const created = { email: 'o6@slouchcat.test', password: randomBytes(18).toString('base64url') };
  const setup = await api.post('/api/v1/setup', {
    headers: csrf,
    data: {
      email: created.email,
      password: created.password,
      language: 'de',
      library: { name: 'Filme', kind: 'movies', path: join(repo, 'testdata', 'media', 'Movies') },
    },
  });
  if (!setup.ok()) throw new Error(`setup: ${setup.status()} ${await setup.text()}`);
  writeFileSync(accountFile, `${JSON.stringify(created)}\n`, { mode: 0o600 });
  return created;
}

async function start() {
  mkdirSync(dir, { recursive: true });
  const server = startServer();
  try {
    const api = await request.newContext({ baseURL: base });
    await until(
      'server health',
      async () => (await api.get('/api/v1/healthz').catch(() => null))?.ok(),
      30_000,
    );
    const { email, password } = await account(api);
    const login = await api.post('/api/v1/auth/login', {
      headers: csrf,
      data: { email, password },
    });
    if (!login.ok()) throw new Error(`login: ${login.status()}`);
    const itemId = await until(
      'film scanned and probed',
      async () => {
        const issues = await api.get('/api/v1/admin/maintenance/issues?kind=unmatched&limit=200');
        const found = (await issues.json()).items?.find((item) => item.title === title);
        return found?.itemId;
      },
      180_000,
    );
    for (const [name, type] of Object.entries(browsers)) {
      const context = await type.launchPersistentContext(join(dir, name), {
        baseURL: base,
        locale: 'de-CH',
      });
      try {
        const page = context.pages()[0] ?? (await context.newPage());
        const signIn = await context.request.post('/api/v1/auth/login', {
          headers: csrf,
          data: { email, password },
        });
        if (!signIn.ok()) throw new Error(`${name} login: ${signIn.status()}`);
        await page.goto('/');
        await page.waitForFunction(() => navigator.serviceWorker?.controller != null, undefined, {
          timeout: 60_000,
        });
        const created = await context.request.post(`/api/v1/items/${itemId}/downloads`, {
          headers: csrf,
          data: { quality: 'medium' },
        });
        if (created.status() !== 201) throw new Error(`${name} download: ${created.status()}`);
        const [download] = (await created.json()).downloads;
        await page.goto('/downloads');
        const stored = await until(
          `${name} download complete`,
          async () => {
            await page.evaluate(() => window.dispatchEvent(new Event('focus')));
            const all = await storedDownloads(page);
            return all.find((entry) => entry.id === download.id && entry.complete);
          },
          300_000,
        );
        const estimate = await page.evaluate(() => navigator.storage.estimate());
        const persisted = await page.evaluate(() => navigator.storage.persisted());
        log({
          mode: 'start',
          browser: name,
          version: context.browser()?.version() ?? type.name(),
          downloadId: download.id,
          title,
          sizeBytes: stored.manifest?.sizeBytes,
          usage: estimate.usage,
          quota: estimate.quota,
          persisted,
        });
      } finally {
        await context.close();
      }
    }
  } finally {
    server.kill('SIGTERM');
  }
}

async function check() {
  const health = await fetch(`${base}/api/v1/healthz`).catch(() => null);
  if (health) throw new Error(`a server answers on ${base}; stop it so the browsers are offline`);
  const profileName = JSON.parse(readFileSync(accountFile, 'utf8')).email.split('@')[0];
  for (const [name, type] of Object.entries(browsers)) {
    const context = await type.launchPersistentContext(join(dir, name), {
      baseURL: base,
      locale: 'de-CH',
    });
    const entry = { mode: 'check', browser: name };
    try {
      const page = context.pages()[0] ?? (await context.newPage());
      const started = Date.now();
      await page.goto('/');
      await page
        .getByRole('heading', { level: 1, name: 'Offline-Bibliothek' })
        .waitFor({ timeout: 15_000 });
      entry.coldStartMs = Date.now() - started;
      const downloads = await storedDownloads(page);
      entry.downloads = downloads.map((d) => ({
        id: d.id,
        complete: d.complete,
        bytes: d.bytesWritten,
      }));
      await page.getByRole('button', { name: new RegExp(profileName) }).click();
      await page
        .getByRole('list', { name: 'Heruntergeladene Titel' })
        .getByRole('link', { name: new RegExp(title) })
        .click();
      await page.getByRole('main', { name: 'Videoplayer' }).waitFor({ timeout: 15_000 });
      const video = page.locator('video');
      await video.evaluate((element) => element.play());
      await page.waitForFunction(
        () => (document.querySelector('video')?.currentTime ?? 0) > 1,
        undefined,
        {
          timeout: 20_000,
        },
      );
      entry.seekMs = await video.evaluate(
        (element) =>
          new Promise((resolve) => {
            const from = performance.now();
            element.addEventListener(
              'seeked',
              () => resolve(Math.round(performance.now() - from)),
              { once: true },
            );
            element.currentTime = element.duration / 2;
          }),
      );
      // Passed: the offline library within 3 s (C-1), the download complete with all its bytes,
      // the jump to 50 % within 2 s (plan 10.4).
      const complete =
        entry.downloads.length > 0 && entry.downloads.every((d) => d.complete && d.bytes > 0);
      entry.result =
        entry.coldStartMs < 3000 && complete && entry.seekMs < 2000
          ? 'bestanden'
          : 'nicht bestanden';
    } catch (error) {
      entry.result = 'nicht bestanden';
      entry.error = String(error);
    } finally {
      await context.close();
    }
    log(entry);
  }
}

function storedDownloads(page) {
  return page.evaluate(async () => {
    const known = await indexedDB.databases();
    if (!known.some((entry) => entry.name === 'slouchcat-offline')) return [];
    return new Promise((resolve, reject) => {
      const open = indexedDB.open('slouchcat-offline');
      open.onerror = () => reject(open.error);
      open.onsuccess = () => {
        const db = open.result;
        const read = db.transaction('downloads').objectStore('downloads').getAll();
        read.onerror = () => reject(read.error);
        read.onsuccess = () => {
          db.close();
          resolve(read.result);
        };
      };
    });
  });
}

const mode = process.argv[2];
if (mode === 'start') await start();
else if (mode === 'check') await check();
else {
  console.error('usage: node web/scripts/o6-longterm.mjs start|check');
  process.exit(2);
}
