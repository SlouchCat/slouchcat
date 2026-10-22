import type { APIRequestContext } from '@playwright/test';
import { OWNER, OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { H264_TITLE, itemIdByTitle, play, videoState } from './playback';

// /admin/stats (A-8) by keyboard (rule 8): after a playback of the H.264 film the last 7 days show
// it among the most watched titles, the watch time of the main profile, the playback tiles and the
// devices by type. The period sits in the address (?days=); arrow keys switch it.

interface StatsData {
  topTitles: { itemId: string; title: string; plays: number; watchedMs: number }[];
  watchTime: { profileName: string; accountEmail: string; watchedMs: number }[];
  devices: { type: string; count: number }[];
}

async function stats(api: APIRequestContext, days: number): Promise<StatsData> {
  const response = await api.get(`/api/v1/admin/stats?days=${days}`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as StatsData;
}

test('A-8: a playback shows up in the statistics; period, table and Back by keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(60_000);
  // A few seconds of playback; leaving the player reports the played time.
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  await play(page, itemId);
  await expect
    .poll(async () => (await videoState(page)).time, { timeout: 15_000 })
    .toBeGreaterThan(2);
  await page.goto('/admin/stats?days=7');
  // The page has started before the reload below (Firefox aborts a reload during the start).
  await expect(page.getByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
  await expect
    .poll(
      async () =>
        (await stats(owner, 7)).topTitles.find((title) => title.itemId === itemId)?.watchedMs ?? 0,
      {
        timeout: 15_000,
      },
    )
    .toBeGreaterThan(0);
  const numbers = await stats(owner, 7);
  expect(numbers.watchTime.find((entry) => entry.profileName === OWNER_PROFILE)?.accountEmail).toBe(
    OWNER.email,
  );
  expect(numbers.devices.reduce((sum, device) => sum + device.count, 0)).toBeGreaterThan(0);

  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
  const seven = page.getByRole('radio', { name: '7 Tage' });
  await expect(seven).toBeChecked();
  const top = page.getByRole('table', { name: 'Meistgesehen', exact: true });
  const film = top.getByRole('link', { name: `${H264_TITLE} (2024)` });
  await expect(film).toHaveAttribute('href', `/admin/items/${itemId}`);
  const profiles = page.getByRole('table', { name: 'Schauzeit je Profil', exact: true });
  await expect(
    profiles.getByRole('rowheader', { name: new RegExp(`^${OWNER_PROFILE}`) }),
  ).toContainText(OWNER.email);
  const playback = page.getByRole('region', { name: 'Wiedergabe' });
  await expect(playback.getByRole('term')).toHaveText([
    'Direct Play',
    'Remux',
    'Transcode',
    'Transcode auf dem Grafikchip',
  ]);
  await expect(
    page.getByRole('table', { name: 'Geräte', exact: true }).getByRole('rowheader').first(),
  ).toBeVisible();

  // The checked period is the group's Tab stop; arrow keys switch and keep it in the address.
  await resetFocus(page);
  await tabTo(page, seven, 60);
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('radio', { name: '30 Tage' })).toBeChecked();
  await expect(page).toHaveURL(/\/admin\/stats\?days=30$/);
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, seven);
  await expect(page).toHaveURL(/\/admin\/stats\?days=7$/);
  await expect(film).toBeVisible();

  // From the period into the table: Enter opens the title's admin page, Back returns to 7 days.
  await tabTo(page, film, 10);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/admin/items/${itemId}$`));
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/stats\?days=7$/);
  await expect(page.getByRole('radio', { name: '7 Tage' })).toBeChecked();
  await expect(film).toBeVisible();
  await expectNoFocusTrap(page, 120);
});
