import { existsSync, rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { MOVIES_DIR } from './env';
import { expect, test } from './fixtures';
import { expectVisibleFocus } from './keyboard';
import { CHAPTERS_FOLDER, CHAPTERS_TITLE, itemIdByTitle, play, resetProgress } from './playback';

// I-5: a subtitle that Bazarr places next to a film shows up in the player's track menu within
// 60 s, without restarting the playback: the watcher reports the folder, the scan adds the
// sidecar to the tracks, and the open menu asks again every 30 s.

const BUDGET_MS = 60_000;
const SIDECAR = join(MOVIES_DIR, CHAPTERS_FOLDER, `${CHAPTERS_FOLDER}.de.srt`);
const SRT = [
  '1',
  '00:00:00,500 --> 00:00:03,000',
  'Untertitel aus der Sidecar-Datei (E2E)',
  '',
  '2',
  '00:00:03,500 --> 00:00:06,000',
  'Zweiter Untertitel',
  '',
].join('\n');

test('a .de.srt placed during playback appears in the track menu within 60 s', async ({
  page,
  owner,
  browserName,
}) => {
  test.setTimeout(180_000);
  // Each browser places the file anew; a leftover of an aborted run goes first.
  rmSync(SIDECAR, { force: true });
  try {
    const itemId = await itemIdByTitle(owner, CHAPTERS_TITLE);
    // The previous browser's file is gone from the tracks once the watcher saw the removal.
    await expect
      .poll(
        async () => {
          const list = (await (await owner.get(`/api/v1/items/${itemId}/tracks`)).json()) as {
            subtitles: { source: string }[];
          };
          return list.subtitles.filter((track) => track.source === 'sidecar').length;
        },
        { timeout: BUDGET_MS },
      )
      .toBe(0);
    await resetProgress(page.request, itemId, 10_000);
    const info = await play(page, itemId);
    expect(info.tracks.subtitles.filter((track) => track.source === 'sidecar')).toHaveLength(0);

    writeFileSync(SIDECAR, SRT, 'utf8');
    const placed = Date.now();

    await page.keyboard.press('t');
    const menu = page.getByRole('menu', { name: 'Spuren' });
    await expect(menu).toBeVisible();
    await expectVisibleFocus(page);
    // The menu stays open; it asks the server again every 30 s.
    const sidecar = menu.getByRole('menuitemradio', { name: /extern/ });
    await expect(sidecar).toBeVisible({ timeout: BUDGET_MS });
    const elapsedMs = Date.now() - placed;
    expect(elapsedMs).toBeLessThan(BUDGET_MS);
    await expect(sidecar).toHaveAccessibleName(/Deutsch/);

    // The server lists it as well.
    const tracks = (await (
      await page.request.get(`/api/v1/items/${itemId}/tracks?mediaFileId=${info.mediaFile.id}`)
    ).json()) as { subtitles: { source: string; language?: string; index: number }[] };
    const listed = tracks.subtitles.find((track) => track.source === 'sidecar');
    expect(listed?.index).toBe(-1);
    console.log(JSON.stringify({ measurement: 'sidecar', browser: browserName, elapsedMs }));
    await page.keyboard.press('Escape');
    await expect(menu).toHaveCount(0);
  } finally {
    rmSync(SIDECAR, { force: true });
  }
  expect(existsSync(SIDECAR)).toBe(false);
});
