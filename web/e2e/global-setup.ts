import { mkdirSync } from 'node:fs';
import { dirname } from 'node:path';
import { request, type APIRequestContext } from '@playwright/test';
import { BASE_URL, MOVIES_DIR, OWNER, STORAGE_STATE } from './env';

const SCAN_TIMEOUT_MS = 180_000;
const CSRF = { 'X-Requested-With': 'Slouchcat' };

interface LibraryState {
  scan: { state: string; lastError?: string };
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
 * API (main account, library on testdata/media/Movies, no TMDB key), waits for the first scan,
 * logs in and stores the session for the specs.
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
    await waitForScan(api);
    mkdirSync(dirname(STORAGE_STATE), { recursive: true });
    await api.storageState({ path: STORAGE_STATE });
  } finally {
    await api.dispose();
  }
}
