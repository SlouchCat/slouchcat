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

/** Unlinks every account, puts the wish settings back and deletes the connection. */
export async function disconnectSeerr(
  api: APIRequestContext,
  settingsBefore: Record<string, unknown> | undefined,
): Promise<void> {
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
