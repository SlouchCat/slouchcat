import { loginOwner } from './accounts';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus } from './keyboard';
import {
  arrowTo,
  H264_TITLE,
  HDR_TITLE,
  itemIdByTitle,
  play,
  player,
  resetProgress,
  startOf,
  useFreshProfile,
} from './playback';
import { CSRF } from './catalog';

// W-8: manual quality switches the variant; a profile limit of 720p keeps the output at or below
// 720p; a bandwidth limit of 4000 kbit/s keeps the bit rate at or below 4 Mbit/s; one stream per
// profile turns the second start away with 409 and a hint. The server decides; the specs check
// its answers and what the player shows.

test('choosing 480p by keyboard starts a new session with a 480p transcode', async ({
  page,
  owner,
}) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const first = await play(page, itemId);
  await expect(page.getByRole('button', { name: /^Qualität: Automatisch/ })).toBeVisible();

  await page.keyboard.press('q');
  const menu = page.getByRole('menu', { name: /^Qualität/ });
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await expect(menu.getByRole('menuitemradio', { name: /^Automatisch/ })).toHaveAttribute(
    'aria-checked',
    'true',
  );
  await arrowTo(page, menu.getByRole('menuitemradio', { name: '480p' }));
  const second = await startOf(page, () => page.keyboard.press('Enter'));
  expect(second.status).toBe(200);
  expect(second.request.quality).toBe('480p');
  expect(second.body.playbackSessionId).not.toBe(first.playbackSessionId);
  expect(second.body.mode).toBe('hls');
  expect(second.body.transcoding?.mode).toBe('transcode');
  expect(second.body.transcoding?.height).toBe(480);
  expect(second.body.transcoding?.reason).toBe('quality');

  await expect(player(page)).toHaveAttribute(
    'data-playback-session-id',
    second.body.playbackSessionId,
  );
  await expect(player(page)).toHaveAttribute('data-switch-ms', /\d+/, { timeout: 15_000 });
  await expect(page.getByRole('button', { name: 'Qualität: 480p' })).toBeVisible();
  // Escape closes a menu and leaves the focus on its button.
  await page.keyboard.press('q');
  await expect(menu).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);
});

test('a profile limit of 720p: automatic quality plays at most 720p', async ({ page, owner }) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  await useFreshProfile(page, owner, 'E2E 720p', { maxHeight: 720 });
  const info = await play(page, itemId);
  // The 1080p source is above the limit: transcoded to the 720p rung.
  expect(info.mode).toBe('hls');
  expect(info.transcoding?.height ?? 0).toBeLessThanOrEqual(720);
  expect(info.transcoding?.reason).toBe('max_height');
  const rung1080 = info.qualities.find((option) => option.quality === '1080p');
  expect(rung1080?.available ?? false).toBe(false);

  await page.keyboard.press('q');
  const menu = page.getByRole('menu', { name: /^Qualität/ });
  await expect(menu.getByRole('menuitemradio', { name: '1080p' })).toHaveAttribute(
    'aria-disabled',
    'true',
  );
  await page.keyboard.press('Escape');
});

test('a bandwidth limit of 4000 kbit/s keeps the bit rate at or below 4 Mbit/s', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  // The 4K HDR clip is transcoded anyway; without a limit its rung would be 1080p at 8 Mbit/s.
  const itemId = await itemIdByTitle(owner, HDR_TITLE);
  await useFreshProfile(page, owner, 'E2E 4 Mbit', { maxBitrateKbps: 4000 });
  const info = await play(page, itemId);
  expect(info.mode).toBe('hls');
  expect(info.transcoding?.bitrate ?? Infinity).toBeLessThanOrEqual(4_000_000);
  const rung1080 = info.qualities.find((option) => option.quality === '1080p');
  expect(rung1080?.available ?? false).toBe(false);
});

test('one stream per profile: a second start answers 409 and the page shows the hint', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  const profile = await useFreshProfile(page, owner, 'E2E 1 Stream', { maxStreams: 1 });
  await play(page, itemId);

  // The same profile in another browser.
  const other = await browser.newContext({ baseURL, locale: 'de-CH', extraHTTPHeaders });
  try {
    await loginOwner(other.request);
    const selected = await other.request.post(`/api/v1/auth/profiles/${profile.id}/select`, {
      headers: CSRF,
    });
    expect(selected.status()).toBe(200);
    const second = await other.newPage();
    const start = await startOf(second, () => second.goto(`/play/${itemId}`));
    expect(start.status).toBe(409);
    expect(start.body.code).toBe('stream_limit_reached');
    expect(start.body.extensions?.maxStreams).toBe(1);
    await expect(second.getByText('Zu viele gleichzeitige Streams')).toBeVisible();
    await expect(second.getByText(/darf 1 Stream gleichzeitig abspielen/)).toBeVisible();
    await expect(second.locator('main')).toHaveAttribute('data-error-code', 'stream_limit_reached');
    // The hint page keeps the keyboard rules: the back button has the focus, Escape leaves.
    await expectFocusOn(second, second.getByRole('button', { name: 'Zurück' }));
    await second.keyboard.press('Escape');
    await expect(second).toHaveURL(new RegExp(`/items/${itemId}$`));
  } finally {
    await other.close();
  }
});
