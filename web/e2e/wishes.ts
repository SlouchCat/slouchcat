import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { SEERR_BASE } from './env';
import { expect } from './fixtures';
import { connectSeerr, disconnectSeerr } from './seerr';
import { TMDB_API_KEY } from './tmdb-server.mjs';

// The wishes of concept 25 stage 2 for the viewer specs: TMDB (the mock of tmdb-server.mjs) for the
// titles outside the library, and Seerr (seerr-server.mjs) to send the wishes to.
//
// How the TMDB mock is switched on and off: start-server.mjs always points the server's TMDB
// addresses at the mock (SLOUCHCAT_TEST_TMDB_BASE_URL, --dev only), but the run has no TMDB key,
// and without one the server never asks TMDB, so every other spec runs as before (titles from
// the file names). `openWishes` stores the key TMDB_API_KEY through the administration, like an
// administrator would, and `closeWishes` clears it again. While it is stored the mock answers
// only its own made-up titles (ids from 9000 up) and «nothing found» or 404 for every other
// search and id, so the library's titles stay unmatched as without a key.

/** The owner's account as the specs use it, with a Seerr user that needs an approval. */
export const SEERR_USER_PENDING = 3;

async function send(api: APIRequestContext, method: 'put' | 'post', path: string, data?: object) {
  const response = await api[method](`/api/v1${path}`, { headers: CSRF, data });
  expect(response.ok(), `${path}: ${await response.text()}`).toBe(true);
  return response;
}

/** What `openWishes` changed, for `closeWishes`. */
export interface WishesSetup {
  settings: Record<string, unknown>;
  accountId: number;
}

/**
 * TMDB key, Seerr connected with the wishes on, the owner's account linked to the Seerr user
 * `seerrUserId` (one that needs an approval by default) with `limit` (films and seasons, 0 = none).
 */
export async function openWishes(
  api: APIRequestContext,
  { seerrUserId = SEERR_USER_PENDING, limit = 0 }: { seerrUserId?: number; limit?: number } = {},
): Promise<WishesSetup> {
  await send(api, 'put', '/admin/settings', { keys: { tmdb: TMDB_API_KEY } });
  const settings = await connectSeerr(api, { enabled: true });
  const session = (await (await api.get('/api/v1/auth/session')).json()) as {
    account: { id: number };
  };
  const accountId = session.account.id;
  // The server knows Seerr's users only once it read them (a job): read them and wait.
  await send(api, 'post', '/admin/wishes/mappings/users/refresh');
  await expect
    .poll(async () => {
      const users = (await (await api.get('/api/v1/admin/wishes/mappings/users')).json()) as {
        id: number;
      }[];
      return users.some((user) => user.id === seerrUserId);
    })
    .toBe(true);
  await send(api, 'put', `/admin/wishes/mappings/accounts/${accountId}`, { seerrUserId });
  await send(
    api,
    'put',
    `/admin/accounts/${accountId}/wish-limit`,
    limit > 0
      ? { mode: 'custom', movie: { limit, days: 7 }, tv: { limit, days: 7 } }
      : { mode: 'unlimited' },
  );
  return { settings, accountId };
}

/** Everything back: the wishes' limit, Seerr's connection and requests, the TMDB key. */
export async function closeWishes(api: APIRequestContext, setup: WishesSetup | undefined) {
  // The profile's open wishes go (those not approved yet); disconnectSeerr has Seerr decline the
  // others, of every profile.
  const open = (await (await api.get('/api/v1/wishes?state=open&limit=100')).json()) as {
    items?: { id: number }[];
  };
  for (const wish of open.items ?? []) {
    await api.delete(`/api/v1/wishes/${wish.id}`, { headers: CSRF });
  }
  if (setup) {
    await send(api, 'put', `/admin/accounts/${setup.accountId}/wish-limit`, { mode: 'default' });
  }
  await disconnectSeerr(api, setup?.settings);
  await fetch(`${SEERR_BASE}/__reset`, { method: 'POST' });
  await send(api, 'put', '/admin/settings', { clear: ['tmdb'] });
}

/** Sets a request's status in the Seerr mock (2 approved, 3 declined, 4 failed, 5 completed). */
export async function setSeerrRequest(id: number, status: number) {
  const response = await fetch(`${SEERR_BASE}/__control`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ request: { id, status } }),
  });
  expect(response.ok).toBe(true);
}

/** The requests the Seerr mock holds. */
export async function seerrRequests(): Promise<{ id: number; type: string; mediaId: number }[]> {
  return (await (await fetch(`${SEERR_BASE}/__requests`)).json()) as {
    id: number;
    type: string;
    mediaId: number;
  }[];
}
