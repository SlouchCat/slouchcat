import { expect, test, type Page } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectVisibleFocus, resetFocus, tabTo } from './keyboard';

// B-5: two files of one film ("… (2024).mp4" in 1080p and "… (2024) - 720p.mp4") are one title
// with two versions; the chosen version is what the player streams (Stream-Info shows its id).

let detail = '';

const versions = (page: Page) => page.getByRole('group', { name: 'Versionen' });
const player = (page: Page) => page.getByRole('main', { name: 'Videoplayer' });

/** The media file id of the playback the server started (PlaybackInfo.mediaFile.id). */
async function startedFile(page: Page, start: () => Promise<void>): Promise<string> {
  const response = page.waitForResponse(
    (candidate) =>
      candidate.request().method() === 'POST' && /\/items\/[^/]+\/play$/.test(candidate.url()),
  );
  await start();
  const body = (await (await response).json()) as { mediaFile: { id: string } };
  return body.mediaFile.id;
}

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  await resetFocus(page);
});

test('one title with two versions; the start page lists it once', async ({ page }) => {
  await expect(versions(page).getByRole('button')).toHaveCount(2);
  await expect(versions(page).getByRole('button', { name: '1080p' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(versions(page).getByRole('button', { name: '720p' })).toBeVisible();
  await page.goto('/');
  const recent = page.getByRole('region', { name: 'Neu hinzugefügt' });
  await expect(
    recent.getByRole('link').filter({ has: page.getByText(H264_TITLE, { exact: true }) }),
  ).toHaveCount(1);
});

test('choose 720p by keyboard → the player streams that file (Stream-Info)', async ({ page }) => {
  const sd = versions(page).getByRole('button', { name: '720p' });
  await tabTo(page, sd);
  await page.keyboard.press('Enter');
  await expect(sd).toHaveAttribute('aria-pressed', 'true');
  const playLink = page.getByRole('link', {
    name: /^(Abspielen|Weiterschauen ab \d{2}:\d{2}:\d{2})$/,
  });
  const href = (await playLink.getAttribute('href')) ?? '';
  const chosen = new URL(href, 'http://x').searchParams.get('file');
  expect(chosen).toBeTruthy();
  await expect(page.getByRole('region', { name: 'Datei' }).getByText('720p')).toBeVisible();

  const started = await startedFile(page, async () => {
    await playLink.focus();
    await page.keyboard.press('Enter');
  });
  expect(started).toBe(chosen);
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/);
  await expect(player(page)).toHaveAttribute('data-media-file-id', chosen ?? '');

  await page.keyboard.press('i');
  const info = page.getByRole('region', { name: 'Stream-Info' });
  await expect(info).toBeVisible();
  await expect(info.getByText(chosen ?? '', { exact: true })).toBeVisible();
  await expect(info.getByText('720p', { exact: true })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(info).toHaveCount(0);
});

test('the version menu of the player switches to the other file', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  const first = await startedFile(page, () => page.keyboard.press('Enter'));
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/);
  await expect(player(page)).toHaveAttribute('data-media-file-id', first);

  const menuButton = page.getByRole('button', { name: /^Version: / });
  await tabTo(page, menuButton, 10);
  await page.keyboard.press('Enter');
  const menu = page.getByRole('menu', { name: 'Version' });
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  // The menu opens on the current (primary, first) version; the other one is a step down.
  await expect(menu.getByRole('menuitemradio', { checked: true })).toBeFocused();
  await page.keyboard.press('ArrowDown');
  const second = await startedFile(page, () => page.keyboard.press('Enter'));
  expect(second).not.toBe(first);
  await expect(player(page)).toHaveAttribute('data-media-file-id', second);
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/);
  // The player stays (same route) with the focus on play/pause, as after any start.
  await expect(page).toHaveURL(/\/play\//);
  await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
  await expect(page.getByRole('button', { name: /^Version: / })).toHaveText('720p');
});
