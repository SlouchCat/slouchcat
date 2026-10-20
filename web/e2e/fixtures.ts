import { test as base, expect, type APIRequestContext } from '@playwright/test';
import { ownerApi, removeGuests, resetProfiles, stopAllStreams } from './accounts';
import { removeDeviceDownloads } from './offline';

/**
 * A client address of its own for every test (10.x.y.z from the test's name and browser). The
 * server limits login, PIN and invitation attempts to 10 per minute and address; with the e2e
 * server trusting `X-Forwarded-For` (start-server.mjs), tests no longer use up each other's budget.
 */
function clientAddress(key: string): string {
  let hash = 2166136261;
  for (const char of key) hash = Math.imul(hash ^ char.charCodeAt(0), 16777619) >>> 0;
  return `10.${(hash >>> 16) & 255}.${(hash >>> 8) & 255}.${hash & 255 || 1}`;
}

// `extraHTTPHeaders`: the test's client address for the page, `page.request` and new contexts that
// pass it on.
// `owner`: an API client with the shared session of the main account (primary profile, never
// switched), for setting up and tidying data. It needs no login. It runs for every test (`auto`):
// before and after each one the account is left with its primary profile only, without guests,
// without running streams and without downloads on the shared session's device.
export const test = base.extend<{ owner: APIRequestContext }>({
  extraHTTPHeaders: async ({ browserName }, provide, testInfo) => {
    const address = clientAddress(`${browserName}/${testInfo.titlePath.join('/')}`);
    await provide({ 'X-Forwarded-For': address });
  },
  owner: [
    async ({ playwright, baseURL, extraHTTPHeaders }, provide) => {
      if (!baseURL) throw new Error('baseURL is not set');
      const api = await ownerApi(playwright, baseURL, extraHTTPHeaders);
      try {
        await stopAllStreams(api);
        await removeDeviceDownloads(api);
        await resetProfiles(api);
        await removeGuests(api);
        await provide(api);
      } finally {
        await stopAllStreams(api);
        // Downloads of the shared device would come back into every later spec's browser (M5).
        await removeDeviceDownloads(api);
        await resetProfiles(api);
        await removeGuests(api);
        await api.dispose();
      }
    },
    { auto: true },
  ],
});

export { expect };
