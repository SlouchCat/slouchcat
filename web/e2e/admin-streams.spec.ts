import type { Page } from '@playwright/test';
import { loginOwner } from './accounts';
import { CSRF, HEVC_LONG_TITLE } from './catalog';
import { OWNER_PROFILE, STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import {
  H264_TITLE,
  itemIdByTitle,
  limitCapabilities,
  play,
  player,
  recordEndedAt,
  resetProgress,
  watchStopAnswers,
  type PlaybackData,
} from './playback';

// W-6: two running streams (two browsers) show up as tiles (concept 12, mockup Str-D2) with
// profile, device, title, quality and mode, counted under «Serverlast»; "Beenden" stops the client
// within 5 s with the hint, the tile goes away and the h1 takes the focus.
//
// Concept 13, 11.13: the viewer's 90 s title is fully buffered when the administration ends it, so
// no media request can answer 401; the player learns of the stop from its stop watch
// (getPlaybackSessionState every 4 s) within the same 5 s, also when the stop comes right after
// an answer of the stop watch (the worst moment). With the title partly buffered and the stop
// watch switched off, the 401 of the next segment request stops it within 5 s as well.

const STOP_BUDGET_MS = 5000;

/** The viewer's buffer reaches the end of the title (hls.js loads all 90 s within seconds). */
const FULL_BUFFER_MS = 60_000;

/**
 * Segments of the 401 variant arrive this much later, like over a slow line: the title stays
 * partly buffered until the stop, so a segment request follows within the budget.
 */
const SEGMENT_DELAY_MS = 1000;

interface AdminStreamData {
  id: string;
  profileName: string;
  itemTitle: string;
  mode: string;
}

/** The tiles of a profile: articles with its «Beenden». */
const tileOf = (page: Page, profile: string) =>
  page
    .getByRole('region', { name: 'Laufende Streams' })
    .getByRole('article')
    .filter({ has: page.getByRole('button', { name: `Stream von ${profile} beenden` }) });

/** The number of a counter of «Serverlast». */
async function loadCount(page: Page, label: string): Promise<number> {
  const tile = page
    .getByRole('group', { name: 'Serverlast' })
    .locator('dl')
    .filter({ has: page.getByText(label, { exact: true }) });
  return Number(await tile.locator('dd').first().textContent());
}

test('two streams listed; the admin ends one, its player stops within 5 s', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(120_000);
  // Stream 1: the owner's primary profile in this browser (direct play).
  const h264 = await itemIdByTitle(owner, H264_TITLE);
  await resetProgress(page.request, h264, 10_000);
  const first = await play(page, h264);
  await page.locator('video').evaluate((video: HTMLVideoElement) => {
    video.loop = true;
  });

  // Stream 2: another profile in another browser (HEVC: transcode in Chromium and Firefox).
  const viewerProfile = 'E2E Zuschauer';
  const created = await owner.post('/api/v1/profiles', {
    headers: CSRF,
    data: { name: viewerProfile },
  });
  expect(created.status()).toBe(201);
  const { id: profileId } = (await created.json()) as { id: number };
  // Without the service worker, so the page sees the player's requests itself.
  const other = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    serviceWorkers: 'block',
  });
  const admin = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    await loginOwner(other.request);
    expect(
      (
        await other.request.post(`/api/v1/auth/profiles/${profileId}/select`, { headers: CSRF })
      ).status(),
    ).toBe(200);
    const viewer = await other.newPage();
    // Without HEVC and MKV in the profile the viewer's stream is a transcode in both browsers.
    await limitCapabilities(viewer, { video: ['h264'], containers: ['mp4'] });
    const hevc = await itemIdByTitle(owner, HEVC_LONG_TITLE);
    const second: PlaybackData = await play(viewer, hevc);

    // The overview, in a third window of the owner.
    const overview = await admin.newPage();
    await overview.goto('/admin/streams');
    const heading = overview.getByRole('heading', { level: 1, name: 'Streams' });
    await expect(heading).toBeVisible();
    await expect(overview.getByText('Live, alle 5 Sekunden')).toBeVisible();

    // Earlier specs of the owner may still count as running (two minutes): take this title's tile.
    const ownerRow = tileOf(overview, OWNER_PROFILE).filter({ hasText: H264_TITLE }).first();
    await expect(ownerRow).toContainText(H264_TITLE);
    await expect(ownerRow).toContainText(/Direct|Remux|Transcode \((HW|SW)\)/);
    await expect(ownerRow).toContainText(/Chrome|Chromium|Firefox|Edge|Safari|Unbekanntes Gerät/);
    const viewerRow = tileOf(overview, viewerProfile);
    await expect(viewerRow).toContainText(HEVC_LONG_TITLE);
    await expect(viewerRow).toContainText(/Transcode \((HW|SW)\)|Remux/);
    await expect(viewerRow).toContainText(/\d{3,4}p|Original/);
    await expect(viewerRow).toContainText(/Chrome|Chromium|Firefox|Edge|Safari/);
    // The reason of the transcode is a line of the tile, in the words of the player's stream
    // info (concept 14, 7.1); the label is for screen readers.
    expect(second.transcoding?.reason).toBeTruthy();
    const reason = viewerRow
      .locator('dl > div')
      .filter({ has: overview.locator('dt', { hasText: 'Grund' }) });
    await expect(reason).toHaveCount(1);
    await expect(reason.locator('dd')).not.toBeEmpty();
    await expect(reason.locator('dd')).not.toHaveText(second.transcoding?.reason ?? '');
    expect(first.playbackSessionId).not.toBe(second.playbackSessionId);

    // «Serverlast»: the four counters hold at least these two streams, the bandwidth is a rate.
    await expect(overview.getByRole('heading', { level: 2, name: 'Serverlast' })).toBeVisible();
    const counted = await Promise.all(
      ['Transcode HW', 'Transcode SW', 'Remux', 'Direct'].map((label) =>
        loadCount(overview, label),
      ),
    );
    expect(counted.reduce((sum, count) => sum + count, 0)).toBeGreaterThanOrEqual(2);
    await expect(
      overview
        .getByRole('group', { name: 'Serverlast' })
        .locator('dl')
        .filter({ hasText: 'Bandbreite gesamt' }),
    ).toContainText(/Mb(it)?\/s/);

    // Keyboard: Tab to the first tile's "Beenden"; → to the next tile and ← back, then → on to the
    // viewer's tile (reading order); Enter, Enter.
    await resetFocus(overview);
    const stops = overview
      .getByRole('region', { name: 'Laufende Streams' })
      .getByRole('button', { name: /^Stream von .+ beenden$/ });
    const stopViewer = overview.getByRole('button', {
      name: `Stream von ${viewerProfile} beenden`,
    });
    await tabTo(overview, stops.first(), 40);
    await overview.keyboard.press('ArrowRight');
    await expectFocusOn(overview, stops.nth(1));
    await overview.keyboard.press('ArrowLeft');
    await expectFocusOn(overview, stops.first());
    for (let presses = 0; presses < 12; presses += 1) {
      if (await stopViewer.evaluate((element) => element === document.activeElement)) break;
      await overview.keyboard.press('ArrowRight');
    }
    await expectFocusOn(overview, stopViewer);
    await overview.keyboard.press('Enter');
    const dialog = overview.getByRole('alertdialog', {
      name: `Stream von ${viewerProfile} beenden?`,
    });
    await expect(dialog).toBeVisible();
    await expectVisibleFocus(overview);
    // The poll waits while the confirmation is open: the header no longer says «Live».
    await expect(overview.getByText('Live, alle 5 Sekunden')).toHaveCount(0);
    // Escape cancels and returns the focus; then for real.
    await overview.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(overview, stopViewer);
    await overview.keyboard.press('Enter');
    await expect(dialog.getByRole('button', { name: 'Stream beenden' })).toBeFocused();
    const stopped = overview.waitForResponse(
      (response) =>
        response.request().method() === 'DELETE' &&
        response.url().endsWith(`/admin/streams/${second.playbackSessionId}`),
    );
    // The stop has to find the viewer playing with the whole title buffered: no media request
    // follows that could answer 401.
    const bufferOf = () =>
      viewer.locator('video').evaluate((video: HTMLVideoElement) => ({
        end: video.buffered.length > 0 ? video.buffered.end(video.buffered.length - 1) : 0,
        duration: video.duration,
        paused: video.paused,
      }));
    await expect
      .poll(
        async () => {
          const { end, duration } = await bufferOf();
          return duration > 0 && end >= duration - 2;
        },
        { timeout: FULL_BUFFER_MS },
      )
      .toBe(true);
    const position = await viewer
      .locator('video')
      .evaluate((video: HTMLVideoElement) => video.currentTime);
    const buffered = { ...(await bufferOf()), position };
    expect(buffered.paused, JSON.stringify(buffered)).toBe(false);
    expect(
      position,
      `the stop must come at least 10 s before the end: ${JSON.stringify(buffered)}`,
    ).toBeLessThan(buffered.duration - 10);
    const unauthorized: string[] = [];
    viewer.on('response', (response) => {
      if (response.status() === 401) unauthorized.push(response.url());
    });
    const answers = await watchStopAnswers(viewer, second.playbackSessionId);
    const watched = answers.next('stopped');
    const endedAt = await recordEndedAt(viewer);
    // The worst moment: right after an answer of the stop watch, the next comes 4 s later.
    await answers.next('active');
    const before = Date.now();
    await overview.keyboard.press('Enter');
    expect((await stopped).status()).toBe(204);

    // The viewer's player stops with the hint within 5 s.
    await expect(player(viewer)).toHaveAttribute('data-ended-reason', 'admin', {
      timeout: STOP_BUDGET_MS,
    });
    const stopMs = (await endedAt())! - before;
    expect(stopMs).toBeLessThan(STOP_BUDGET_MS);
    // The stop watch told it (stopped_by admin), not a 401 of a media request.
    expect((await watched).stoppedBy).toBe('admin');
    expect(unauthorized).toEqual([]);
    await expect(viewer.getByRole('alert')).toContainText('Die Wiedergabe wurde beendet.');
    await expect
      .poll(() => viewer.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
      .toBe(true);

    // The tile is gone (the page also asks again every 5 s), the other stream stays; its button
    // went with it, so the h1 has the focus.
    await expect(overview.getByText(`Stream von ${viewerProfile} beendet.`)).toBeVisible();
    await expect(tileOf(overview, viewerProfile)).toHaveCount(0);
    await expect(tileOf(overview, OWNER_PROFILE).filter({ hasText: H264_TITLE })).not.toHaveCount(
      0,
    );
    await expect(heading).toBeFocused();
    const list = (await (await owner.get('/api/v1/admin/streams')).json()) as AdminStreamData[];
    expect(list.map((stream) => stream.id)).not.toContain(second.playbackSessionId);
    await expectNoFocusTrap(overview);
    console.log(JSON.stringify({ measurement: 'admin-stop', stopMs, buffered }));

    // Back (rule 8): the sidebar leads to «Geräte», the browser's Back returns to the overview.
    await resetFocus(overview);
    await tabTo(
      overview,
      overview
        .getByRole('navigation', { name: 'Verwaltung' })
        .getByRole('link', { name: /^Streams(,|$)/ }),
    );
    await overview.keyboard.press('ArrowUp');
    await overview.keyboard.press('Enter');
    await expect(overview).toHaveURL(/\/admin\/devices$/);
    await overview.goBack();
    await expect(overview).toHaveURL(/\/admin\/streams$/);
    await expect(heading).toBeVisible();
    await expect(tileOf(overview, OWNER_PROFILE).filter({ hasText: H264_TITLE })).not.toHaveCount(
      0,
    );
  } finally {
    await admin.close();
    await other.close();
  }
});

test('partly buffered and without the stop watch: the 401 of a segment stops the player within 5 s', async ({
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(60_000);
  // Without the service worker, so the segment and state requests pass page.route.
  const context = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
    serviceWorkers: 'block',
  });
  try {
    const page = await context.newPage();
    await limitCapabilities(page, { video: ['h264'], containers: ['mp4'] });
    await page.route(/\/api\/v1\/playback\/sessions\/[^/]+\/seg\/\d+/, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, SEGMENT_DELAY_MS));
      // The context may be closed meanwhile (end of the test).
      await route.continue().catch(() => undefined);
    });
    // Only the 401 of a media request is left to tell the player.
    const watches: string[] = [];
    await page.route(/\/api\/v1\/playback\/sessions\/[^/]+\/state$/, async (route) => {
      watches.push(route.request().url());
      await route.abort();
    });
    const hevc = await itemIdByTitle(owner, HEVC_LONG_TITLE);
    await resetProgress(page.request, hevc, 90_000);
    const playback = await play(page, hevc);
    const buffered = await page.locator('video').evaluate((video: HTMLVideoElement) => ({
      end: video.buffered.length > 0 ? video.buffered.end(video.buffered.length - 1) : 0,
      duration: video.duration,
    }));
    expect(buffered.end, JSON.stringify(buffered)).toBeLessThan(buffered.duration - 4);
    const revoked = page.waitForResponse(
      (response) => response.status() === 401 && /\/seg\/\d+/.test(response.url()),
    );
    const endedAt = await recordEndedAt(page);
    const before = Date.now();
    const stop = await owner.delete(`/api/v1/admin/streams/${playback.playbackSessionId}`, {
      headers: CSRF,
    });
    expect(stop.status()).toBe(204);
    await expect(player(page)).toHaveAttribute('data-ended-reason', 'admin', {
      timeout: STOP_BUDGET_MS,
    });
    const stopMs = (await endedAt())! - before;
    expect(stopMs).toBeLessThan(STOP_BUDGET_MS);
    await revoked;
    expect(watches.length, 'the stop watch asked (and was cut off)').toBeGreaterThan(0);
    await expect(page.getByRole('alert')).toContainText('Die Wiedergabe wurde beendet.');
    console.log(JSON.stringify({ measurement: 'admin-stop-401', stopMs, buffered }));
  } finally {
    await context.close();
  }
});
