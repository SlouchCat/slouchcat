import { existsSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';
import { CSRF } from './catalog';
import { EXTERNAL_BASE_URL, PORT, TV_DIR } from './env';
import { INTRO_SHOW_TITLE } from './playback';

// Helpers of the M7 viewer specs (C1): watchlist, favourites, notifications and the Trakt mock.
// Picks, collections and the row order come from curation.ts (C2).

/** The Trakt mock (trakt-server.mjs): three below the app's port, like start-server.mjs. */
export const TRAKT_BASE = `http://127.0.0.1:${Number(process.env.SLOUCHCAT_E2E_TRAKT_PORT ?? PORT - 3)}`;

/** Row titles of the start page (home.rows.* in the locales). */
export const ROW_TITLES = {
  spotlight: 'Spotlight',
  picks: "Cat's Empfehlungen",
  continue: 'Weiterschauen',
  recent: 'Neu hinzugefügt',
  released: 'Kürzlich veröffentlicht',
  watchlist: 'Merkliste',
  collections: 'Sammlungen',
  genres: 'Genres',
} as const;

async function expectStatus(response: APIResponse, ok: number[]): Promise<void> {
  expect(ok, `${response.url()} answered ${response.status()}: ${await response.text()}`).toContain(
    response.status(),
  );
}

interface Ref {
  id: string;
  title: string;
}

/** Empties the watchlist of the request context's profile. */
export async function clearWatchlist(api: APIRequestContext): Promise<void> {
  for (let round = 0; round < 20; round += 1) {
    const response = await api.get('/api/v1/watchlist?limit=200');
    await expectStatus(response, [200]);
    const { items } = (await response.json()) as { items: Ref[] };
    if (items.length === 0) return;
    for (const item of items) await setWatchlist(api, item.id, false);
  }
  throw new Error('watchlist not empty after 20 rounds');
}

/** Removes every favourite of the request context's profile. */
export async function clearFavorites(api: APIRequestContext): Promise<void> {
  const response = await api.get('/api/v1/favorites');
  await expectStatus(response, [200]);
  for (const item of (await response.json()) as Ref[]) await setFavorite(api, item.id, false);
}

/** Marks every notification of the request context's profile read (there is no delete). */
export async function readAllNotifications(api: APIRequestContext): Promise<void> {
  const response = await api.post('/api/v1/notifications/read-all', { headers: CSRF });
  await expectStatus(response, [204]);
}

/**
 * The viewer state of M7 for the owner's primary profile: no watchlist entries, no favourites, no
 * unread notifications. Runs before and after every test (fixtures.ts).
 */
export async function resetViewerLists(api: APIRequestContext): Promise<void> {
  await clearWatchlist(api);
  await clearFavorites(api);
  await readAllNotifications(api);
}

/** Puts a title on the watchlist or takes it off (idempotent). */
export async function setWatchlist(api: APIRequestContext, id: string, on: boolean) {
  const response = on
    ? await api.put(`/api/v1/items/${id}/watchlist`, { headers: CSRF })
    : await api.delete(`/api/v1/items/${id}/watchlist`, { headers: CSRF });
  await expectStatus(response, [204]);
}

/** Marks a series or podcast as favourite or not (idempotent). */
export async function setFavorite(api: APIRequestContext, id: string, on: boolean) {
  const response = on
    ? await api.put(`/api/v1/items/${id}/favorite`, { headers: CSRF })
    : await api.delete(`/api/v1/items/${id}/favorite`, { headers: CSRF });
  await expectStatus(response, [204]);
}

/** Watchlist titles of the request context's profile, newest first. */
export async function watchlistTitles(api: APIRequestContext): Promise<string[]> {
  const response = await api.get('/api/v1/watchlist?limit=200');
  await expectStatus(response, [200]);
  return ((await response.json()) as { items: Ref[] }).items.map((item) => item.title);
}

/** The unread count of the request context's profile (the header badge's source). */
export async function unreadNotifications(api: APIRequestContext): Promise<number> {
  const response = await api.get('/api/v1/notifications?limit=1');
  await expectStatus(response, [200]);
  return ((await response.json()) as { unread: number }).unread;
}

/** Lets the Trakt mock answer every list with `status` (503: Trakt is down, I-7). */
export async function traktMockStatus(status: number): Promise<void> {
  let response: Response;
  try {
    response = await fetch(`${TRAKT_BASE}/__control`, {
      method: 'POST',
      body: JSON.stringify({ status }),
    });
  } catch (error) {
    throw new Error(
      `The Trakt mock at ${TRAKT_BASE} does not answer. start-server.mjs starts it; against a ` +
        `server of your own (SLOUCHCAT_E2E_BASE_URL) start e2e/trakt-server.mjs yourself and ` +
        `set SLOUCHCAT_E2E_TRAKT_PORT.`,
      { cause: error },
    );
  }
  expect(response.status, 'trakt mock control').toBe(204);
}

/**
 * The Trakt mock answers normally again (fixtures.ts, before every test), so a test that left it
 * at 503 does not take the Trakt rows from the following ones. Only for the suite's own server:
 * against another one the mock may not run at all.
 */
export async function resetTraktMock(): Promise<void> {
  if (!EXTERNAL_BASE_URL) await traktMockStatus(200);
}

/**
 * "Jetzt abrufen" of the spotlight page (admin API): queues the spotlight fetch, the job runs in
 * the background. The server allows one manual fetch a minute (429 `too_many_requests`, the stamp
 * lives in the database); the suite shares one server across its workers, so a second refresh
 * within that minute, from this test or another, is answered 429. The helper then waits and tries
 * again until the minute is over, instead of relaxing the rule for the test.
 */
export async function refreshSpotlight(api: APIRequestContext): Promise<void> {
  const deadline = Date.now() + 90_000;
  for (;;) {
    const response = await api.post('/api/v1/admin/spotlight/refresh', { headers: CSRF });
    if (response.status() !== 429 || Date.now() > deadline) {
      await expectStatus(response, [202]);
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 5_000));
  }
}

/** Row keys of the start page for the request context's profile. */
export async function homeRowKeys(api: APIRequestContext): Promise<string[]> {
  const response = await api.get('/api/v1/home');
  await expectStatus(response, [200]);
  return ((await response.json()) as { rows: { key: string }[] }).rows.map((row) => row.key);
}

/** The season folder of the intro show (named with or without its TMDB tag), where E-8 copies to. */
export function introSeason(): string {
  const folder = readdirSync(TV_DIR).find((name) => name.startsWith(INTRO_SHOW_TITLE));
  if (!folder) throw new Error(`${INTRO_SHOW_TITLE} is missing in ${TV_DIR}`);
  const season = join(TV_DIR, folder, 'Season 01');
  if (!existsSync(season)) throw new Error(`${season} is missing`);
  return season;
}

/** Copies E-8 writes (S01E10 and up) into the intro show; the test media end at S01E05. */
const E8_COPY = /S01E1\d\.\w+$/;

/**
 * Removes the episode copies of E-8 (notifications.spec.ts) that an aborted run left behind. Runs
 * in the global setup before the series library is scanned: a scanned leftover would be an
 * ordinary episode, and the copy of the next run would count as that file moved (B-9), not as a
 * new episode. Returns the removed file names.
 */
export function removeIntroCopies(): string[] {
  const season = introSeason();
  const copies = readdirSync(season).filter((name) => E8_COPY.test(name));
  for (const name of copies) rmSync(join(season, name), { force: true });
  return copies;
}
