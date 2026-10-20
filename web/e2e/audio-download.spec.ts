import type { Page } from '@playwright/test';
import {
  audioDetail,
  audioItemId,
  feedKey,
  M4B_TITLE,
  subscribeFeed,
  unsubscribe,
  waitForEpisodes,
} from './audio';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus, tabTo } from './keyboard';
import { downloadRow, storedDownloads, waitForComplete } from './offline';

// O-1 for audio (M6): an audiobook and a podcast episode in each of the three download profiles
// (AAC 128/96/64 kbit/s). The dialog shows the bit rate and the size the server expects; the
// device fetches the prepared file into OPFS, and its size lies within ±10 % of that estimate.
// Follows downloads.spec.ts (keyboard on the dialog and on /downloads).

const QUALITIES = [
  { quality: 'low', label: /^Niedrig · 64 kbit\/s · ca\. / },
  { quality: 'medium', label: /^Mittel · 96 kbit\/s · ca\. / },
  { quality: 'high', label: /^Hoch · 128 kbit\/s · ca\. / },
] as const;

const downloadButton = (page: Page) =>
  page.getByRole('button', { name: 'Herunterladen', exact: true });

/** The three profiles of one title by keyboard, each checked against its estimate and deleted. */
async function downloadEachQuality(page: Page, itemId: string, title: string, label: string) {
  const options = (await (
    await page.request.get(`/api/v1/items/${itemId}/download-options`)
  ).json()) as { qualities: { quality: string; estimatedBytes: number; audioBitrate: number }[] };
  expect(options.qualities.map((option) => [option.quality, option.audioBitrate])).toEqual([
    ['high', 128],
    ['medium', 96],
    ['low', 64],
  ]);

  for (const { quality, label: optionLabel } of QUALITIES) {
    await page.goto(`/items/${itemId}`);
    await expect(page.getByRole('heading', { level: 1, name: title })).toBeVisible();
    await downloadButton(page).focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: `${title} herunterladen` });
    await expect(dialog).toBeVisible();
    const radios = dialog.getByRole('radiogroup', { name: 'Qualität' });
    await expect(radios.getByRole('radio')).toHaveCount(3);
    // Audio has no track choice and no subtitles.
    await expect(dialog.getByRole('radiogroup', { name: 'Tonspur' })).toHaveCount(0);
    await expect(dialog.getByText(/Untertitel/)).toHaveCount(0);
    await radios.getByRole('radio', { checked: true }).focus();
    const target = radios.getByRole('radio', { name: optionLabel });
    for (let presses = 0; presses < 3 && !(await target.isChecked()); presses += 1) {
      await page.keyboard.press('ArrowRight');
    }
    await expect(target).toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, dialog.getByRole('button', { name: 'Herunterladen' }), 6);
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);

    const [download] = await waitForComplete(
      page,
      1,
      (entry) => entry.quality === quality && entry.item?.id === itemId,
    );
    const estimate = options.qualities.find((option) => option.quality === quality)!.estimatedBytes;
    const size = download.manifest!.sizeBytes;
    const stored = await page.evaluate(async (id) => {
      const root = await navigator.storage.getDirectory();
      const dir = await (await root.getDirectoryHandle('downloads')).getDirectoryHandle(id);
      return (await (await dir.getFileHandle('media.mp4')).getFile()).size;
    }, download.id);
    const deviation = (size - estimate) / estimate;
    console.log(
      `MEASURE ${test.info().project.name} ${label}-size ${JSON.stringify({ quality, estimate, size, stored, deviation })}`,
    );
    expect(download.bytesWritten).toBe(size);
    expect(stored).toBe(size);
    expect(Math.abs(deviation), `${quality}: ${size} B vs ${estimate} B`).toBeLessThanOrEqual(0.1);

    // One download per title: delete it on /downloads by keyboard before the next profile.
    await page.goto('/downloads');
    const row = downloadRow(page, new RegExp(title.replace(/[()]/g, '\\$&')));
    await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
    await row.getByRole('button', { name: / löschen$/ }).focus();
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: /^Download .* löschen\?$/ });
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Download löschen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);
    await expect.poll(async () => (await storedDownloads(page)).length).toBe(0);
  }
}

test('the audiobook in each of the three profiles, within ±10 % of its estimate (O-1)', async ({
  page,
  owner,
}) => {
  test.setTimeout(420_000);
  const book = await audioItemId(owner, M4B_TITLE);
  await audioDetail(owner, book, 12);
  await downloadEachQuality(page, book, M4B_TITLE, 'audiobook');
});

test('a podcast episode in each of the three profiles, within ±10 % of its estimate (O-1)', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(420_000);
  const podcast = await subscribeFeed(owner, feedKey(testInfo), 2);
  try {
    const [newest] = await waitForEpisodes(owner, podcast.id, 2);
    await downloadEachQuality(page, newest.id, newest.title, 'episode');
  } finally {
    await unsubscribe(owner, podcast.id);
  }
});
