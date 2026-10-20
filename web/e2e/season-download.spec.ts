import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import {
  DOWNLOAD_SHOW_TITLE,
  downloadsTable,
  PREPARE_TIMEOUT_MS,
  storedDownloads,
  syncNow,
  waitForComplete,
} from './offline';
import { episodeIds, itemIdByTitle } from './playback';

// O-2: a whole season in one action. The series "Slouchcat Test Download Show" has six 8-s
// episodes; the season page's button creates six downloads, all of them reach the device, and
// the season shows "6 von 6 Folgen auf diesem Gerät".

test('a season of six episodes downloads in one action (O-2)', async ({ page, owner }) => {
  test.setTimeout(PREPARE_TIMEOUT_MS * 3);
  const seriesId = await itemIdByTitle(owner, DOWNLOAD_SHOW_TITLE);
  const episodes = await episodeIds(owner, seriesId);
  expect(episodes).toHaveLength(6);
  const series = (await (await owner.get(`/api/v1/items/${seriesId}`)).json()) as {
    seasons: { id: string; seasonNumber?: number; title: string }[];
  };
  const season = series.seasons.find((entry) => entry.seasonNumber === 1)!;

  // On the series page the button sits below the episodes of the chosen season.
  await page.goto(`/items/${seriesId}`);
  await expect(page.getByRole('list', { name: 'Folgen' }).getByRole('link')).toHaveCount(6);
  const button = page.getByRole('button', { name: 'Staffel herunterladen' });
  await tabTo(page, button, 40);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `${season.title} herunterladen` });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText('Alle 6 Folgen dieser Staffel werden auf diesem Gerät gespeichert.'),
  ).toBeVisible();
  // A season takes each episode's default audio track: no track choice here.
  await expect(dialog.getByRole('radiogroup', { name: 'Tonspur' })).toHaveCount(0);
  await expect(dialog.getByRole('radio', { name: /^Mittel/ })).toBeChecked();
  await expectVisibleFocus(page);
  // Tab stays inside the dialog (it is modal) and reaches its buttons.
  await tabTo(page, dialog.getByRole('button', { name: 'Herunterladen' }), 6);
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, button);

  await expect.poll(async () => (await storedDownloads(page)).length).toBe(6);
  const stored = await storedDownloads(page);
  expect(stored.map((download) => download.item?.id).sort()).toEqual([...episodes].sort());
  expect(stored.every((download) => download.item?.parentId === season.id)).toBe(true);

  await waitForComplete(page, 6);
  await syncNow(page);
  await expect(page.getByText('6 von 6 Folgen auf diesem Gerät')).toBeVisible();

  // All six on /downloads; the rows are reachable and the page has no focus trap.
  await page.goto('/downloads');
  await expect(downloadsTable(page).getByRole('row')).toHaveCount(7);
  await expect(downloadsTable(page).getByText('Auf diesem Gerät')).toHaveCount(6);
  for (let episode = 1; episode <= 6; episode += 1) {
    await expect(
      downloadsTable(page).getByRole('rowheader', {
        name: new RegExp(`^S01E0${episode} · .* · ${DOWNLOAD_SHOW_TITLE}`),
      }),
    ).toBeVisible();
  }
  await tabTo(
    page,
    downloadsTable(page)
      .getByRole('link', { name: /abspielen$/ })
      .first(),
    30,
  );
  await page.keyboard.press('ArrowDown');
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);

  // The season's own page shows the same state.
  await page.goto(`/items/${season.id}`);
  await expect(page.getByText('6 von 6 Folgen auf diesem Gerät')).toBeVisible();
});
