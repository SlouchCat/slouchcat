import type { Locator, Page } from '@playwright/test';
import { CSRF, H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// Administration view of a title (B-6, B-8, B-9; concept 12 variant C): tabs Metadaten, Bilder,
// Dateien and Trailer in `?tab=`, «Herkunft je Feld» and «Freigabe» as disclosures, the match
// dialog (the e2e server has no TMDB key: the search answers with the provider error), manual
// trailers, the image choice and the question before leaving with an unsaved rating. Keyboard
// only (rule 8).

let detail = '';
let admin = '';

const TAB_MAX = 40;
const tabList = (page: Page) => page.getByRole('tablist', { name: 'Bereiche des Titels' });
const trailers = (page: Page) => page.getByRole('region', { name: 'Trailer' });
const heading = (page: Page) => page.getByRole('heading', { level: 1, name: H264_TITLE });

/** From the top of the page to the selected tab, then with the arrow keys to `name`. */
async function selectTab(page: Page, name: RegExp, steps: number): Promise<void> {
  await resetFocus(page);
  await tabTo(page, tabList(page).getByRole('tab', { selected: true }), TAB_MAX);
  const key = steps < 0 ? 'ArrowLeft' : 'ArrowRight';
  for (let step = 0; step < Math.abs(steps); step += 1) await page.keyboard.press(key);
  await expect(tabList(page).getByRole('tab', { selected: true })).toHaveText(name);
}

test.beforeEach(async ({ page }) => {
  if (!admin) {
    detail = await itemPath(page, H264_TITLE);
    admin = `/admin/items/${idOf(detail)}`;
  }
  await page.goto(admin);
  await expect(heading(page)).toBeVisible();
});

test.afterEach(async ({ owner }) => {
  // A trailer left by a failed run would make "Löschen" ambiguous here and add a trailer button
  // to the detail page of the other specs.
  if (!admin) return;
  const item = (await (await owner.get(`/api/v1${admin}`)).json()) as {
    trailers: { id: number }[];
  };
  for (const trailer of item.trailers) {
    await owner.delete(`/api/v1${admin}/trailers/${trailer.id}`, { headers: CSRF });
  }
});

test('shows kind, state, both version files and the trailer state', async ({ page }) => {
  const facts = page.getByRole('region', { name: 'Metadaten' });
  await expect(facts.getByRole('term').filter({ hasText: /^Art$/ })).toBeVisible();
  await expect(facts.getByRole('definition').filter({ hasText: /^Film$/ })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zur Detailseite' })).toHaveAttribute('href', detail);

  await selectTab(page, /^Dateien/, 2);
  const files = page.getByRole('region', { name: 'Dateien' });
  await expect(files.getByRole('row')).toHaveCount(3);
  // The row shows the file's name and its folder; Enter folds out the whole path.
  const short = files.getByRole('button', { name: /^Slouchcat Test H264 \(2024\)\.mp4 / });
  await tabTo(page, short, 10);
  await expect(short).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(short).toHaveAttribute('aria-expanded', 'true');
  await expect(files.getByText(/\/Slouchcat Test H264 \(2024\)\.mp4$/)).toBeVisible();
  await expect(
    files.getByRole('button', { name: /^Slouchcat Test H264 \(2024\) - 720p\.mp4 / }),
  ).toBeVisible();

  await selectTab(page, /^Trailer/, 1);
  await expect(trailers(page)).toBeVisible();
  await expect(
    trailers(page).getByText(/^(Suche ausstehend|Gefunden|Keiner gefunden|Suche fehlgeschlagen)$/),
  ).toBeVisible();
});

test('Back from the detail page returns to the tab of the address', async ({ page }) => {
  await selectTab(page, /^Dateien/, 2);
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=files$`));
  const view = page.getByRole('link', { name: 'Zur Detailseite' });
  await resetFocus(page);
  await tabTo(page, view, TAB_MAX);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=files$`));
  await expect(heading(page)).toBeVisible();
  await expect(tabList(page).getByRole('tab', { selected: true })).toHaveText(/^Dateien/);
  await expect(page.getByRole('region', { name: 'Dateien' })).toBeVisible();
});

test("the detail page links here through the pen's editor (concept 15 7.4)", async ({ page }) => {
  await page.goto(detail);
  await tabTo(page, page.getByRole('button', { name: 'Metadaten bearbeiten', exact: true }), 60);
  await page.keyboard.press('Enter');
  const link = page.getByRole('dialog').getByRole('link', { name: 'In der Verwaltung öffnen' });
  await tabTo(page, link, 10);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
  await expect(page.getByRole('dialog')).toHaveCount(0);
});

test('match dialog: the search without a key shows the provider error; Escape closes', async ({
  page,
}) => {
  const match = page.getByRole('button', { name: 'Zuordnen' });
  await tabTo(page, match);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Titel zuordnen' });
  await expect(dialog).toBeVisible();
  const query = dialog.getByLabel('Suchbegriff');
  await expectFocusOn(page, query);
  await expect(query).toHaveValue(H264_TITLE);
  await page.keyboard.press('Enter');
  await expect(
    dialog.getByText(/TMDB hat den Schlüssel abgelehnt|Dieser Anbieter ist nicht eingerichtet/),
  ).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, match);
});

test('add a manual trailer by keyboard, then delete it after confirmation', async ({ page }) => {
  await selectTab(page, /^Trailer/, 3);
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=trailers$`));
  const add = trailers(page).getByRole('button', { name: 'Hinzufügen' });
  await tabTo(page, add, 10);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  const source = dialog.getByLabel('YouTube-Link oder Video-ID');
  await expectFocusOn(page, source);
  await page.keyboard.type('https://www.youtube.com/watch?v=dQw4w9WgXcQ');
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toBeVisible();
  await expect(trailers(page).getByText('Manuell')).toBeVisible();
  await expect(page.getByText('Trailer hinzugefügt.')).toBeVisible();
  await expect(tabList(page).getByRole('tab', { selected: true })).toHaveText(/^Trailer\s*1/);

  const remove = trailers(page).getByRole('button', { name: 'Löschen' });
  await tabTo(page, remove, 10);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Trailer löschen?' });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Löschen' }));
  await page.keyboard.press('Enter');
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toHaveCount(0);
  await expect(page.getByText('Trailer gelöscht.')).toBeVisible();
  // The row went with its button: the h1 takes the focus (ALL-2).
  await expect(heading(page)).toBeFocused();
});

test('an invalid YouTube link is reported at the field', async ({ page }) => {
  await selectTab(page, /^Trailer/, 3);
  await tabTo(page, trailers(page).getByRole('button', { name: 'Hinzufügen' }), 10);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  const source = dialog.getByLabel('YouTube-Link oder Video-ID');
  await expectFocusOn(page, source);
  await page.keyboard.type('https://example.com/video');
  await page.keyboard.press('Enter');
  await expect(source).toHaveAttribute('aria-invalid', 'true');
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
});

test('Tab: header, actions, tabs by arrow keys, disclosures by Enter; no focus trap @keyboard', async ({
  page,
}) => {
  // The admin sidebar's Tab stop is its current page: a title's page belongs to "Pflege".
  await tabTo(
    page,
    page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: /^Pflege(,|$)/ }),
  );
  await tabTo(page, page.getByRole('button', { name: 'Zuordnen' }), 12);
  await tabTo(page, page.getByRole('button', { name: 'Aktualisieren' }), 2);
  await tabTo(page, page.getByRole('link', { name: 'Zur Detailseite' }), 2);

  // The tabs move with the arrow keys; the selection lives in `?tab=`.
  const selected = tabList(page).getByRole('tab', { selected: true });
  await tabTo(page, selected, 4);
  await expect(selected).toHaveText('Metadaten');
  await page.keyboard.press('ArrowRight');
  await expect(selected).toHaveText(/^Bilder/);
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=images$`));
  await page.keyboard.press('ArrowRight');
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=files$`));
  await page.keyboard.press('ArrowRight');
  await expect(selected).toHaveText(/^Trailer/);
  await expect(page).toHaveURL(new RegExp(`${admin}\\?tab=trailers$`));
  for (let presses = 0; presses < 3; presses += 1) await page.keyboard.press('ArrowLeft');
  await expect(selected).toHaveText('Metadaten');
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
  await expectFocusOn(page, selected);

  // «Freigabe» folds out with Enter and in again.
  const rating = page.getByRole('button', { name: /^Freigabe/ });
  await tabTo(page, rating, 10);
  await expect(rating).toHaveAttribute('aria-expanded', 'false');
  await page.keyboard.press('Enter');
  await expect(rating).toHaveAttribute('aria-expanded', 'true');
  await expect(page.getByRole('radiogroup', { name: 'Eigene Freigabe setzen' })).toBeVisible();
  await page.keyboard.press('Enter');
  await expect(rating).toHaveAttribute('aria-expanded', 'false');
  await expect(page.getByRole('radiogroup', { name: 'Eigene Freigabe setzen' })).toBeHidden();
  await expectNoFocusTrap(page);
});

test('original language: chosen by keys, saved and locked; «Aus Metadaten» lifts the lock (concept 13 11.7) @keyboard', async ({
  page,
  owner,
}) => {
  const url = `/api/v1${admin}/original-language`;
  const stored = async () =>
    (await (await owner.get(`/api/v1${admin}`)).json()) as {
      originalLanguage?: string;
      lockedFields: string[];
    };
  /** Moves the closed select to `value` with ↑ or ↓ only. */
  const pick = async (select: Locator, value: string) => {
    await expect(select).toBeFocused();
    const steps = await select.evaluate((element, wanted) => {
      const field = element as HTMLSelectElement;
      return (
        [...field.options].findIndex((option) => option.value === wanted) - field.selectedIndex
      );
    }, value);
    for (let step = 0; step < Math.abs(steps); step += 1)
      await page.keyboard.press(steps > 0 ? 'ArrowDown' : 'ArrowUp');
    await expect(select).toHaveValue(value);
  };
  try {
    const language = page.getByRole('button', { name: /^Originalsprache/ });
    await tabTo(page, language, TAB_MAX);
    await expect(language).toHaveAttribute('aria-expanded', 'false');
    await page.keyboard.press('Enter');
    await expect(language).toHaveAttribute('aria-expanded', 'true');
    const panel = page.getByRole('group', { name: /^Originalsprache/ });
    const select = panel.getByRole('combobox', { name: 'Originalsprache setzen' });
    const save = panel.getByRole('button', { name: 'Speichern' });
    // The codes come from the metadata settings once the disclosure is open.
    await expect(select.getByRole('option', { name: 'Japanisch' })).toHaveCount(1);
    await tabTo(page, select, 4);
    await expect(select).toHaveValue('metadata');
    await pick(select, 'ja');
    await expect(
      page.getByRole('tabpanel').getByText('ungespeichert', { exact: true }),
    ).toBeVisible();
    await tabTo(page, save, 2);
    await page.keyboard.press('Enter');
    await expect(page.getByText('Originalsprache gespeichert.')).toBeVisible();
    await expect(panel.getByText('Von Hand (gesperrt)')).toBeVisible();
    await expect(select).toHaveValue('ja');
    let item = await stored();
    expect(item.originalLanguage).toBe('ja');
    expect(item.lockedFields).toContain('original_language');

    // Back to «Aus Metadaten»: the lock goes and the language waits for the next lookup.
    await resetFocus(page);
    await tabTo(page, select, TAB_MAX);
    await pick(select, 'metadata');
    await tabTo(page, save, 2);
    await page.keyboard.press('Enter');
    await expect(panel.getByText('Noch nicht abgefragt', { exact: true })).toBeVisible();
    item = await stored();
    expect(item.originalLanguage).toBeUndefined();
    expect(item.lockedFields).not.toContain('original_language');
    await expectNoFocusTrap(page);
  } finally {
    await owner.put(url, { headers: CSRF, data: { language: null } });
  }
});

test('an unknown tab in the address shows «Metadaten» and leaves the address', async ({ page }) => {
  await page.goto(`${admin}?tab=nonsense`);
  await expect(heading(page)).toBeVisible();
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
  await expect(tabList(page).getByRole('tab', { selected: true })).toHaveText('Metadaten');
});

test('an unsaved rating survives the tabs; leaving asks first', async ({ page }) => {
  const rating = page.getByRole('button', { name: /^Freigabe/ });
  await tabTo(page, rating, TAB_MAX);
  await page.keyboard.press('Enter');
  const group = page.getByRole('radiogroup', { name: 'Eigene Freigabe setzen' });
  const checked = group.getByRole('radio', { checked: true });
  const before = await checked.getAttribute('value');
  await tabTo(page, checked, 10);
  await page.keyboard.press('ArrowDown');
  await expect(checked).not.toHaveAttribute('value', before ?? '');
  const choice = await checked.getAttribute('value');
  // The chip next to «Freigabe», and the mark on the tab that every tab shows.
  const unsaved = page.getByRole('tabpanel').getByText('ungespeichert', { exact: true });
  await expect(unsaved).toBeVisible();
  const marked = tabList(page).getByRole('tab', { name: 'Metadaten ungespeichert' });
  await expect(marked).toBeVisible();

  // Another tab and back: the choice is still there and not saved.
  await selectTab(page, /^Bilder/, 1);
  await expect(marked).toBeVisible();
  await selectTab(page, /^Metadaten/, -1);
  await expect(unsaved).toBeVisible();
  await expect(group.getByRole('radio', { checked: true })).toHaveAttribute('value', choice ?? '');

  // The link to the detail page waits for the question, whose default stays on the page.
  const view = page.getByRole('link', { name: 'Zur Detailseite' });
  await resetFocus(page);
  await tabTo(page, view, TAB_MAX);
  await page.keyboard.press('Enter');
  const question = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expectFocusOn(page, question.getByRole('button', { name: 'Auf der Seite bleiben' }));
  await page.keyboard.press('Enter');
  await expect(question).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
  await expect(unsaved).toBeVisible();

  // Escape stays as well; «Verwerfen und verlassen» goes.
  await expectFocusOn(page, view);
  await page.keyboard.press('Enter');
  await expect(question).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(question).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
  await expectFocusOn(page, view);
  await page.keyboard.press('Enter');
  const leave = question.getByRole('button', { name: 'Verwerfen und verlassen' });
  await tabTo(page, leave, 3);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test.describe('image choice', () => {
  // Chromium sends the page's GET requests through the service worker, where page.route does not
  // see them; the candidates and the reread title are answered here, so the worker stays out.
  test.use({ serviceWorkers: 'block' });

  test('image choice: the grid by arrow keys, Enter chooses, Escape closes, focus returns (concept 10 7.3)', async ({
    page,
    owner,
  }) => {
    // The e2e server has no TMDB key and the test film no image file next to it: the candidates and
    // the choice are answered here; the server side is covered by the Go tests of package metadata.
    const stored = (await (await owner.get(`/api/v1${admin}`)).json()) as Record<string, unknown>;
    const candidates = [
      {
        provider: 'local',
        lang: '',
        width: 600,
        height: 900,
        score: 0,
        voteCount: 0,
        source: 'file:A/poster.jpg?1-1',
        previewUrl: '',
        chosen: false,
      },
      {
        provider: 'tmdb',
        lang: 'de',
        width: 1000,
        height: 1500,
        score: 5.5,
        voteCount: 10,
        source: '/de.jpg',
        previewUrl: '',
        chosen: false,
      },
      {
        provider: 'tmdb',
        lang: 'en',
        width: 2000,
        height: 3000,
        score: 6,
        voteCount: 20,
        source: '/en.jpg',
        previewUrl: '',
        chosen: false,
      },
    ];
    let chosen: unknown = null;
    await page.route(
      (url) => url.pathname === `/api/v1${admin}/image-candidates`,
      (route) =>
        route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({ candidates, unavailable: [] }),
        }),
    );
    await page.route(
      (url) => url.pathname === `/api/v1${admin}/images/poster`,
      async (route) => {
        chosen = route.request().postDataJSON();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...stored,
            images: [
              {
                kind: 'poster',
                provider: 'tmdb',
                lang: 'de',
                source: '/de.jpg',
                chosen: true,
                locked: true,
              },
            ],
          }),
        });
      },
    );

    // After the choice the page reads the title again: that answer carries the choice too.
    await page.route(
      (url) => url.pathname === `/api/v1${admin}`,
      async (route) => {
        if (chosen === null || route.request().method() !== 'GET') return route.continue();
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            ...stored,
            images: [
              {
                kind: 'poster',
                provider: 'tmdb',
                lang: 'de',
                source: '/de.jpg',
                chosen: true,
                locked: true,
              },
            ],
          }),
        });
      },
    );

    await selectTab(page, /^Bilder/, 1);
    const images = page.getByRole('region', { name: 'Bilder' });
    const poster = images.getByRole('row').filter({ hasText: 'Poster' });
    const pick = poster.getByRole('button', { name: 'Andere wählen' });
    await tabTo(page, pick, 10);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Poster wählen' });
    const grid = dialog.getByRole('listbox', { name: 'Kandidaten für Poster' });
    const tiles = grid.getByRole('option');
    await expect(tiles).toHaveCount(3);
    await expectFocusOn(page, tiles.nth(0));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, tiles.nth(1));

    // Escape closes without a choice; the focus is back on the button.
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, pick);
    expect(chosen).toBeNull();

    await page.keyboard.press('Enter');
    await expectFocusOn(page, tiles.nth(0));
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(dialog).toHaveCount(0);
    expect(chosen).toEqual({ provider: 'tmdb', source: '/de.jpg' });
    await expect(
      page.getByText('Poster gewählt und gesperrt; das Bild wird geholt.'),
    ).toBeVisible();
    await expect(poster.getByText('von Hand gewählt')).toBeVisible();
    await expect(poster.getByRole('button', { name: 'Sperre aufheben' })).toBeVisible();
    await expectFocusOn(page, pick);
    await expectNoFocusTrap(page);
  });
});
