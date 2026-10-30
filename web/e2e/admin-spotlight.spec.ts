import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import { arrowTo } from './playback';

// /admin/spotlight (M11, ADR 0011, mockup H) by keyboard (rule 8): the page is reached through the
// admin navigation, the switch of the page head and the card "Quelle" are walked with Tab, the
// list kinds are chosen with the arrow keys (box office locks the media to films), the source is
// saved, "Jetzt abrufen" queues the fetch, and one entry of the Trakt mock's list is hidden and
// shown, pinned and unpinned, annotated and its note removed, all with Enter on the row's text
// buttons and in their dialogs. ↑/↓ between the rows, Escape, visible focus, browser back, no trap.
// The test leaves the settings and the entry as it found them: the start page specs depend on the
// spotlight row.

/**
 * The first entry of the mock's trending list (trakt-server.mjs, TMDB 7001) and the one below it
 * (7002); the administration lists films before series, each in list order.
 */
const FILM = 'Slouchcat Test AC3 51';
const NEXT_FILM = 'Slouchcat Test DTS';
const FILM_PATH = '/admin/spotlight/entries/movie/7001';
/** Five films and two series in the mock's lists, all in the library. */
const MOCK_ENTRIES = 7;

/** The writable part of the stored settings, to put back after the test. */
async function storedSettings(api: APIRequestContext): Promise<Record<string, unknown>> {
  const response = await api.get('/api/v1/admin/spotlight');
  expect(response.status(), await response.text()).toBe(200);
  const settings = (await response.json()) as Record<string, unknown>;
  // The catalogue is read only; the update takes the other fields.
  delete settings.listKinds;
  return settings;
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

/** Presses → in the admin navigation until `name` has the focus. */
async function navTo(page: Page, name: string): Promise<void> {
  const link = page
    .getByRole('navigation', { name: 'Verwaltung' })
    .getByRole('link', { name, exact: true });
  for (let presses = 0; presses < 20; presses += 1) {
    if (await link.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, link);
}

test('spotlight by keyboard: source, fetch, hide, pin and note an entry', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const settings = await storedSettings(owner);
  try {
    // From the overview through the admin navigation; Back returns there at the end.
    await page.goto('/admin');
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    const nav = page.getByRole('navigation', { name: 'Verwaltung' });
    await nav.locator('a[tabindex="0"]').focus();
    await navTo(page, 'Spotlight');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/spotlight$/);
    const heading = page.getByRole('heading', { level: 1, name: 'Spotlight' });
    await expect(heading).toBeVisible();
    // Tab starts at the page header (focusable with tabIndex -1): Firefox continues Tab after a
    // blurred element, so resetFocus alone would start below a control the test just used.
    const fromTop = () => heading.locator('..').focus();
    await expect(nav.getByRole('link', { name: 'Spotlight', exact: true })).toHaveAttribute(
      'aria-current',
      'page',
    );

    // The card "Quelle": start-server.mjs sets the Trakt mock's key; the foot says so and links to
    // the integrations, the key itself never shows here.
    const source = page.getByRole('region', { name: 'Quelle' });
    await expect(source.getByRole('link', { name: 'Schlüssel gesetzt' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );

    // Tab to the switch in the page head and on to the list; the arrow keys choose a kind. Box
    // office offers films only (listKinds), and back on Trending all three media return.
    const media = source.getByRole('radiogroup', { name: 'Medienart' });
    await fromTop();
    await tabTo(page, page.getByRole('switch', { name: 'Auf der Startseite zeigen' }), 60);
    const trending = source.getByRole('radio', { name: 'Trending bei Trakt' });
    await tabTo(page, trending, 5);
    await expect(trending).toBeChecked();
    await page.keyboard.press('ArrowRight');
    await expect(source.getByRole('radio', { name: 'Beliebt bei Trakt' })).toBeChecked();
    await expectVisibleFocus(page);
    for (let presses = 0; presses < 4; presses += 1) await page.keyboard.press('ArrowRight');
    await expect(source.getByRole('radio', { name: 'Kinocharts' })).toBeChecked();
    await expect(media.getByRole('radio')).toHaveCount(1);
    await expect(media.getByRole('radio', { name: 'Nur Filme' })).toBeChecked();
    await expect(media.getByRole('radio', { name: 'Nur Filme' })).toBeDisabled();
    for (let presses = 0; presses < 5; presses += 1) await page.keyboard.press('ArrowLeft');
    await expect(trending).toBeChecked();
    await expect(media.getByRole('radio')).toHaveCount(3);
    await expect(media.getByRole('radio', { name: 'Filme und Serien' })).toBeChecked();

    // The four numbers, each with its own line (plan 5.5); Tab through them to "Speichern". The
    // band's rhythm (concept 08, F-10) goes from 5 to 7 seconds.
    for (const label of ['Höchstens', 'Ausblenden unter', 'Abruf alle … Stunden']) {
      await tabTo(page, source.getByLabel(label, { exact: true }), 5);
    }
    const rotation = source.getByLabel('Takt der Bühne … Sekunden', { exact: true });
    await tabTo(page, rotation, 5);
    await expect(rotation).toHaveValue(String(settings.rotationSeconds));
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type('7');
    const save = source.getByRole('button', { name: 'Speichern' });
    await tabTo(page, save, 5);
    const stored = answer(page, 'PUT', '/admin/spotlight');
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(source.getByText('Spotlight ist gespeichert.')).toBeVisible();
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
    // (one manual fetch a minute, discover.ts refreshSpotlight). Both answers show their notice.
    const refresh = source.getByRole('button', { name: 'Jetzt abrufen' });
    await fromTop();
    await tabTo(page, refresh, 60);
    const queued = answer(page, 'POST', '/admin/spotlight/refresh');
    await page.keyboard.press('Enter');
    const answered = (await queued).status();
    expect([202, 429]).toContain(answered);
    await expect(
      source.getByText(
        answered === 202
          ? 'Die Liste wird abgerufen.'
          : 'Die Liste wurde vor weniger als einer Minute abgerufen. Warte kurz und versuche es erneut.',
      ),
    ).toBeVisible();
    await expectVisibleFocus(page);

    // The mock's seven titles, all in the library.
    await expect
      .poll(() => entryCount(owner), { timeout: 60_000, intervals: [1000] })
      .toBe(MOCK_ENTRIES);
    const table = page.getByRole('table', { name: 'Einträge der Liste' });
    if ((await table.getByRole('rowheader').count()) !== MOCK_ENTRIES) await page.reload();
    await expect(table.getByRole('rowheader')).toHaveCount(MOCK_ENTRIES);
    await expect(table.getByText('Nicht in der Bibliothek', { exact: true })).toHaveCount(0);
    // A `has` locator is resolved inside the row, so it must not start at the table.
    const row = table.getByRole('row').filter({ has: page.getByText(FILM, { exact: true }) });
    await expect(row).toContainText('Sichtbar');

    // "Ausblenden" by Enter; the button turns into "Einblenden". ↓ to the next row and ↑ back,
    // then "Einblenden" by Enter.
    const pin = table.getByRole('button', { name: `${FILM} anheften` });
    const hide = table.getByRole('button', { name: `${FILM} ausblenden` });
    const show = table.getByRole('button', { name: `${FILM} einblenden` });
    await fromTop();
    await tabTo(page, hide, 80);
    let patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(row).toContainText('Ausgeblendet');
    await expect(page.getByText(`${FILM} ist ausgeblendet.`)).toBeVisible();
    await expectFocusOn(page, show);
    await arrowTo(page, table.getByRole('button', { name: `${NEXT_FILM} anheften` }));
    await expectVisibleFocus(page);
    await arrowTo(page, pin, 'ArrowUp');
    await tabTo(page, show, 3);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(row).toContainText('Sichtbar');
    await expectFocusOn(page, hide);

    // "Anheften": the dialog asks for the place, Enter saves; "Lösen" takes the pin off.
    const pinDialog = page.getByRole('dialog', { name: `${FILM} anheften` });
    const unpin = table.getByRole('button', { name: `${FILM} lösen` });
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, pin);
    await page.keyboard.press('Enter');
    await expect(pinDialog).toBeVisible();
    await expectFocusOn(page, pinDialog.getByLabel(/Platz in der Reihe/));
    await page.keyboard.type('2');
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(pinDialog).toHaveCount(0);
    await expect(row).toContainText('Angeheftet auf Platz 2');
    await expectFocusOn(page, unpin);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    const unpinned = await patched;
    expect(unpinned.status()).toBe(200);
    expect(unpinned.request().postDataJSON()).toEqual({ pinnedRank: null });
    await expect(row).not.toContainText('Angeheftet');

    // "Notiz": set one, then empty the field to remove it.
    const noteButton = table.getByRole('button', { name: `Notiz zu ${FILM}` });
    const noteDialog = page.getByRole('dialog', { name: `Notiz zu ${FILM}` });
    const noteField = noteDialog.getByRole('textbox', { name: 'Notiz' });
    const note = 'E2E: auf der grossen Leinwand';
    const openNote = async () => {
      await hide.focus();
      await tabTo(page, noteButton, 3);
      await page.keyboard.press('Enter');
      await expect(noteDialog).toBeVisible();
      await expectFocusOn(page, noteField);
    };
    await openNote();
    await page.keyboard.type(note);
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(noteDialog).toHaveCount(0);
    await expect(row).toContainText(`«${note}»`);

    await openNote();
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.press('Backspace');
    patched = answer(page, 'PATCH', FILM_PATH);
    await page.keyboard.press('Enter');
    const removed = await patched;
    expect(removed.status()).toBe(200);
    expect(removed.request().postDataJSON()).toEqual({ note: null });
    await expect(noteDialog).toHaveCount(0);
    await expect(row).not.toContainText(note);

    // Escape closes the dialog without saving; the focus returns to its button.
    await openNote();
    await page.keyboard.press('Escape');
    await expect(noteDialog).toHaveCount(0);
    await expectFocusOn(page, noteButton);

    // The footer leads to the row settings of the start page.
    await expect(page.getByRole('link', { name: 'Zur Seite «Startseite»' })).toHaveAttribute(
      'href',
      '/admin/home-rows',
    );
    await expectNoFocusTrap(page);

    await page.goBack();
    await expect(page).toHaveURL(/\/admin$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  } finally {
    // The settings as before and no override left on the film.
    const put = await owner.put('/api/v1/admin/spotlight', { headers: CSRF, data: settings });
    expect(put.status(), await put.text()).toBe(200);
    const reset = await owner.patch(`/api/v1${FILM_PATH}`, { headers: CSRF, data: {} });
    expect([200, 404], await reset.text()).toContain(reset.status());
  }
});
