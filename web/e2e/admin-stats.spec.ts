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

// /admin/stats (A-8; concept 12, mockup Sta-B) by keyboard (rule 8): after a playback of the H.264
// film the last 7 days show it among the most watched titles, the watch time of the main profile,
// the playback strip, the devices by type and «Wann geschaut wird» with its tables. The period
// sits in the header and in the address (?days=); arrow keys switch it.

interface StatsData {
  topTitles: { itemId: string; title: string; plays: number; watchedMs: number }[];
  watchTime: { profileName: string; accountEmail: string; watchedMs: number }[];
  devices: { type: string; count: number }[];
  watchTimeByQuarterHour: number[];
  timeWindowDays: number;
  timeZone: string;
}

async function stats(api: APIRequestContext, days: number): Promise<StatsData> {
  const response = await api.get(`/api/v1/admin/stats?days=${days}`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as StatsData;
}

test('A-8: a playback shows up in the statistics; period, tables, grid and Back by keyboard', async ({
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
  // The played time lands in the quarter hours of the last whole week.
  expect(numbers.watchTimeByQuarterHour).toHaveLength(672);
  expect(numbers.watchTimeByQuarterHour.reduce((sum, value) => sum + value, 0)).toBeGreaterThan(0);
  expect(numbers.timeWindowDays).toBe(7);

  await page.reload();
  await expect(page.getByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
  const seven = page.getByRole('radio', { name: '7 Tage' });
  await expect(seven).toBeChecked();
  // The film is on the podium or in the table from place 4, depending on what else was played.
  const top = page.getByRole('region', { name: 'Meistgesehen' });
  const film = top.getByRole('link', { name: `${H264_TITLE} (2024)` });
  await expect(film).toHaveAttribute('href', `/admin/items/${itemId}`);
  await expect(top.getByRole('list', { name: 'Platz 1 bis 3' }).getByRole('link')).not.toHaveCount(
    0,
  );
  const profiles = page.getByRole('table', { name: 'Schauzeit je Profil', exact: true });
  await expect(
    profiles.getByRole('rowheader', { name: new RegExp(`^${OWNER_PROFILE}`) }),
  ).toContainText(OWNER.email);
  const playback = page.getByRole('group', { name: 'Wiedergabe' });
  await expect(playback.getByRole('term')).toHaveText([
    'Direct Play',
    'Remux',
    'Transcode',
    'Transcode auf dem Grafikchip',
  ]);
  await expect(
    page.getByRole('table', { name: 'Geräte', exact: true }).getByRole('rowheader').first(),
  ).toBeVisible();
  // The charts' tables are for screen readers only, their captions too: nothing of them is
  // painted where they stand (Firefox once clipped the table but not its caption).
  const caption = page
    .getByRole('table', { name: 'Schauzeit je Profil als Diagramm' })
    .locator('caption');
  await caption.scrollIntoViewIfNeeded();
  expect(
    await caption.evaluate((element) => {
      const box = element.getBoundingClientRect();
      const hit = document.elementFromPoint(box.left + 2, box.top + box.height / 2);
      return hit === null ? 'nothing' : element.contains(hit) ? 'caption' : 'other';
    }),
  ).toBe('other');

  // «Wann geschaut wird»: the window, the server's zone, the peak and both charts as tables.
  const when = page.getByRole('region', { name: 'Wann geschaut wird' });
  await expect(when.getByText('Aus der letzten Woche')).toBeVisible();
  await expect(when).toContainText(`Stunden in der Zeitzone des Servers (${numbers.timeZone}).`);
  await expect(when).toContainText(/Spitze: \S+, \d+–\d+ Uhr/);
  await expect(when.getByRole('table', { name: 'Nach Tageszeit' }).getByRole('row')).toHaveCount(
    25,
  );
  await expect(
    when.getByRole('table', { name: 'Nach Wochentag' }).getByRole('rowheader'),
  ).toHaveText(['Montag', 'Dienstag', 'Mittwoch', 'Donnerstag', 'Freitag', 'Samstag', 'Sonntag']);

  // The checked period is the group's Tab stop; arrow keys switch and keep it in the address.
  await resetFocus(page);
  await tabTo(page, seven, 60);
  await page.keyboard.press('ArrowRight');
  await expect(page.getByRole('radio', { name: '30 Tage' })).toBeChecked();
  await expect(page).toHaveURL(/\/admin\/stats\?days=30$/);
  await expectVisibleFocus(page);
  await expect(when.getByText('Aus den letzten 4 Wochen')).toBeVisible();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, seven);
  await expect(page).toHaveURL(/\/admin\/stats\?days=7$/);
  await expect(film).toBeVisible();

  // «Woche und Stunde» starts folded; Enter opens it and its table is there for screen readers.
  const grid = when.getByRole('table', { name: 'Schauzeit nach Wochentag und Stunde' });
  const fold = when.getByRole('button', { name: 'Woche und Stunde' });
  await expect(fold).toHaveAttribute('aria-expanded', 'false');
  await expect(grid).toHaveCount(0);
  await tabTo(page, fold, 80);
  await expectFocusOn(page, fold);
  await page.keyboard.press('Enter');
  await expect(fold).toHaveAttribute('aria-expanded', 'true');
  await expect(grid.getByRole('rowheader')).toHaveCount(7);
  await expect(grid.getByRole('row').first().getByRole('columnheader')).toHaveCount(25);

  // From the period into the titles: Enter opens the title's admin page, Back returns to 7 days.
  await resetFocus(page);
  await tabTo(page, seven, 60);
  await tabTo(page, film, 40);
  await expectFocusOn(page, film);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/admin/items/${itemId}$`));
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/stats\?days=7$/);
  await expect(page.getByRole('radio', { name: '7 Tage' })).toBeChecked();
  await expect(film).toBeVisible();
  await expectNoFocusTrap(page, 160);
});
