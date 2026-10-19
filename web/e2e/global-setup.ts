import { existsSync, mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import { request, type APIRequestContext } from '@playwright/test';
import { BASE_URL, MOVIES_DIR, OWNER, STORAGE_STATE, TV_DIR } from './env';

const SCAN_TIMEOUT_MS = 180_000;
const CSRF = { 'X-Requested-With': 'Slouchcat' };

interface LibraryState {
  name: string;
  scan: { state: string; lastError?: string };
}

/** The series library of M3 (testdata/media/TV), added next to "Filme" once per run. */
async function ensureSeriesLibrary(api: APIRequestContext): Promise<void> {
  if (!existsSync(TV_DIR)) return;
  const libraries = (await (await api.get('/api/v1/admin/libraries')).json()) as LibraryState[];
  if (libraries.some((library) => library.name === 'Serien')) return;
  const created = await api.post('/api/v1/admin/libraries', {
    data: { name: 'Serien', kind: 'series', path: TV_DIR },
  });
  if (created.status() !== 201 && created.status() !== 202) {
    throw new Error(`POST /admin/libraries answered ${created.status()}: ${await created.text()}`);
  }
}

async function waitForScan(api: APIRequestContext): Promise<void> {
  const deadline = Date.now() + SCAN_TIMEOUT_MS;
  for (;;) {
    const response = await api.get('/api/v1/admin/libraries');
    if (!response.ok()) throw new Error(`GET /admin/libraries answered ${response.status()}`);
    const libraries = (await response.json()) as LibraryState[];
    const states = libraries.map((library) => library.scan.state);
    if (states.length > 0 && states.every((state) => state === 'done')) return;
    const broken = libraries.find((l) => l.scan.state === 'failed' || l.scan.state === 'aborted');
    if (broken)
      throw new Error(`library scan ${broken.scan.state}: ${broken.scan.lastError ?? ''}`);
    if (Date.now() > deadline)
      throw new Error(`scan not done after ${SCAN_TIMEOUT_MS} ms: ${states.join(', ')}`);
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

/**
 * Builds nothing and expects the server from start-server.mjs: runs the setup wizard through the
 * API (main account, library on testdata/media/Movies, no TMDB key), adds the series library on
 * testdata/media/TV, waits for the first scans, logs in and stores the session for the specs.
 */
export default async function globalSetup(): Promise<void> {
  const api = await request.newContext({ baseURL: BASE_URL, extraHTTPHeaders: CSRF });
  try {
    const status = (await (await api.get('/api/v1/setup')).json()) as { setupRequired: boolean };
    if (status.setupRequired) {
      const setup = await api.post('/api/v1/setup', {
        data: {
          email: OWNER.email,
          password: OWNER.password,
          language: 'de',
          library: { name: 'Filme', kind: 'movies', path: MOVIES_DIR },
        },
      });
      if (setup.status() !== 201) {
        throw new Error(`POST /setup answered ${setup.status()}: ${await setup.text()}`);
      }
    }
    const login = await api.post('/api/v1/auth/login', {
      data: { email: OWNER.email, password: OWNER.password },
    });
    if (!login.ok()) throw new Error(`login answered ${login.status()}: ${await login.text()}`);
    await ensureSeriesLibrary(api);
    await waitForScan(api);
    mkdirSync(dirname(STORAGE_STATE), { recursive: true });
    await api.storageState({ path: STORAGE_STATE });
  } finally {
    await api.dispose();
  }
}
