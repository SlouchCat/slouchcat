import { expect, request, test, type APIRequestContext, type Page } from '@playwright/test';
import { CSRF, H264_MKV_LONG_TITLE, H264_TITLE, itemPath } from './catalog';
import { BASE_URL } from './env';

/**
 * Stream tokens (plan 5.4, DoD M1b): playlists, segments and the Range stream work with `st`
 * alone (no cookie), answer 401 without it, and 401 within 1 s after the playback session ended.
 */
const REVOCATION_BUDGET_MS = 1000;
const CAPABILITIES = { video: ['h264'], audio: ['aac'], hls: 'mse' };

interface PlaybackInfo {
  mode: string;
  streamUrl: string;
  playbackSessionId: string;
}

async function startPlayback(page: Page, title: string): Promise<PlaybackInfo> {
  const itemId = (await itemPath(page, title)).split('/').pop() ?? '';
  const response = await page.request.post(`/api/v1/items/${itemId}/play`, {
    headers: CSRF,
    data: { capabilities: CAPABILITIES },
  });
  expect(response.status()).toBe(200);
  return (await response.json()) as PlaybackInfo;
}

/** A request context without the session cookie: only the token authorises. */
let anonymous: APIRequestContext;
test.beforeAll(async () => {
  anonymous = await request.newContext({ baseURL: BASE_URL, storageState: undefined });
});
test.afterAll(async () => {
  await anonymous.dispose();
});

const withoutToken = (url: string) => {
  const parsed = new URL(url, BASE_URL);
  parsed.searchParams.delete('st');
  return parsed.pathname + parsed.search;
};

test('HLS playlists and segments: token only, 401 without, 401 within 1 s after stop', async ({
  page,
  browserName,
}) => {
  const info = await startPlayback(page, H264_MKV_LONG_TITLE);
  expect(info.mode).toBe('hls');
  const st = new URL(info.streamUrl, BASE_URL).searchParams.get('st') ?? '';
  const base = `/api/v1/playback/sessions/${info.playbackSessionId}`;
  const media = `${base}/media.m3u8?st=${encodeURIComponent(st)}`;

  const master = await anonymous.get(info.streamUrl);
  expect(master.status()).toBe(200);
  expect(master.headers()['content-type']).toBe('application/vnd.apple.mpegurl');
  expect(master.headers()['referrer-policy']).toBe('no-referrer');
  expect(await master.text()).toContain(`media.m3u8?st=${encodeURIComponent(st)}`);
  const playlist = await anonymous.get(media);
  expect(playlist.status()).toBe(200);
  expect(await playlist.text()).toContain('#EXT-X-MAP:URI="init.mp4?st=');
  for (const part of ['init.mp4', 'seg/0']) {
    const response = await anonymous.get(`${base}/${part}?st=${encodeURIComponent(st)}`);
    expect(response.status(), part).toBe(200);
    expect(response.headers()['content-type']).toBe('video/mp4');
  }

  // Without the token: 401, also with the session cookie.
  for (const context of [anonymous, page.request]) {
    const response = await context.get(withoutToken(media));
    expect(response.status()).toBe(401);
    expect(((await response.json()) as { code: string }).code).toBe('unauthenticated');
  }

  // Stop: the same URL answers 401 within 1 s.
  const stop = await page.request.delete(base, { headers: CSRF });
  expect(stop.status()).toBe(204);
  const stoppedAt = Date.now();
  const revoked = await anonymous.get(media);
  const latencyMs = Date.now() - stoppedAt;
  expect(revoked.status()).toBe(401);
  expect(latencyMs).toBeLessThan(REVOCATION_BUDGET_MS);
  const segment = await anonymous.get(`${base}/seg/1?st=${encodeURIComponent(st)}`);
  expect(segment.status()).toBe(401);
  // Measurement output for docs/perf/M1b.md.
  console.log(JSON.stringify({ measurement: 'revocation', browser: browserName, latencyMs }));
});

test('direct play: the Range stream needs the token', async ({ page }) => {
  const info = await startPlayback(page, H264_TITLE);
  expect(info.mode).toBe('direct');
  const ranged = await anonymous.get(info.streamUrl, { headers: { Range: 'bytes=0-99' } });
  expect(ranged.status()).toBe(206);
  expect((await ranged.body()).length).toBe(100);
  expect(ranged.headers()['referrer-policy']).toBe('no-referrer');

  for (const context of [anonymous, page.request]) {
    const response = await context.get(withoutToken(info.streamUrl), {
      headers: { Range: 'bytes=0-99' },
    });
    expect(response.status()).toBe(401);
  }

  const stop = await page.request.delete(`/api/v1/playback/sessions/${info.playbackSessionId}`, {
    headers: CSRF,
  });
  expect(stop.status()).toBe(204);
  const after = await anonymous.get(info.streamUrl, { headers: { Range: 'bytes=0-99' } });
  expect(after.status()).toBe(401);
});
