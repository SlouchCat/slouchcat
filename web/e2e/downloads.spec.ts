import type { Page } from '@playwright/test';
import { H264_TITLE, idOf, itemPath } from './catalog';
import { OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import {
  createDownload,
  downloadRow,
  downloadsTable,
  offlinePlayer,
  storedDownloads,
  waitForComplete,
} from './offline';
import { arrowBetweenRows, CHAPTERS_TITLE, itemIdByTitle } from './playback';

// O-1: a film in each of the three download profiles. The detail page offers them with the size
// the server expects; the device fetches the prepared file into OPFS, and its size lies within
// ±10 % of that estimate. Keyboard on the dialog and on /downloads (rule 8).

const QUALITIES = [
  { quality: 'low', label: /^Niedrig · \d+p · ca\. / },
  { quality: 'medium', label: /^Mittel · \d+p · ca\. / },
  { quality: 'high', label: /^Hoch · \d+p · ca\. / },
] as const;

let detail = '';

const downloadButton = (page: Page) =>
  page.getByRole('button', { name: 'Herunterladen', exact: true });

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
});

test('three download profiles, each within ±10 % of its estimate (O-1)', async ({
  page,
  owner,
}) => {
  test.setTimeout(420_000);
  const itemId = idOf(detail);
  const options = (await (await owner.get(`/api/v1/items/${itemId}/download-options`)).json()) as {
    qualities: { quality: string; estimatedBytes: number }[];
  };
  expect(options.qualities.map((option) => option.quality)).toEqual(['high', 'medium', 'low']);

  for (const { quality, label } of QUALITIES) {
    await page.goto(detail);
    await downloadButton(page).focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `${H264_TITLE} herunterladen` });
    await expect(dialog).toBeVisible();
    const radios = dialog.getByRole('radiogroup', { name: 'Qualität' });
    await expect(radios.getByRole('radio')).toHaveCount(3);
    // Arrow keys move inside the radio group; the choice is made by the arrows (react-aria).
    await radios.getByRole('radio', { checked: true }).focus();
    const target = radios.getByRole('radio', { name: label });
    for (let presses = 0; presses < 3 && !(await target.isChecked()); presses += 1) {
      await page.keyboard.press('ArrowRight');
    }
    await expect(target).toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, dialog.getByRole('button', { name: 'Herunterladen' }), 6);
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Zu den Downloads' })).toBeVisible();

    const [download] = await waitForComplete(page, 1, (entry) => entry.quality === quality);
    const estimate = options.qualities.find((option) => option.quality === quality)!.estimatedBytes;
    const size = download.manifest!.sizeBytes;
    expect(download.bytesWritten).toBe(size);
    expect(
      Math.abs(size - estimate) / estimate,
      `${quality}: ${size} B vs ${estimate} B`,
    ).toBeLessThanOrEqual(0.1);
    // The file in OPFS has exactly the manifest's size.
    const stored = await page.evaluate(async (id) => {
      const root = await navigator.storage.getDirectory();
      const dir = await (await root.getDirectoryHandle('downloads')).getDirectoryHandle(id);
      return (await (await dir.getFileHandle('media.mp4')).getFile()).size;
    }, download.id);
    expect(stored).toBe(size);

    // Delete it on /downloads by keyboard before the next profile (one download per title).
    await page.goto('/downloads');
    const row = downloadRow(page, new RegExp(H264_TITLE));
    await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
    await row.getByRole('button', { name: `${H264_TITLE} löschen` }).focus();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: `Download ${H264_TITLE} löschen?` });
    await expect(confirm).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Download löschen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);
    await expect.poll(async () => (await storedDownloads(page)).length).toBe(0);
  }
});

test('downloads page by keyboard: rows, play, back; no focus trap', async ({ page, owner }) => {
  test.setTimeout(180_000);
  await downloadButton(page).focus();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `${H264_TITLE} herunterladen` });
  await expect(dialog.getByRole('radio', { name: /^Niedrig/ })).toBeVisible();
  // Escape closes the dialog and the focus comes back to the button.
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, downloadButton(page));
  await page.keyboard.press('Enter');
  await dialog.getByText(/^Niedrig/).click();
  await dialog.getByRole('button', { name: 'Herunterladen' }).click();
  await waitForComplete(page);

  // A second title so the table has two rows to move between.
  await createDownload(page, await itemIdByTitle(owner, CHAPTERS_TITLE));
  await waitForComplete(page, 2);

  // The offline library by arrows (rule 8, W11): the profile tile, then between the two titles.
  await page.goto('/offline');
  const tile = page
    .getByRole('list', { name: 'Profile' })
    .getByRole('button', { name: new RegExp(OWNER_PROFILE) });
  await expectFocusOn(page, tile);
  await page.keyboard.press('Enter');
  const cards = page.getByRole('list', { name: 'Heruntergeladene Titel' }).getByRole('link');
  await expect(cards).toHaveCount(2);
  await expectFocusOn(page, cards.first());
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards.nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards.first());
  // Escape goes back to the profiles.
  await page.keyboard.press('Escape');
  await expect(page.getByRole('list', { name: 'Profile' })).toBeVisible();

  await page.goto('/downloads');
  await expect(page.getByRole('heading', { level: 1, name: 'Downloads' })).toBeVisible();
  await expect(
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: 'Downloads' }),
  ).toHaveAttribute('aria-current', 'page');
  await expect(downloadsTable(page).getByRole('row')).toHaveCount(3);

  // Tab reaches the first row's play link; ↑/↓ move between the rows.
  await resetFocus(page);
  const firstPlay = downloadsTable(page)
    .getByRole('link', { name: /abspielen$/ })
    .first();
  await tabTo(page, firstPlay, 30);
  await arrowBetweenRows(
    page,
    downloadsTable(page)
      .getByRole('link', { name: /abspielen$/ })
      .last(),
  );
  await expectVisibleFocus(page);

  // Enter plays the download; Escape returns to the downloads page.
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/offline\/play\//);
  await expect(offlinePlayer(page)).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/downloads$/);

  // The storage panel and the rule form follow; Tab leaves the page at its end.
  await expect(page.getByRole('region', { name: 'Speicher' })).toBeVisible();
  await expect(page.getByRole('region', { name: 'Automatische Downloads' })).toBeVisible();
  await expectNoFocusTrap(page);

  // Backspace (a remote's back key) goes back.
  await resetFocus(page);
  await page.keyboard.press('Backspace');
  await expect(page).not.toHaveURL(/\/downloads$/);
});
