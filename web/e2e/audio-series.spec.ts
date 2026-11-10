import type { APIRequestContext, Page } from '@playwright/test';
import { AUDIO_SERIES, audioItemId, M4B_TITLE, MP3_TITLE } from './audio';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// Concept 19 stage D2 (rule 8, PD-5, PD-7, PD-14): the row «Reihen und Sammlungen» above the grid
// of the library «Hörbücher» and the page of a series, by keyboard. The series is the one of the
// generated test books (testdata/generate-media.sh): the M4B book carries «Slouchcat Test Audio
// Series», volume 1, in its tags; the MP3 book lies in the folder of that series as «02 - …», so
// both share the series and the author «Slouchcat Test Author». Nothing here changes data.

const LIBRARY = 'Hörbücher';

interface AudioGroup {
  id: string;
  kind: 'series' | 'collection';
  name: string;
  count: number;
}

async function booksLibrary(owner: APIRequestContext): Promise<number> {
  const response = await owner.get('/api/v1/libraries/visible');
  expect(response.status(), await response.text()).toBe(200);
  const libraries = (await response.json()) as { id: number; name: string }[];
  const library = libraries.find((entry) => entry.name === LIBRARY);
  expect(library, `library "${LIBRARY}" visible`).toBeDefined();
  return library!.id;
}

/** The test series of the library, as the row lists it. */
async function testSeries(owner: APIRequestContext, libraryId: number): Promise<AudioGroup> {
  let found: AudioGroup | undefined;
  // The books are grouped once their scan is done.
  await expect
    .poll(async () => {
      const response = await owner.get(`/api/v1/libraries/${libraryId}/audio-groups`);
      if (response.status() !== 200) return undefined;
      const list = (await response.json()) as { groups: AudioGroup[] };
      found = list.groups.find((group) => group.kind === 'series' && group.name === AUDIO_SERIES);
      return found?.count;
    })
    .toBe(2);
  return found!;
}

const row = (page: Page) => page.getByRole('region', { name: 'Reihen und Sammlungen' });
const volumes = (page: Page) => page.getByRole('list', { name: 'Bände' });

let libraryId = 0;
let series: AudioGroup;
let m4b = '';
let mp3 = '';

test.beforeEach(async ({ owner }) => {
  libraryId ||= await booksLibrary(owner);
  series ??= await testSeries(owner, libraryId);
  m4b ||= await audioItemId(owner, M4B_TITLE);
  mp3 ||= await audioItemId(owner, MP3_TITLE);
});

test('PD-5: the row above the grid: Tab to the tile, arrows to the grid and back, Enter, back to the tile @keyboard', async ({
  page,
}) => {
  await page.goto(`/libraries/${libraryId}`);
  const tile = row(page).getByRole('link', { name: new RegExp(AUDIO_SERIES) });
  await expect(tile).toBeVisible();
  await expect(tile).toContainText('2 Bände');
  await expect(page.getByRole('status').filter({ hasText: /· \d+ Reihen?\b/ })).toBeVisible();
  // Every book stays in the grid below.
  const grid = page.getByRole('list', { name: LIBRARY });
  await expect(grid.getByRole('link', { name: new RegExp(M4B_TITLE) })).toBeVisible();
  await expect(grid.getByRole('link', { name: new RegExp(MP3_TITLE) })).toBeVisible();

  await resetFocus(page);
  await tabTo(page, tile, 40);
  // ↓ leaves the row for the grid, ↑ comes back to the same tile.
  await page.keyboard.press('ArrowDown');
  await expect
    .poll(() => grid.evaluate((list) => list.contains(document.activeElement)))
    .toBe(true);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, tile);

  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`/libraries/${libraryId}/series/${series.id}`);
  await expect(page.getByRole('heading', { level: 1, name: AUDIO_SERIES })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(`/libraries/${libraryId}`);
  await expectFocusOn(page, tile);
  await expectNoFocusTrap(page, 120);
});

test('PD-7, PD-14: the series page: play button, «Reihe bearbeiten», volumes, play and pen by keyboard; back to the library @keyboard', async ({
  page,
}) => {
  await page.goto(`/libraries/${libraryId}`);
  const tile = row(page).getByRole('link', { name: new RegExp(AUDIO_SERIES) });
  await resetFocus(page);
  await tabTo(page, tile, 40);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`/libraries/${libraryId}/series/${series.id}`);
  await expect(page.getByText('Reihe', { exact: true })).toBeVisible();
  await expect(
    page.getByText(/Slouchcat Test Author · gelesen von Slouchcat Test Narrator · 2 Bände/),
  ).toBeVisible();

  // The first arrow lands on the big button (the page's start focus).
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  // The head's button, not the play buttons of the volumes («Abspielen: <title>»).
  const play = page.getByRole('link', { name: /^(Abspielen|Nochmals anhören|Weiterhören: .+)$/ });
  await expectFocusOn(page, play);
  await expect(play).toHaveAttribute('href', new RegExp(`^/play/(${m4b}|${mp3})`));

  // «Reihe bearbeiten»: Enter opens the dialog with the name, Escape returns to the button.
  await page.keyboard.press('Tab');
  const edit = page.getByRole('button', { name: 'Reihe bearbeiten' });
  await expectFocusOn(page, edit);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Reihe bearbeiten' });
  await expect(dialog.getByRole('textbox', { name: 'Name der Reihe' })).toBeFocused();
  await expect(dialog.getByRole('textbox', { name: 'Name der Reihe' })).toHaveValue(AUDIO_SERIES);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, edit);

  // The volumes in order, Band 1 then Band 2; ↓ walks them, → reaches the play button.
  const first = volumes(page).getByRole('link', { name: `Band 1: ${M4B_TITLE}` });
  const second = volumes(page).getByRole('link', { name: `Band 2: ${MP3_TITLE}` });
  await expect(volumes(page).getByRole('listitem')).toHaveCount(2);
  await tabTo(page, first, 10);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, second);
  await page.keyboard.press('ArrowRight');
  const playSecond = volumes(page).getByRole('link', { name: `Abspielen: ${MP3_TITLE}` });
  await expectFocusOn(page, playSecond);
  await expect(playSecond).toHaveAttribute('href', new RegExp(`^/play/${mp3}`));

  // The pen after the play button by Tab; its editor closes with Escape back onto it.
  await page.keyboard.press('Tab');
  const pen = volumes(page).getByRole('button', { name: `${MP3_TITLE}: Metadaten bearbeiten` });
  await expectFocusOn(page, pen);
  await page.keyboard.press('Enter');
  await expect(page.getByRole('dialog', { name: MP3_TITLE })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog', { name: MP3_TITLE })).toHaveCount(0);
  await expectFocusOn(page, pen);

  // A volume opens its detail page, which links back to the series; back returns to the volume.
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, second);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, first);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`/items/${m4b}`);
  await expect(page.getByRole('link', { name: `${AUDIO_SERIES} · Band 1` })).toHaveAttribute(
    'href',
    `/libraries/${libraryId}/series/${series.id}`,
  );
  await page.goBack();
  await expect(page).toHaveURL(`/libraries/${libraryId}/series/${series.id}`);
  await expectFocusOn(page, first);
  await expectNoFocusTrap(page, 80);

  // The page's back arrow (after the side navigation) returns to the overview, to the tile it
  // was left from.
  await resetFocus(page);
  const back = page.getByRole('button', { name: 'Zurück' });
  await tabTo(page, back, 40);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`/libraries/${libraryId}`);
  await expectFocusOn(page, tile);
});
