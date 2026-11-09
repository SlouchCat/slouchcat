import { expect, test, type Page, type Route } from '@playwright/test';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Keyboard test of the page "Bibliotheken" (CLAUDE.md rule 8; concept 12, variant C, Bib-C): the
// filter tabs by kind with the arrow keys, a library's row that folds out with Enter, its actions
// and «Weitere Aktionen» with arrows and Escape, and the dialog «Bibliothek hinzufügen» with the
// sources of a folder ordered by position (S-16). Other specs add libraries of their own ("E2E
// leer", "Dokus", "E2E Ordner"), so every count and Tab budget leaves room for them.

// The worker registers on the first page load and takes the page over (clientsClaim) within
// seconds; from then on it sends every GET under /api to the network itself, where the page's
// routes do not see it. Two tests answer or hold back the library list through a route, which
// would give way to the server's in the middle of a test. The page needs no worker.
test.use({ serviceWorkers: 'block' });

/** The toggle of a library's row: its name. */
const toggle = (page: Page, name: string) => page.getByRole('button', { name, exact: true });
/** The table row of a library. */
const rowOf = (page: Page, name: string) =>
  page.getByRole('row').filter({ has: toggle(page, name) });
/** The fold-out of a library's row (a region named like the library). */
const foldOut = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const addDialog = (page: Page) => page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
const addButton = (page: Page) => page.getByRole('button', { name: 'Bibliothek hinzufügen' });

/** Tabs to a library's toggle and folds its row out with Enter. */
async function openRow(page: Page, name: string): Promise<void> {
  const button = toggle(page, name);
  await tabTo(page, button, 60);
  if ((await button.getAttribute('aria-expanded')) !== 'true') await page.keyboard.press('Enter');
  await expect(button).toHaveAttribute('aria-expanded', 'true');
  await expect(foldOut(page, name)).toBeVisible();
}

/** Opens «Bibliothek hinzufügen» from the head with the keyboard; the name field has the focus. */
async function openAdd(page: Page): Promise<void> {
  await tabTo(page, addButton(page), 10);
  await page.keyboard.press('Enter');
  await expect(addDialog(page)).toBeVisible();
  await expectFocusOn(page, addDialog(page).getByLabel('Name', { exact: true }));
}

test.beforeEach(async ({ page }) => {
  await page.goto('/admin/libraries');
  await expect(page.getByRole('heading', { level: 1, name: 'Bibliotheken' })).toBeVisible();
  await expect(toggle(page, 'Filme')).toBeVisible();
  await resetFocus(page);
});

test('the scanned test library is listed with its state and counts', async ({ page }) => {
  const row = rowOf(page, 'Filme');
  await expect(row.getByText('Aktuell')).toBeVisible();
  await expect(row.getByText(/Titel: \d+ · Dateien: \d+/)).toBeVisible();
  await expect(row.getByText(/testdata\/media\/Movies$/)).toBeVisible();
  // The whole path, the watch mode and the file system are in the fold-out (S-5, BIB-2).
  await openRow(page, 'Filme');
  await expect(foldOut(page, 'Filme').getByText(/testdata\/media\/Movies$/)).toBeVisible();
  await expect(foldOut(page, 'Filme').getByText(/^Dateisystem /)).toBeVisible();

  // The keyframe index of the video library (concept 14, 7.2): its counts, and the link to the
  // backfill only while files wait (the next test follows it).
  const listed = await page.request.get('/api/v1/admin/libraries');
  expect(listed.status(), await listed.text()).toBe(200);
  const movies = (
    (await listed.json()) as { name: string; keyframeIndex?: { pending: number } }[]
  ).find((library) => library.name === 'Filme');
  expect(movies?.keyframeIndex).toBeDefined();
  await expect(
    foldOut(page, 'Filme').getByText(
      /^Keyframe-Index( vollständig|: \d+ von \d+ Videodateien · \d+ ausstehend · \d+ nicht möglich)$/,
    ),
  ).toBeVisible();
  const backfill = foldOut(page, 'Filme').getByRole('link', {
    name: 'Nachholen auf der Seite Hintergrundjobs',
  });
  await expect(backfill).toHaveCount((movies?.keyframeIndex?.pending ?? 0) > 0 ? 1 : 0);
});

test('the keyframe index links the backfill, whose row the jobs page opens', async ({ page }) => {
  // With cue indexes the test library has nothing pending: the answer says files wait, the rest
  // is the real page.
  await page.route('**/api/v1/admin/libraries', async (route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    const response = await route.fetch();
    const body = (await response.json()) as { name: string; keyframeIndex?: unknown }[];
    for (const library of body) {
      if (library.name === 'Filme') {
        library.keyframeIndex = { videoFiles: 4, indexed: 3, pending: 1, unavailable: 0 };
      }
    }
    await route.fulfill({ response, json: body });
  });
  await page.reload();
  await expect(toggle(page, 'Filme')).toBeVisible();
  await resetFocus(page);
  await openRow(page, 'Filme');
  await expect(
    foldOut(page, 'Filme').getByText(
      'Keyframe-Index: 3 von 4 Videodateien · 1 ausstehend · 0 nicht möglich',
    ),
  ).toBeVisible();
  const backfill = foldOut(page, 'Filme').getByRole('link', {
    name: 'Nachholen auf der Seite Hintergrundjobs',
  });
  await tabTo(page, backfill, 20);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/jobs\?item=keyframes\.backfill$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Keyframe-Index nachholen', exact: true }),
  ).toHaveAttribute('aria-expanded', 'true');
});

test('tabs by kind with the arrow keys, counts and the tab in the address', async ({ page }) => {
  const tabs = page.getByRole('tablist', { name: 'Bibliotheken nach Art' });
  const all = tabs.getByRole('tab', { name: /^Alle \d+ Bibliotheken$/ });
  await tabTo(page, all, 20);
  await expect(all).toHaveAttribute('aria-selected', 'true');
  await page.keyboard.press('ArrowRight');
  const video = tabs.getByRole('tab', { name: /^Video \d+ Bibliothek/ });
  await expectFocusOn(page, video);
  await expect(video).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/\?tab=video$/);
  await expect(toggle(page, 'Filme')).toBeVisible();
  await expect(toggle(page, 'Hörbücher')).toHaveCount(0);

  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tabs.getByRole('tab', { name: /^Hörbücher \d+ Bibliothek/ }));
  await expect(page).toHaveURL(/\?tab=audiobooks$/);
  await expect(toggle(page, 'Hörbücher')).toBeVisible();
  await expect(toggle(page, 'Filme')).toHaveCount(0);

  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, tabs.getByRole('tab', { name: /^Podcasts \d+ Bibliothek/ }));
  await expect(page).toHaveURL(/\?tab=podcasts$/);
  await expect(toggle(page, 'Podcasts')).toBeVisible();

  // Back to «Alle»: the address loses its tab again. Tab goes on into the table.
  await page.keyboard.press('Home');
  await expectFocusOn(page, all);
  await expect(page).toHaveURL(/\/admin\/libraries$/);
  await page.keyboard.press('Tab');
  await expectVisibleFocus(page);
  await expectNoFocusTrap(page);

  // An unknown tab in the address shows «Alle» and is taken out of it.
  await page.goto('/admin/libraries?tab=nonsense');
  await expect(all).toHaveAttribute('aria-selected', 'true');
  await expect(page).toHaveURL(/\/admin\/libraries$/);
});

test('Tab reaches header, the add dialog and its fields in order; no focus trap @keyboard', async ({
  page,
}) => {
  // The admin sidebar is one Tab stop: its current page.
  await tabTo(
    page,
    page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: /^Bibliotheken(,|$)/ }),
  );
  // Then the head: help, the main action.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Hilfe zu dieser Seite' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, addButton(page));
  await page.keyboard.press('Enter');
  const dialog = addDialog(page);
  await expectFocusOn(page, dialog.getByLabel('Name', { exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('radio', { name: 'Video' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, dialog.getByRole('radio', { name: 'Hörbücher' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByLabel('Ordner', { exact: true }));
  // The folder browser of idea 02 sits right after the field, as the wizard's check does.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Durchsuchen …' }));
  // An audiobook folder has one content and one source: nothing to choose, the next stop is the
  // button for another card (F-13), then the default age rating.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    dialog.getByRole('radiogroup', { name: 'Standard-Altersstufe' }).getByRole('radio', {
      name: 'FSK 0',
    }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    dialog.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
  );
  // Escape closes the dialog and hands the focus back to the head's button.
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, addButton(page));
  await expectNoFocusTrap(page);
});

test('content and metadata sources by keyboard: Space ticks, the position orders, cards come and go (ADR 0015, S-16)', async ({
  page,
}) => {
  await openAdd(page);
  const form = addDialog(page);
  await page.keyboard.type('E2E Anime');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Video' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByLabel('Ordner', { exact: true }));
  await page.keyboard.type('/does/not/exist/slouchcat-e2e-anime');
  await page.keyboard.press('Tab'); // Durchsuchen …
  // «Inhalt»: films are chosen; the arrow switches to series, whose default list is TMDB › TVDB.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Filme' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Serien' }));
  await expect(form.getByRole('radio', { name: 'Serien' })).toBeChecked();
  const sources = form.getByRole('group', { name: 'Metadatenquellen' });
  await expect(sources.getByRole('checkbox', { name: 'TMDB' })).toBeChecked();
  await expect(sources.getByRole('checkbox', { name: 'TVDB' })).toBeChecked();
  await expect(sources.getByRole('checkbox', { name: 'AniList' })).not.toBeChecked();
  // No arrow buttons any more: a ticked source has a position to choose.
  await expect(sources.getByRole('button', { name: /nach oben/ })).toHaveCount(0);
  // Tab walks the rows: checkbox, position; Space ticks AniList, which comes last.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, sources.getByRole('checkbox', { name: 'TMDB' }));
  await expectVisibleFocus(page);
  await tabTo(page, sources.getByRole('checkbox', { name: 'AniList' }), 8);
  await page.keyboard.press('Space');
  await expect(sources.getByRole('checkbox', { name: 'AniList' })).toBeChecked();
  await expect(sources.getByText('füllt auf')).toHaveCount(2);
  // ↑ on its position moves AniList up one place; the select keeps the focus.
  const rank = sources.getByRole('combobox', { name: 'Platz für AniList' });
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rank);
  await expect(rank).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, rank);
  await expect(sources.getByRole('checkbox').nth(1)).toHaveAccessibleName('AniList');
  // In front it may not stand: the list says so before the server does.
  await page.keyboard.press('ArrowUp');
  await expect(sources.getByRole('checkbox').first()).toHaveAccessibleName('AniList');
  await expect(sources.getByRole('alert')).toHaveText(/AniList kann keinen Titel erkennen/);
  await expect(
    form.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }),
  ).toBeDisabled();
  await page.keyboard.press('ArrowDown');
  await expect(sources.getByRole('alert')).toHaveCount(0);
  // «+ Weiteren Ordner hinzufügen» by Enter: the new card takes the focus in its path field and
  // proposes the other content; «Entfernen» hands the focus back to the button.
  await tabTo(page, form.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }), 20);
  await page.keyboard.press('Enter');
  const second = form.getByRole('listitem', { name: 'Ordner 2' });
  await expectFocusOn(page, second.getByLabel('Ordner', { exact: true }));
  await expect(second.getByRole('radio', { name: 'Filme' })).toBeChecked();
  await expect(
    form.getByRole('button', { name: 'Bibliothek mit 2 Ordnern hinzufügen' }),
  ).toBeVisible();
  // «Entfernen» stands in the card's head, right before the path field.
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, second.getByRole('button', { name: 'Ordner 2 entfernen' }));
  await page.keyboard.press('Enter');
  await expect(second).toHaveCount(0);
  await expectFocusOn(page, form.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
  await expectNoFocusTrap(page);
});

test('audio kinds by keyboard: default rating with arrows, no folder for podcasts (M6)', async ({
  page,
}) => {
  await openAdd(page);
  const form = addDialog(page);
  const folder = form.getByLabel('Ordner', { exact: true });
  const rating = form.getByRole('radiogroup', { name: 'Standard-Altersstufe' });
  await page.keyboard.type('E2E Hörbücher');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Video' }));
  // A video library has no default rating; the second kind is audiobooks.
  await expect(rating).toHaveCount(0);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Hörbücher' }));
  await expect(form.getByRole('radio', { name: 'Hörbücher' })).toBeChecked();
  await expect(rating).toBeVisible();
  // Tab: folder, then the rating (FSK 0 by default); the arrows choose FSK 6.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, folder);
  await page.keyboard.type('/does/not/exist/slouchcat-e2e-audio');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('button', { name: 'Durchsuchen …' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('button', { name: 'Weiteren Ordner hinzufügen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 0' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expect(rating.getByRole('radio', { name: 'FSK 6' })).toBeChecked();
  await expectVisibleFocus(page);
  // Back to the folder with Shift+Tab; Enter there sends kind and rating, and the missing
  // folder is reported at the field.
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, folder);
  const sent = page.waitForRequest(
    (request) => request.method() === 'POST' && request.url().endsWith('/admin/libraries'),
  );
  await page.keyboard.press('Enter');
  expect((await sent).postDataJSON()).toMatchObject({ kind: 'audiobooks', defaultRating: 6 });
  await expect(folder).toHaveAttribute('aria-invalid', 'true');
  // Podcasts: the card goes, the hint says why; the add button and the rating stay (F-13).
  await expectFocusOn(page, folder);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Hörbücher' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, form.getByRole('radio', { name: 'Podcasts' }));
  await expect(folder).toHaveCount(0);
  await expect(form.getByText(/Ordner optional/)).toBeVisible();
  // Without a folder field there is nothing to browse either; the next stop is the add button.
  await expect(form.getByRole('button', { name: 'Durchsuchen …' })).toHaveCount(0);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByRole('button', { name: 'Ordner hinzufügen', exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, rating.getByRole('radio', { name: 'FSK 6' }));
  await expect(
    form.getByRole('button', { name: 'Bibliothek ohne Ordner hinzufügen' }),
  ).toBeVisible();
  await expectNoFocusTrap(page);
});

test('Enter on "Erneut scannen" queues a scan that finishes', async ({ page }) => {
  await openRow(page, 'Filme');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, foldOut(page, 'Filme').getByRole('button', { name: 'Erneut scannen' }));
  const scan = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().endsWith('/scan'),
  );
  await page.keyboard.press('Enter');
  expect((await scan).status()).toBe(202);
  await expect(rowOf(page, 'Filme').getByText('Aktuell')).toBeVisible({ timeout: 60_000 });
});

test('an invalid path is reported at the field; Enter submits from the field', async ({ page }) => {
  await openAdd(page);
  const form = addDialog(page);
  await page.keyboard.type('Fehlt');
  await page.keyboard.press('Tab'); // Video
  await page.keyboard.press('Tab');
  await expectFocusOn(page, form.getByLabel('Ordner', { exact: true }));
  await page.keyboard.type('/does/not/exist/slouchcat-e2e');
  await page.keyboard.press('Enter');
  await expect(form.getByLabel('Ordner', { exact: true })).toHaveAttribute('aria-invalid', 'true');
  await expect(form.getByText(/Diesen Ordner gibt es auf dem Server nicht/)).toBeVisible();
  // The dialog stays open with what was typed, so it can be corrected.
  await expect(form.getByLabel('Ordner', { exact: true })).toHaveValue(
    '/does/not/exist/slouchcat-e2e',
  );
});

test('the audio libraries show kind and default rating; the menu opens the rating dialog', async ({
  page,
}) => {
  const books = rowOf(page, 'Hörbücher');
  await expect(books.getByText('Hörbücher', { exact: true })).toHaveCount(2);
  await expect(books.getByText('Standard: FSK 0')).toBeVisible();
  await expect(rowOf(page, 'Podcasts').getByText('Standard: FSK 0')).toBeVisible();

  // «Weitere Aktionen» of the audiobooks: Enter opens it, the arrows move, Escape closes it.
  await openRow(page, 'Hörbücher');
  const more = foldOut(page, 'Hörbücher').getByRole('button', {
    name: 'Weitere Aktionen für Hörbücher',
  });
  await tabTo(page, more, 10);
  await page.keyboard.press('Enter');
  const item = page.getByRole('menuitem', { name: 'Standardstufe ändern' });
  await expectFocusOn(page, item);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, item);
  await page.keyboard.press('Escape');
  await expect(item).toHaveCount(0);
  await expectFocusOn(page, more);

  // "Standardstufe ändern" opens the dialog with the current rating; Escape leaves it unchanged.
  await page.keyboard.press('Enter');
  await expectFocusOn(page, item);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Standardstufe von Hörbücher' });
  await expect(dialog.getByRole('radio', { name: 'FSK 0' })).toBeChecked();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, more);

  // The podcasts come after the audiobooks: their fold-out says where the titles come from.
  await openRow(page, 'Podcasts');
  await expect(
    foldOut(page, 'Podcasts').getByText('Kein Ordner: Die Titel kommen aus abonnierten Feeds.'),
  ).toBeVisible();
  await expectNoFocusTrap(page);
});

test('«Vorschau beim Spulen» by keyboard: groups with arrows, Speichern, scope, Escape and save (concept 18, 9) @keyboard', async ({
  page,
}) => {
  const libraryId = async () => {
    const listed = await page.request.get('/api/v1/admin/libraries');
    const movies = ((await listed.json()) as { id: number; name: string }[]).find(
      (library) => library.name === 'Filme',
    );
    if (!movies) throw new Error('no library Filme');
    return movies.id;
  };
  const id = await libraryId();
  try {
    await openRow(page, 'Filme');
    await expect(
      foldOut(page, 'Filme').getByText('Vorschau beim Spulen: Filme Im Voraus · Serien Im Voraus'),
    ).toBeVisible();
    const open = foldOut(page, 'Filme').getByRole('button', {
      name: 'Vorschau beim Spulen von Filme',
    });
    await tabTo(page, open, 20);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Vorschau beim Spulen: Filme' });
    await expect(dialog).toBeVisible();
    const movies = dialog.getByRole('radiogroup', { name: 'Filme' });
    const series = dialog.getByRole('radiogroup', { name: 'Serien' });
    // The estimate of getLibrary.
    await expect(dialog.getByText(/^Heute belegt: Filme .* · Serien /)).toBeVisible();

    // Films: ↓ to «Beim Start» shows «Speichern», Tab reaches it, Space unticks it.
    await tabTo(page, movies.getByRole('radio', { name: 'Im Voraus' }), 10);
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowDown');
    await expect(movies.getByRole('radio', { name: 'Beim Start' })).toBeChecked();
    const keep = dialog.getByRole('switch', { name: 'Speichern' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, keep);
    await page.keyboard.press('Space');
    await expect(keep).not.toBeChecked();
    await expect(dialog.getByText('Pro Film, nur während der Wiedergabe')).toBeVisible();

    // Escape drops the change: nothing was saved, the focus is back on the button.
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, open);
    await page.keyboard.press('Enter');
    await expect(movies.getByRole('radio', { name: 'Im Voraus' })).toBeChecked();

    // Series: ↓↓ to «Beim Start alle Folgen», Tab to the scope, ↓↓ to «Nächste Folge», save.
    await tabTo(page, series.getByRole('radio', { name: 'Im Voraus' }), 10);
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await expect(series.getByRole('radio', { name: 'Beim Start alle Folgen' })).toBeChecked();
    const scope = dialog.getByRole('radiogroup', { name: 'Umfang' });
    await page.keyboard.press('Tab');
    await expectFocusOn(page, scope.getByRole('radio', { name: 'Ganze Serie' }));
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await expect(scope.getByRole('radio', { name: 'Nächste Folge' })).toBeChecked();
    const save = dialog.getByRole('button', { name: 'Speichern', exact: true });
    await tabTo(page, save, 10);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(dialog).toBeHidden();
    await expect(
      page.getByText('Die Vorschau beim Spulen von Filme ist gespeichert.'),
    ).toBeVisible();
    await expect(
      foldOut(page, 'Filme').getByText(
        'Vorschau beim Spulen: Filme Im Voraus · Serien Beim Start alle Folgen',
      ),
    ).toBeVisible();
    const stored = await page.request.get(`/api/v1/admin/libraries/${id}`);
    expect(((await stored.json()) as { trickplay: unknown }).trickplay).toEqual({
      movies: { mode: 'ahead', keep: true },
      series: { mode: 'series_start', keep: true, scope: 'next_episode' },
    });
  } finally {
    // Other specs play the library with its setting from the start.
    const reset = await page.request.patch(`/api/v1/admin/libraries/${id}`, {
      headers: { 'X-Requested-With': 'Slouchcat' },
      data: {
        trickplay: {
          movies: { mode: 'ahead', keep: true },
          series: { mode: 'ahead', keep: true, scope: 'series' },
        },
      },
    });
    expect(reset.status(), await reset.text()).toBe(200);
  }
});

test('browser back leaves the page to the start page', async ({ page }) => {
  // Both pages must have rendered before going back: Firefox aborts a history navigation that
  // races the router's own history update on load (page.goBack: NS_ERROR_FAILURE).
  await page.goto('/');
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await page.goto('/admin/libraries');
  await expect(toggle(page, 'Filme')).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/$/);
});

test('"Scan abbrechen" shows while a scan is queued or running and cancels it', async ({
  page,
}) => {
  // Hold the list requests back, so the queued state the scan request answered with stays on
  // screen long enough to act on (the test library scans in well under a second), also past the
  // second in which the button ignores a cancel (a double press must not cancel the new scan).
  // The handler may still be waiting when the test unroutes it: then the request is gone already.
  const delay = async (route: Route) => {
    await new Promise((resolve) => setTimeout(resolve, 3000));
    await route.continue().catch(() => undefined);
  };
  await openRow(page, 'Filme');
  await page.route('**/api/v1/admin/libraries', delay);
  const filme = foldOut(page, 'Filme');
  await page.keyboard.press('Tab');
  await expectFocusOn(page, filme.getByRole('button', { name: 'Erneut scannen' }));
  await page.keyboard.press('Enter');
  // The same button turns into «Scan abbrechen» and keeps the focus.
  const cancel = filme.getByRole('button', { name: 'Scan abbrechen' });
  await expectFocusOn(page, cancel);
  await expect(page.getByText('Live, alle 5 Sekunden')).toBeVisible();
  await expect(
    page.getByRole('status').getByText('Der Scan von Filme ist gestartet.'),
  ).toBeVisible();
  // Enter at once is the double press the button ignores; a moment later it cancels.
  await page.keyboard.press('Enter');
  await page.waitForTimeout(1_200);
  const cancelled = page.waitForResponse(
    (response) => response.request().method() === 'DELETE' && response.url().endsWith('/scan'),
  );
  await page.keyboard.press('Enter');
  const response = await cancelled;
  expect(response.status()).toBe(202);
  const library = (await response.json()) as { scan: { state: string; lastError?: string } };
  await page.unroute('**/api/v1/admin/libraries', delay);

  // The cancel either stopped the scan, or the (small) scan finished just before it arrived;
  // anything else, or a finished scan with an error, is a failure.
  expect(['aborted', 'done']).toContain(library.scan.state);
  if (library.scan.state === 'done') expect(library.scan.lastError).toBeUndefined();
  if (library.scan.state === 'aborted') {
    expect(library.scan.lastError).toBe('cancelled');
    await expect(
      page.getByRole('status').getByText('Der Scan von Filme ist abgebrochen.'),
    ).toBeVisible();
    await expect(
      filme.getByText('Der Scan wurde abgebrochen. Der nächste Scan läuft wieder normal.'),
    ).toBeVisible();
    // A cancelled scan needs no loss acceptance: a video library has no menu then.
    await expect(filme.getByRole('button', { name: 'Weitere Aktionen für Filme' })).toHaveCount(0);
  }
  await expect(cancel).toHaveCount(0);

  // Leave the library scanned for the other specs; the button has the focus still.
  const rescan = filme.getByRole('button', { name: 'Erneut scannen' });
  await expectFocusOn(page, rescan);
  await page.keyboard.press('Enter');
  await expect(rowOf(page, 'Filme').getByText('Aktuell')).toBeVisible({ timeout: 60_000 });
});
