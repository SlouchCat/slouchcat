import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { SEERR_BASE } from './env';
import { expect } from './fixtures';
import { SEERR_API_KEY } from './seerr-server.mjs';

// The Seerr mock (seerr-server.mjs) for the specs outside the wishes administration that need
// Seerr set up: connect and test it, switch the wishes on, and leave everything as found.

async function send(api: APIRequestContext, method: 'put' | 'post', path: string, data?: object) {
  const response = await api[method](`/api/v1${path}`, { headers: CSRF, data });
  expect(response.ok(), await response.text()).toBe(true);
  return response;
}

/**
 * Connects the mock (its users as at the start) and tests it; with `enabled` the wishes go on as
 * well. Returns the wish settings before, for `disconnectSeerr`.
 */
export async function connectSeerr(
  api: APIRequestContext,
  { enabled = false }: { enabled?: boolean } = {},
): Promise<Record<string, unknown>> {
  const reset = await fetch(`${SEERR_BASE}/__reset`, { method: 'POST' });
  expect(reset.ok).toBe(true);
  const before = (await (await api.get('/api/v1/admin/wishes/settings')).json()) as Record<
    string,
    unknown
  >;
  await send(api, 'put', '/admin/seerr', { url: SEERR_BASE, apiKey: SEERR_API_KEY, enabled: true });
  await send(api, 'post', '/admin/seerr/test', {});
  if (enabled) await send(api, 'put', '/admin/wishes/settings', { enabled: true });
  return before;
}

/**
 * Closes every open wish of the household, of every profile as the administration lists them. A
 * wish Seerr approved cannot be withdrawn, and without Seerr it would stay open for the rest of the
 * run and turn a later spec's new wish of its title into a join. So the mock declines the wish's
 * requests and the sync closes the wish as declined; a wish still in the outbox is sent by the
 * sync first and declined on a later round.
 */
export async function declineOpenWishes(api: APIRequestContext): Promise<void> {
  const openWishes = async () =>
    (
      (await (await api.get('/api/v1/admin/wishes?state=open&limit=100')).json()) as {
        items?: {
          id: number;
          state: string;
          seerrRequestId?: number;
          seasons: { seerrRequestId?: number }[];
        }[];
      }
    ).items ?? [];
  if ((await openWishes()).length === 0) return;
  await expect(async () => {
    const open = await openWishes();
    for (const wish of open) {
      const ids = [wish.seerrRequestId, ...wish.seasons.map((season) => season.seerrRequestId)];
      for (const id of ids) {
        if (id === undefined) continue;
        await fetch(`${SEERR_BASE}/__control`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ request: { id, status: 3 } }),
        });
      }
    }
    if (open.length > 0) await send(api, 'post', '/admin/seerr/sync');
    expect(open.map((wish) => `${wish.id} ${wish.state}`)).toEqual([]);
  }).toPass({ intervals: [1000], timeout: 30_000 });
}

/**
 * Closes the open wishes (declineOpenWishes), unlinks every account, puts the wish settings back
 * and deletes the connection.
 */
export async function disconnectSeerr(
  api: APIRequestContext,
  settingsBefore: Record<string, unknown> | undefined,
): Promise<void> {
  await declineOpenWishes(api);
  const mappings = (await (await api.get('/api/v1/admin/wishes/mappings')).json()) as {
    accounts?: { accountId: number; seerrUserId?: number }[];
  };
  for (const mapping of mappings.accounts ?? []) {
    if (mapping.seerrUserId !== undefined) {
      await send(api, 'put', `/admin/wishes/mappings/accounts/${mapping.accountId}`, {
        seerrUserId: null,
      });
    }
  }
  if (settingsBefore) await send(api, 'put', '/admin/wishes/settings', settingsBefore);
  await send(api, 'put', '/admin/seerr', { clear: true, enabled: false });
}
