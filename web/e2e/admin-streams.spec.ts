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
  resetProgress,
  type PlaybackData,
} from './playback';

// W-6: two running streams (two browsers) show up in the overview with profile, device, title,
// quality and mode; "Beenden" stops the client within 5 s with the hint, and the row goes away.

const STOP_BUDGET_MS = 5000;

interface AdminStreamData {
  id: string;
  profileName: string;
  itemTitle: string;
  mode: string;
}

const rowOf = (page: Page, profile: string) =>
  page.getByRole('rowheader', { name: new RegExp(`^${profile}`) }).locator('xpath=..');

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
  const other = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
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
    await expect(overview.getByRole('heading', { level: 1, name: 'Streams' })).toBeVisible();
    const table = overview.getByRole('table', { name: 'Laufende Streams' });
    await expect(table).toBeVisible();

    // Earlier specs of the owner may still count as running (two minutes): take this title's row.
    const ownerRow = rowOf(overview, OWNER_PROFILE).filter({ hasText: H264_TITLE }).first();
    await expect(ownerRow).toContainText(H264_TITLE);
    await expect(ownerRow).toContainText(/Direct|Remux|Transcode \((HW|SW)\)/);
    await expect(ownerRow).toContainText(/Chrome|Chromium|Firefox|Edge|Safari|Unbekanntes Gerät/);
    const viewerRow = rowOf(overview, viewerProfile);
    await expect(viewerRow).toContainText(HEVC_LONG_TITLE);
    await expect(viewerRow).toContainText(/Transcode \((HW|SW)\)|Remux/);
    await expect(viewerRow).toContainText(/\d{3,4}p|Original/);
    await expect(viewerRow).toContainText(/Chrome|Chromium|Firefox|Edge|Safari/);
    expect(first.playbackSessionId).not.toBe(second.playbackSessionId);

    // Keyboard: from the top to the viewer's "Beenden", ↑/↓ between the rows, Enter, Enter.
    await resetFocus(overview);
    const stopOwner = overview
      .getByRole('button', { name: `Stream von ${OWNER_PROFILE} beenden` })
      .first();
    const stopViewer = overview.getByRole('button', {
      name: `Stream von ${viewerProfile} beenden`,
    });
    await tabTo(overview, stopOwner.or(stopViewer).first(), 40);
    for (let presses = 0; presses < 10; presses += 1) {
      if (await stopViewer.evaluate((element) => element === document.activeElement)) break;
      await overview.keyboard.press('ArrowDown');
    }
    await expectFocusOn(overview, stopViewer);
    await overview.keyboard.press('Enter');
    const dialog = overview.getByRole('alertdialog', {
      name: `Stream von ${viewerProfile} beenden?`,
    });
    await expect(dialog).toBeVisible();
    await expectVisibleFocus(overview);
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
    const before = Date.now();
    await overview.keyboard.press('Enter');
    expect((await stopped).status()).toBe(204);

    // The viewer's player stops with the hint within 5 s.
    await expect(player(viewer)).toHaveAttribute('data-ended-reason', 'admin', {
      timeout: STOP_BUDGET_MS,
    });
    const stopMs = Date.now() - before;
    expect(stopMs).toBeLessThan(STOP_BUDGET_MS);
    // The 401 names no reason yet: the player says neutrally that the playback ended.
    await expect(viewer.getByRole('alert')).toContainText('Die Wiedergabe wurde beendet.');
    await expect
      .poll(() => viewer.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
      .toBe(true);

    // The row is gone (the page also asks again every 5 s), the other stream stays.
    await expect(overview.getByText(`Stream von ${viewerProfile} beendet.`)).toBeVisible();
    await expect(rowOf(overview, viewerProfile)).toHaveCount(0);
    await expect(rowOf(overview, OWNER_PROFILE).filter({ hasText: H264_TITLE })).not.toHaveCount(0);
    const list = (await (await owner.get('/api/v1/admin/streams')).json()) as AdminStreamData[];
    expect(list.map((stream) => stream.id)).not.toContain(second.playbackSessionId);
    await expectNoFocusTrap(overview);
    console.log(JSON.stringify({ measurement: 'admin-stop', stopMs }));
  } finally {
    await admin.close();
    await other.close();
  }
});
