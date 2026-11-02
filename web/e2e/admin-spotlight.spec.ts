import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/spotlight (M11, ADR 0011; concept 12, variant A, mockup Spo-A) by keyboard (rule 8): the
// page is reached through the admin navigation; the three bands fold by Enter on their title; in
// «Quelle» the switch, the list kinds (arrow keys; box office locks the media to films) and the
// numbers are walked with Tab and saved together; "Jetzt abrufen" queues the fetch; one entry of
// the Trakt mock's list is hidden and shown, pinned on a place of the grid and unpinned, annotated
// and its note removed, all with Enter on the row's buttons and in their dialogs. ↑/↓ between the
// rows, Escape, the question before leaving with an unsaved source, visible focus, browser back,
// no trap. The tests leave the settings and the entry as they found them: the start page specs
// depend on the spotlight row.

/**
 * The first entry of the mock's trending list (trakt-server.mjs, TMDB 7001) and the one below it
 * (7002); the administration lists films before series, each in list order.
 */
const FILM = 'Slouchcat Test AC3 51';
const NEXT_FILM = 'Slouchcat Test DTS';
const FILM_PATH = '/admin/spotlight/entries/movie/7001';
/** Five films and two series in the mock's lists, all in the library. */
const MOCK_ENTRIES = 7;
const TAB_MAX = 200;

/** The writable part of the stored settings, to put back after the test. */
async function storedSettings(api: APIRequestContext): Promise<Record<string, unknown>> {
  const response = await api.get('/api/v1/admin/spotlight');
  expect(response.status(), await response.text()).toBe(200);
  const settings = (await response.json()) as Record<string, unknown>;
  // The catalogue is read only; the update takes the other fields.
  delete settings.listKinds;
  return settings;
}

async function restore(api: APIRequestContext, settings: Record<string, unknown>) {
  const put = await api.put('/api/v1/admin/spotlight', { headers: CSRF, data: settings });
  expect(put.status(), await put.text()).toBe(200);
}

/** Entries of the configured list the server holds (admin API). */
async function entryCount(api: APIRequestContext): Promise<number> {
  const response = await api.get('/api/v1/admin/spotlight/entries');
  expect(response.status(), await response.text()).toBe(200);
  return ((await response.json()) as { entries: unknown[] }).entries.length;
}

const answer = (page: Page, method: string, path: string) =>
  page.waitForResponse(
    (response) => response.request().method() === method && response.url().endsWith(path),
  );

const region = (page: Page, name: string) => page.getByRole('region', { name, exact: true });
const sidebar = (page: Page) => page.getByRole('navigation', { name: 'Verwaltung' });
const sidebarLink = (page: Page, name: string) =>
  sidebar(page).getByRole('link', { name: new RegExp(`^${name}(,|$)`) });

/** Tab from the top of the page to `target`. */
async function tabToFromTop(page: Page, target: Locator) {
  await resetFocus(page);
  await tabTo(page, target, TAB_MAX);
}

/** The sidebar is one Tab stop: Tab to its current entry, then ↓ until `name` has the focus. */
async function navTo(page: Page, current: string, name: string): Promise<void> {
  await tabToFromTop(page, sidebarLink(page, current));
  await arrowTo(page, sidebarLink(page, name), 'ArrowDown', 30);
  await expectFocusOn(page, sidebarLink(page, name));
}

async function openSpotlight(page: Page) {
  await page.goto('/admin/spotlight');
  await expect(page.getByRole('heading', { level: 1, name: 'Spotlight' })).toBeVisible();
  await expect(region(page, 'Quelle').getByRole('switch')).toBeVisible();
}

test('spotlight by keyboard: bands, source, fetch, hide, pin and note an entry', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const settings = await storedSettings(owner);
  try {
    // From the overview through the admin sidebar; Back returns there at the end.
    await page.goto('/admin');
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    await navTo(page, 'Übersicht', 'Spotlight');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/spotlight$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Spotlight' })).toBeVisible();
    await expect(
      sidebar(page).getByRole('link', { name: 'Spotlight', exact: true }),
    ).toHaveAttribute('aria-current', 'page');

    // The intro sits behind the help button of the title (S-2).
    const intro = page.getByText(/Eine öffentliche Trakt-Liste als Reihe der Startseite/);
    await expect(intro).toBeHidden();
    await tabToFromTop(page, page.getByRole('button', { name: 'Hilfe zu dieser Seite' }));
    await page.keyboard.press('Enter');
    await expect(intro).toBeVisible();

    // The band «Quelle» folds by Enter and opens by Space; folded, the next Tab stop is the next
    // band.
    const source = region(page, 'Quelle');
    const sourceBand = source.getByRole('button', { name: 'Quelle' });
    const entriesBand = region(page, 'Einträge der Liste').getByRole('button', {
      name: 'Einträge der Liste',
    });
    const show = source.getByRole('switch', { name: 'Auf der Startseite zeigen' });
    await tabTo(page, sourceBand, 5);
    await page.keyboard.press('Enter');
    await expect(sourceBand).toHaveAttribute('aria-expanded', 'false');
    await expect(show).toBeHidden();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, entriesBand);
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, sourceBand);
    await page.keyboard.press('Space');
    await expect(sourceBand).toHaveAttribute('aria-expanded', 'true');
    await expect(show).toBeVisible();

    // start-server.mjs sets the Trakt mock's key; the foot says so and links to the
    // integrations, the key itself never shows here.
    await expect(source.getByRole('link', { name: 'Schlüssel gesetzt' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );

    // Tab to the switch and on to the list; the arrow keys choose a kind (one per line: ↓ and ↑). Box office offers films
    // only (listKinds), and back on Trending all three media return.
    const media = source.getByRole('radiogroup', { name: 'Medienart' });
    await tabTo(page, show, 5);
    const trending = source.getByRole('radio', { name: 'Trending bei Trakt' });
    await tabTo(page, trending, 5);
    await expect(trending).toBeChecked();
    await page.keyboard.press('ArrowDown');
    await expect(source.getByRole('radio', { name: 'Beliebt bei Trakt' })).toBeChecked();
    await expectVisibleFocus(page);
    await expect(source.getByText('Ungespeicherte Änderungen')).toBeVisible();
    for (let presses = 0; presses < 4; presses += 1) await page.keyboard.press('ArrowDown');
    await expect(source.getByRole('radio', { name: 'Kinocharts' })).toBeChecked();
    await expect(media.getByRole('radio')).toHaveCount(1);
    await expect(media.getByRole('radio', { name: 'Nur Filme' })).toBeChecked();
    await expect(media.getByRole('radio', { name: 'Nur Filme' })).toBeDisabled();
    for (let presses = 0; presses < 5; presses += 1) await page.keyboard.press('ArrowUp');
    await expect(trending).toBeChecked();
    await expect(media.getByRole('radio')).toHaveCount(3);
    await expect(media.getByRole('radio', { name: 'Filme und Serien' })).toBeChecked();
    // Back on the stored kind, nothing is unsaved.
    await expect(source.getByText('Ungespeicherte Änderungen')).toHaveCount(0);

    // The four numbers, each with its own line (plan 5.5); Tab through them to "Speichern". The
    // band's rhythm (concept 08, F-10) goes from its value to 7 seconds.
    for (const label of ['Höchstens', 'Ausblenden unter', 'Abruf alle … Stunden']) {
      await tabTo(page, source.getByRole('spinbutton', { name: label, exact: true }), 5);
    }
    const rotation = source.getByRole('spinbutton', {
      name: 'Takt der Bühne … Sekunden',
      exact: true,
    });
    await tabTo(page, rotation, 5);
    await expect(rotation).toHaveValue(String(settings.rotationSeconds));
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type('7');
    const save = source.getByRole('button', { name: 'Speichern' });
    await tabTo(page, save, 5);
    const stored = answer(page, 'PUT', '/admin/spotlight');
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    const said = page.getByRole('status').filter({ hasText: 'Spotlight ist gespeichert.' });
    await expect(said).toBeVisible();
    await expectVisibleFocus(page);
    // Stored: the page shows it after a reload, and the start page's band takes it (F-10).
    await page.reload();
    await expect(rotation).toHaveValue('7');
    await expect
      .poll(
        async () =>
          ((await (await owner.get('/api/v1/home')).json()) as { heroIntervalSeconds?: number })
            .heroIntervalSeconds,
        { timeout: 60_000, intervals: [1000] },
      )
      .toBe(7);

    // "Jetzt abrufen": 202 queues the fetch; 429 means another spec fetched within the last minute
    // (one manual fetch a minute, discover.ts refreshSpotlight). Either answer shows its notice.
    const refresh = source.getByRole('button', { name: 'Jetzt abrufen' });
    await tabToFromTop(page, refresh);
    const queued = answer(page, 'POST', '/admin/spotlight/refresh');
    await page.keyboard.press('Enter');
    expect([202, 429]).toContain((await queued).status());
    await expect(
      page
        .getByRole('status')
        .or(page.getByRole('alert'))
        .filter({
          hasText:
            /^Die Liste wird abgerufen\.$|^Die Liste wurde vor weniger als einer Minute abgerufen\./,
        }),
    ).toBeVisible();
    await expectVisibleFocus(page);

    // The mock's seven titles, all in the library; the page shows what the server holds.
    await expect
      .poll(() => entryCount(owner), { timeout: 60_000, intervals: [1000] })
      .toBe(MOCK_ENTRIES);
    await page.reload();
    const table = page.getByRole('table', { name: 'Einträge der Liste' });
    await expect(table.getByRole('rowheader')).toHaveCount(MOCK_ENTRIES);
    await expect(table.getByText('Nicht in der Bibliothek', { exact: true })).toHaveCount(0);
    // A `has` locator is resolved inside the row, so it must not start at the table.
    const row = table.getByRole('row').filter({ has: page.getByText(FILM, { exact: true }) });
    await expect(row).toContainText('Sichtbar');

    // "Ausblenden" by Enter; the button turns into "Einblenden". ↓ to the next row and ↑ back,
    // then "Einblenden" by Enter.
    const pin = table.getByRole('button', { name: `${FILM} anheften` });
    const hide = table.getByRole('button', { name: `${FILM} ausblenden` });
    const showEntry = table.getByRole('button', { name: `${FILM} einblenden` });
    await tabToFromTop(page, hide);
    let patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(row).toContainText('Ausgeblendet');
    await expect(page.getByText(`${FILM} ist ausgeblendet.`)).toBeVisible();
    await expectFocusOn(page, showEntry);
    // ↓/↑ land on the first button of the same cell in the other row.
    await arrowTo(page, table.getByRole('button', { name: `${NEXT_FILM} anheften` }));
    await expectVisibleFocus(page);
    await arrowTo(page, pin, 'ArrowUp');
    await tabTo(page, showEntry, 3);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(row).toContainText('Sichtbar');
    await expectFocusOn(page, hide);

    // "Anheften": the dialog offers the places of the row as a grid up to «Höchstens»; the
    // arrow keys choose one, Enter on "Speichern" stores it. "Lösen" takes the pin off.
    const pinDialog = page.getByRole('dialog', { name: `${FILM} anheften` });
    const places = pinDialog.getByRole('radiogroup', { name: 'Platz in der Reihe' });
    const unpin = table.getByRole('button', { name: `${FILM} lösen` });
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, pin);
    await page.keyboard.press('Enter');
    await expect(pinDialog).toBeVisible();
    await expect(places.getByRole('radio')).toHaveCount(Number(settings.itemCount));
    await expectFocusOn(page, places.getByRole('radio', { name: '1', exact: true }));
    await page.keyboard.press('ArrowRight');
    await expect(places.getByRole('radio', { name: '2', exact: true })).toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, pinDialog.getByRole('button', { name: 'Speichern' }), 3);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    const pinned = await patched;
    expect(pinned.status()).toBe(200);
    expect(pinned.request().postDataJSON()).toEqual({ pinnedRank: 2 });
    await expect(pinDialog).toHaveCount(0);
    await expect(row).toContainText('Angeheftet auf Platz 2');
    await expectFocusOn(page, unpin);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    const unpinned = await patched;
    expect(unpinned.status()).toBe(200);
    expect(unpinned.request().postDataJSON()).toEqual({ pinnedRank: null });
    await expect(row).not.toContainText('Angeheftet');
    await expectFocusOn(page, pin);

    // "Notiz", the column before the actions: set one, then empty the field to remove it.
    const noteButton = table.getByRole('button', { name: `Notiz zu ${FILM}` });
    const noteDialog = page.getByRole('dialog', { name: `Notiz zu ${FILM}` });
    const noteField = noteDialog.getByRole('textbox', { name: 'Notiz' });
    const note = 'E2E: auf der grossen Leinwand';
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, noteButton);
    await page.keyboard.press('Enter');
    await expect(noteDialog).toBeVisible();
    await expectFocusOn(page, noteField);
    await page.keyboard.type(note);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(noteDialog).toHaveCount(0);
    await expect(row).toContainText(`«${note}»`);
    await expectFocusOn(page, noteButton);

    await page.keyboard.press('Enter');
    await expectFocusOn(page, noteField);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.press('Backspace');
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    const removed = await patched;
    expect(removed.status()).toBe(200);
    expect(removed.request().postDataJSON()).toEqual({ note: null });
    await expect(noteDialog).toHaveCount(0);
    await expect(row).not.toContainText(note);
    await expectFocusOn(page, noteButton);

    // Escape closes the dialog without saving; the focus returns to its button.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, noteField);
    await page.keyboard.press('Escape');
    await expect(noteDialog).toHaveCount(0);
    await expectFocusOn(page, noteButton);

    // The footer leads to the row settings of the start page.
    await expect(page.getByRole('link', { name: 'Zur Seite «Startseite»' })).toHaveAttribute(
      'href',
      '/admin/home-rows',
    );
    await expectNoFocusTrap(page, TAB_MAX);

    await page.goBack();
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  } finally {
    // The settings as before and no override left on the film.
    await restore(owner, settings);
    const reset = await owner.patch(`/api/v1${FILM_PATH}`, { headers: CSRF, data: {} });
    expect([200, 404], await reset.text()).toContain(reset.status());
  }
});

test('leaving with an unsaved source asks first; staying keeps it, leaving stores nothing', async ({
  page,
  owner,
}) => {
  const settings = await storedSettings(owner);
  try {
    await openSpotlight(page);
    const source = region(page, 'Quelle');
    const show = source.getByRole('switch', { name: 'Auf der Startseite zeigen' });
    const before = await show.isChecked();
    await tabToFromTop(page, show);
    await page.keyboard.press('Space');
    await expect(show).toBeChecked({ checked: !before });
    await expect(source.getByText('Ungespeicherte Änderungen')).toBeVisible();

    // The sidebar link waits for the question, which starts on «Auf der Seite bleiben»; Enter
    // stays.
    const leave = page.getByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    const stay = leave.getByRole('button', { name: 'Auf der Seite bleiben' });
    await navTo(page, 'Spotlight', 'Sammlungen');
    await page.keyboard.press('Enter');
    await expectFocusOn(page, stay);
    await page.keyboard.press('Enter');
    await expect(leave).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/spotlight$/);
    await expect(show).toBeChecked({ checked: !before });

    // Escape stays as well.
    await navTo(page, 'Spotlight', 'Sammlungen');
    await page.keyboard.press('Enter');
    await expectFocusOn(page, stay);
    await page.keyboard.press('Escape');
    await expect(leave).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/spotlight$/);

    // «Verwerfen und verlassen» goes; nothing was stored.
    await navTo(page, 'Spotlight', 'Sammlungen');
    await page.keyboard.press('Enter');
    await expectFocusOn(page, stay);
    await tabTo(page, leave.getByRole('button', { name: 'Verwerfen und verlassen' }), 3);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/collections$/);
    expect(await storedSettings(owner)).toEqual(settings);
    await expectNoFocusTrap(page, TAB_MAX);
  } finally {
    await restore(owner, settings);
  }
});
