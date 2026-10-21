import { existsSync, mkdirSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { request, type APIRequestContext } from '@playwright/test';
import { M4B_TITLE, MP3_TITLE } from './audio';
import { ensureBaselinePick } from './curation';
import { removeIntroCopies } from './discover';
import {
  AUDIO_IDS,
  AUDIOBOOKS_DIR,
  BASE_URL,
  EXTERNAL_BASE_URL,
  FEED_BASE,
  FEED_URL,
  MOVIES_DIR,
  OWNER,
  STORAGE_STATE,
  TV_DIR,
} from './env';

const SCAN_TIMEOUT_MS = 180_000;
const CSRF = { 'X-Requested-With': 'Slouchcat' };

interface LibraryState {
  id: number;
  name: string;
  kind: string;
  scan: { state: string; lastError?: string };
}

async function listLibraries(api: APIRequestContext): Promise<LibraryState[]> {
  const response = await api.get('/api/v1/admin/libraries');
  if (!response.ok()) throw new Error(`GET /admin/libraries answered ${response.status()}`);
  return (await response.json()) as LibraryState[];
}

async function createLibrary(api: APIRequestContext, data: Record<string, unknown>): Promise<void> {
  const created = await api.post('/api/v1/admin/libraries', { data });
  if (created.status() !== 201 && created.status() !== 202) {
    throw new Error(`POST /admin/libraries answered ${created.status()}: ${await created.text()}`);
  }
}

/**
 * The audio libraries of M6, once per run: "Hörbücher" on testdata/media/Audiobooks and
 * "Podcasts" without a folder, both with the default rating 0 (AN-2).
 */
async function ensureAudioLibraries(api: APIRequestContext): Promise<void> {
  if (!existsSync(AUDIOBOOKS_DIR)) {
    throw new Error(`${AUDIOBOOKS_DIR} is missing: run \`task testdata:media\` first`);
  }
  const libraries = await listLibraries(api);
  if (!libraries.some((library) => library.name === 'Hörbücher')) {
    await createLibrary(api, {
      name: 'Hörbücher',
      kind: 'audiobooks',
      path: AUDIOBOOKS_DIR,
      defaultRating: 0,
    });
  }
  if (!libraries.some((library) => library.name === 'Podcasts')) {
    await createLibrary(api, { name: 'Podcasts', kind: 'podcasts', defaultRating: 0 });
  }
}

/** Subscribes the library "Podcasts" to the local feed (feed-server.mjs) with two episodes (H-2). */
async function ensureFeed(api: APIRequestContext): Promise<void> {
  const library = (await listLibraries(api)).find(
    (entry) => entry.kind === 'podcasts' && entry.name === 'Podcasts',
  );
  if (!library) return;
  const list = await api.get('/api/v1/admin/podcasts');
  if (!list.ok()) throw new Error(`GET /admin/podcasts answered ${list.status()}`);
  const subscribed = (await list.json()) as { feedUrl: string }[];
  if (subscribed.some((podcast) => podcast.feedUrl === FEED_URL)) return;
  const control = await fetch(`${FEED_BASE}/__control`, {
    method: 'POST',
    body: JSON.stringify({ episodes: 2 }),
  }).catch((error: unknown) => {
    // start-server.mjs starts the feed; a server of your own needs `node e2e/feed-server.mjs`.
    const hint = EXTERNAL_BASE_URL ? ' (start `node e2e/feed-server.mjs` next to your server)' : '';
    throw new Error(`feed server on ${FEED_BASE} not reachable${hint}`, { cause: error });
  });
  if (!control.ok) throw new Error(`feed server answered ${control.status}`);
  const created = await api.post('/api/v1/admin/podcasts', {
    data: { libraryId: library.id, feedUrl: FEED_URL },
    timeout: 30_000,
  });
  if (created.status() !== 201) {
    throw new Error(`POST /admin/podcasts answered ${created.status()}: ${await created.text()}`);
  }
}

/**
 * Ids of the two test books, looked up once while they are among the newest titles of the start
 * page (right after their scan), so later titles of the run cannot push them out of the row.
 */
async function storeAudioIds(api: APIRequestContext): Promise<void> {
  const titles = [M4B_TITLE, MP3_TITLE];
  const deadline = Date.now() + SCAN_TIMEOUT_MS;
  for (;;) {
    const response = await api.get('/api/v1/home');
    if (!response.ok()) throw new Error(`GET /home answered ${response.status()}`);
    const home = (await response.json()) as { rows: { items: { id: string; title: string }[] }[] };
    const items = home.rows.flatMap((row) => row.items);
    const ids = Object.fromEntries(
      titles.map((title) => [title, items.find((item) => item.title === title)?.id]),
    );
    if (titles.every((title) => ids[title])) {
      writeFileSync(AUDIO_IDS, JSON.stringify(ids));
      return;
    }
    if (Date.now() > deadline)
      throw new Error(`test audiobooks not on the start page: ${titles.join(', ')}`);
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
}

/**
 * The video libraries, once per run: "Filme" on testdata/media/Movies and the series library of M3
 * on testdata/media/TV.
 */
async function ensureVideoLibraries(api: APIRequestContext): Promise<void> {
  const libraries = await listLibraries(api);
  if (!libraries.some((library) => library.name === 'Filme')) {
    await createLibrary(api, { name: 'Filme', kind: 'movies', path: MOVIES_DIR });
  }
  if (existsSync(TV_DIR) && !libraries.some((library) => library.name === 'Serien')) {
    await createLibrary(api, { name: 'Serien', kind: 'series', path: TV_DIR });
  }
}

async function waitForScan(api: APIRequestContext): Promise<void> {
  const deadline = Date.now() + SCAN_TIMEOUT_MS;
  for (;;) {
    const response = await api.get('/api/v1/admin/libraries');
    if (!response.ok()) throw new Error(`GET /admin/libraries answered ${response.status()}`);
    // A podcast library has no folder to scan: its titles come from the feeds (M6).
    const libraries = ((await response.json()) as LibraryState[]).filter(
      (library) => library.kind !== 'podcasts',
    );
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
 * API (main account, the audiobook library on testdata/media/Audiobooks, no TMDB key), adds the
 * podcast library, subscribes to the local feed and notes the ids of the test books, then adds
 * the video libraries on testdata/media/Movies and testdata/media/TV and waits for their scans.
 * Last, one global pick of the H264 film gives the start page its carousel (M7).
 * The order matters: the start page's "Neu hinzugefügt" holds the 20 newest titles, and the test
 * media have exactly 20 films and series; scanned last, they fill the row in every run (titles of
 * one scan batch share their time, so the row would otherwise drop an arbitrary older title).
 */
export default async function globalSetup(): Promise<void> {
  // Copies of an aborted E-8 run would be scanned as ordinary episodes (notifications.spec).
  removeIntroCopies();
  const api = await request.newContext({ baseURL: BASE_URL, extraHTTPHeaders: CSRF });
  try {
    const status = (await (await api.get('/api/v1/setup')).json()) as { setupRequired: boolean };
    if (status.setupRequired) {
      const setup = await api.post('/api/v1/setup', {
        data: {
          email: OWNER.email,
          password: OWNER.password,
          language: 'de',
          library: {
            name: 'Hörbücher',
            kind: 'audiobooks',
            path: AUDIOBOOKS_DIR,
            defaultRating: 0,
          },
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
    await ensureAudioLibraries(api);
    await waitForScan(api);
    await ensureFeed(api);
    mkdirSync(dirname(STORAGE_STATE), { recursive: true });
    await storeAudioIds(api);
    await ensureVideoLibraries(api);
    await waitForScan(api);
    // Since M7 the start page has a carousel only with picks: one global pick of the H264 film.
    await ensureBaselinePick(api);
    await api.storageState({ path: STORAGE_STATE });
  } finally {
    await api.dispose();
  }
}
