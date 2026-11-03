import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { createProfile, loginOwner } from './accounts';
import { CSRF, H264_TITLE } from './catalog';
import { createPick, titleId } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/picks (M7, E-2; concept 12, variant C) by keyboard (rule 8): add a title for every
// profile with a note, choose its place at the front of the picks row, give it a time window in
// the past and in the future in its fold-out, find it in the tabs «Abgelaufen» and «Geplant»,
// leave with unsaved input only after a question, remove it; a pick for one profile reaches only
// that profile, and a title rated 18 never reaches a profile rated 12 (E-3). The picks are a
// poster row on the start page, never the stage (M11, plan 3.5); the spotlight band above them
// runs with reduced motion, so the page stands still.

const SUBTITLES_TITLE = 'Slouchcat Test Subtitles';
const PGS_TITLE = 'Slouchcat Test PGS';
const MULTI_AUDIO_TITLE = 'Slouchcat Test Multi Audio';

const picksRow = (page: Page) => page.getByRole('region', { name: "Cat's Empfehlungen" });
// The first card: the row's header also links its page ("Alle anzeigen ›", concept 08).
const firstPick = (page: Page) => picksRow(page).getByRole('list').getByRole('link').first();

/** Titles of the picks row the session's profile gets from `GET /home`. */
async function pickTitles(api: APIRequestContext): Promise<string[]> {
  const response = await api.get('/api/v1/home');
  expect(response.status(), await response.text()).toBe(200);
  const home = (await response.json()) as {
    rows: { key: string; items: { title: string }[] }[];
  };
  return home.rows.find((row) => row.key === 'picks')?.items.map((item) => item.title) ?? [];
}

/** A `datetime-local` value `days` from now (local time, minutes). */
function localInput(days: number): string {
  const date = new Date(Date.now() + days * 86_400_000);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}`;
}

async function setRating(owner: APIRequestContext, itemId: string, rating: number | null) {
  const response = await owner.put(`/api/v1/admin/items/${itemId}/rating`, {
    headers: CSRF,
    data: { rating },
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** The table of the open tab. */
const picksTable = (page: Page) =>
  page.getByRole('table', { name: 'Empfehlungen in der Reihenfolge der Reihe' });

/** The row of `title` in the open tab. */
const pickRow = (page: Page, title: string) =>
  picksTable(page)
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader').getByText(title, { exact: true }) });

/** The button in the row header of `title` that folds the row out. */
const pickToggle = (page: Page, title: string) =>
  pickRow(page, title).getByRole('rowheader').getByRole('button');

/** The fold-out of `title`, found by the toggle's `aria-controls` (it follows a move). */
async function foldOut(page: Page, title: string): Promise<Locator> {
  const id = await pickToggle(page, title).getAttribute('aria-controls');
  expect(id, `fold-out of ${title}`).toBeTruthy();
  return page.locator(`[id="${id}"]`);
}

/** Folds out the row of `title` by keyboard: Tab from the top of the page to its toggle, Enter. */
async function openByKeyboard(page: Page, title: string): Promise<Locator> {
  const toggle = pickToggle(page, title);
  await resetFocus(page);
  await tabTo(page, toggle, 60);
  await page.keyboard.press('Enter');
  await expect(toggle).toHaveAttribute('aria-expanded', 'true');
  return foldOut(page, title);
}

/** The «Stelle» of `title` in its fold-out. */
async function placeOf(page: Page, title: string): Promise<Locator> {
  return (await foldOut(page, title)).getByRole('combobox', { name: `Platz für ${title}` });
}

test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
});

test('picks page by keyboard: add with a note, place at the top, time window in the fold-out, tabs, remove @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(120_000);
  const note = 'E2E: unbedingt mit Untertiteln schauen';
  await page.goto('/admin/picks');
  const heading = page.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" });
  await expect(heading).toBeVisible();
  await expect(
    page
      .getByRole('navigation', { name: 'Verwaltung' })
      .getByRole('link', { name: "Cat's Empfehlungen" }),
  ).toHaveAttribute('aria-current', 'page');
  const tabs = page.getByRole('tablist', { name: 'Empfehlungen nach Zeitraum' });
  await expect(tabs.getByRole('tab', { selected: true })).toHaveAccessibleName(
    /^Alle \d+ Empfehlung(en)?$/,
  );
  // The global setup's pick of the H264 film.
  await expect(pickRow(page, H264_TITLE)).toHaveCount(1);

  // "Hinzufügen": Escape closes the dialog onto its button; then search, choose, note, Enter.
  await resetFocus(page);
  const add = page.getByRole('button', { name: 'Hinzufügen' });
  await tabTo(page, add);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Empfehlung hinzufügen' });
  await expect(dialog).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, add);
  await page.keyboard.press('Enter');
  await expect(dialog).toBeVisible();
  const search = dialog.getByRole('searchbox', { name: 'Titel suchen' });
  await expectFocusOn(page, search);
  await page.keyboard.type('Slouchcat Test Subt');
  const choose = dialog.getByRole('button', { name: `${SUBTITLES_TITLE} wählen`, exact: true });
  await expect(choose).toBeVisible();
  await tabTo(page, choose, 10);
  await page.keyboard.press('Enter');
  await expect(dialog.getByText(new RegExp(`^Gewählt: ${SUBTITLES_TITLE}`))).toBeVisible();
  await expectFocusOn(page, search);
  await expect(dialog.getByRole('checkbox', { name: 'Alle Profile' })).toBeChecked();
  const noteField = dialog.getByRole('textbox', { name: 'Notiz' });
  await tabTo(page, noteField, 20);
  await page.keyboard.type(note);
  const created = page.waitForResponse(
    (response) => response.request().method() === 'POST' && response.url().endsWith('/admin/picks'),
  );
  await page.keyboard.press('Enter');
  expect((await created).status()).toBe(201);
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText(`${SUBTITLES_TITLE} ist in Cat's Empfehlungen.`)).toBeVisible();
  await expectFocusOn(page, add);
  await expect(pickRow(page, SUBTITLES_TITLE)).toContainText(note);
  await expect(pickRow(page, SUBTITLES_TITLE)).toContainText('Alle');

  // ↑/↓ move between the rows (DataTable): from the new title's toggle to the other's and back;
  // Enter folds the new pick out.
  await resetFocus(page);
  await tabTo(page, pickToggle(page, SUBTITLES_TITLE), 60);
  await arrowTo(page, pickToggle(page, H264_TITLE), 'ArrowUp');
  await expectVisibleFocus(page);
  await arrowTo(page, pickToggle(page, SUBTITLES_TITLE), 'ArrowDown');
  await page.keyboard.press('Enter');
  await expect(pickToggle(page, SUBTITLES_TITLE)).toHaveAttribute('aria-expanded', 'true');

  // «Stelle» by keyboard: ↑ puts it first, the others move along, the choice keeps the focus.
  let place = await placeOf(page, SUBTITLES_TITLE);
  await tabTo(page, place, 20);
  await expect(place).toHaveValue('1');
  await expect(place).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
  const reorder = () =>
    page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/admin/picks/order'),
    );
  let reordered = reorder();
  await page.keyboard.press('ArrowUp');
  expect((await reordered).status()).toBe(204);
  await expect(picksTable(page).getByRole('rowheader').first()).toHaveText(
    new RegExp(`^${SUBTITLES_TITLE}`),
  );
  place = await placeOf(page, SUBTITLES_TITLE);
  await expectFocusOn(page, place);
  await expect(place).toHaveValue('0');
  await expect(page.getByText(`${SUBTITLES_TITLE} steht jetzt an Stelle 1.`)).toBeVisible();
  expect((await pickTitles(owner)).slice(0, 2)).toEqual([SUBTITLES_TITLE, H264_TITLE]);

  // ↓ and ↑ again, each saved at once.
  for (const [key, order] of [
    ['ArrowDown', [H264_TITLE, SUBTITLES_TITLE]],
    ['ArrowUp', [SUBTITLES_TITLE, H264_TITLE]],
  ] as const) {
    reordered = reorder();
    await page.keyboard.press(key);
    expect((await reordered).status()).toBe(204);
    await expect(picksTable(page).getByRole('rowheader').first()).toHaveText(
      new RegExp(`^${order[0]}`),
    );
    await expectFocusOn(page, await placeOf(page, SUBTITLES_TITLE));
    expect((await pickTitles(owner)).slice(0, 2)).toEqual(order);
  }

  // The start page: the new first pick with its note on its card; back returns to the admin page.
  await page.goto('/');
  await expect(firstPick(page)).toContainText(SUBTITLES_TITLE);
  await expect(firstPick(page).getByText(note)).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/picks$/);

  // A time window, saved in the fold-out. Date inputs have a Tab stop per part (day, month,
  // year, hours, minutes): the value is filled once the field has the focus.
  const saveWindow = async (from: string, to: string) => {
    const open = await openByKeyboard(page, SUBTITLES_TITLE);
    const fromField = open.getByLabel('Sichtbar ab');
    await tabTo(page, fromField, 10);
    await fromField.fill(from);
    await expectVisibleFocus(page);
    const toField = open.getByLabel('Sichtbar bis');
    await tabTo(page, toField, 10);
    await toField.fill(to);
    await expectVisibleFocus(page);
    await expect(pickToggle(page, SUBTITLES_TITLE)).toContainText('geändert');
    const save = open.getByRole('button', { name: `${SUBTITLES_TITLE} speichern` });
    const patched = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' && /\/admin\/picks\/\d+$/.test(response.url()),
    );
    await tabTo(page, save, 20);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(
      page.getByRole('status').getByText(`${SUBTITLES_TITLE} ist gespeichert.`),
    ).toBeVisible();
    await expect(pickToggle(page, SUBTITLES_TITLE)).not.toContainText('geändert');
    // The draft is spent: «Verwerfen» goes, «Speichern» keeps the focus.
    await expect(
      open.getByRole('button', { name: `Änderungen an ${SUBTITLES_TITLE} verwerfen` }),
    ).toHaveCount(0);
    await expectFocusOn(page, save);
    // Folded again, for the next round.
    await resetFocus(page);
    await tabTo(page, pickToggle(page, SUBTITLES_TITLE), 60);
    await page.keyboard.press('Enter');
    await expect(pickToggle(page, SUBTITLES_TITLE)).toHaveAttribute('aria-expanded', 'false');
  };
  /** Moves the selected tab with → to `name`, where the pick is listed, and with ← back. */
  const visitTab = async (name: string, steps: number) => {
    await resetFocus(page);
    await tabTo(page, tabs.getByRole('tab', { selected: true }), 30);
    for (let step = 0; step < steps; step += 1) await page.keyboard.press('ArrowRight');
    await expect(tabs.getByRole('tab', { selected: true })).toHaveText(new RegExp(`^${name}`));
    await expectVisibleFocus(page);
    await expect(page).toHaveURL(/\?tab=/);
    await expect(pickRow(page, SUBTITLES_TITLE)).toHaveCount(1);
    for (let step = 0; step < steps; step += 1) await page.keyboard.press('ArrowLeft');
    await expect(tabs.getByRole('tab', { selected: true })).toHaveText(/^Alle/);
    await expect(page).toHaveURL(/\/admin\/picks$/);
  };

  // Without a window the pick is in the tab «Aktiv».
  await visitTab('Aktiv', 1);

  // A window in the past: listed as expired, in the tab «Abgelaufen», gone from the picks row.
  await saveWindow(localInput(-2), localInput(-1));
  await expect(pickRow(page, SUBTITLES_TITLE).getByText('Abgelaufen')).toBeVisible();
  expect(await pickTitles(owner)).not.toContain(SUBTITLES_TITLE);
  await visitTab('Abgelaufen', 3);

  // A window in the future: listed as scheduled, in the tab «Geplant», not shown yet.
  await saveWindow(localInput(1), localInput(2));
  await expect(pickRow(page, SUBTITLES_TITLE).getByText('Geplant')).toBeVisible();
  expect(await pickTitles(owner)).not.toContain(SUBTITLES_TITLE);
  await visitTab('Geplant', 2);
  await page.goto('/');
  await expect(firstPick(page)).toContainText(H264_TITLE);
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/picks$/);

  // Unsaved input: leaving asks first and starts on «Auf der Seite bleiben»; Escape stays, the
  // focus goes back to the link and the input is kept. «Verwerfen» drops it.
  const detail = await openByKeyboard(page, SUBTITLES_TITLE);
  const editNote = detail.getByRole('textbox', { name: 'Notiz' });
  await tabTo(page, editNote, 10);
  await page.keyboard.press('End');
  await page.keyboard.type(' (neu)');
  // The hint under the list links the start page settings.
  const homeRows = page.getByRole('main').getByRole('link', { name: 'Startseite', exact: true });
  await expect(homeRows).toHaveAttribute('href', '/admin/home-rows');
  await tabTo(page, homeRows, 40);
  await page.keyboard.press('Enter');
  const leave = page.getByRole('alertdialog', {
    name: 'Seite mit ungespeicherten Änderungen verlassen?',
  });
  await expectFocusOn(page, leave.getByRole('button', { name: 'Auf der Seite bleiben' }));
  await page.keyboard.press('Escape');
  await expect(leave).toHaveCount(0);
  await expectFocusOn(page, homeRows);
  await expect(page).toHaveURL(/\/admin\/picks$/);
  await expect(editNote).toHaveValue(`${note} (neu)`);
  await expect(pickToggle(page, SUBTITLES_TITLE)).toContainText('geändert');
  const discard = detail.getByRole('button', {
    name: `Änderungen an ${SUBTITLES_TITLE} verwerfen`,
  });
  await resetFocus(page);
  await tabTo(page, discard, 80);
  await page.keyboard.press('Enter');
  await expect(editNote).toHaveValue(note);
  await expect(discard).toHaveCount(0);
  await expectFocusOn(page, detail.getByRole('button', { name: `${SUBTITLES_TITLE} speichern` }));

  // "Entfernen" (red) with its confirmation: Escape keeps the pick, Enter removes it; the row
  // goes, the focus to the heading.
  const remove = detail.getByRole('button', { name: `${SUBTITLES_TITLE} entfernen` });
  await tabTo(page, remove, 20);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', {
    name: `${SUBTITLES_TITLE} aus Cat's Empfehlungen entfernen?`,
  });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
  await page.keyboard.press('Escape');
  await expect(confirm).toHaveCount(0);
  await expectFocusOn(page, remove);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
  await page.keyboard.press('Enter');
  await expect(pickRow(page, SUBTITLES_TITLE)).toHaveCount(0);
  await expect(page.getByText(`${SUBTITLES_TITLE} ist nicht mehr empfohlen.`)).toBeVisible();
  await expect(confirm).toHaveCount(0);
  await expect(heading).toBeFocused();
  await expectNoFocusTrap(page);

  // Without unsaved input the hint's link leaves at once: the start page settings, where the
  // order of the rows lives; Back returns, and from the link Tab still leaves the page.
  await resetFocus(page);
  await tabTo(page, homeRows, 60);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/home-rows$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Startseite' })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/picks$/);
  await expect(homeRows).toBeVisible();
  await resetFocus(page);
  await tabTo(page, homeRows, 60);
  await expectNoFocusTrap(page);
});

test('a pick for one profile reaches only that profile; an 18 title never a profile rated 12', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const child = await createProfile(owner, { name: 'E2E Picks 12', maxRating: 12 });
  const pgs = await titleId(owner, PGS_TITLE);
  const multi = await titleId(owner, MULTI_AUDIO_TITLE);
  try {
    // The profile rated 12 may see the PGS film; the multi-audio film is rated 18 (E-3).
    await setRating(owner, pgs, 12);
    await setRating(owner, multi, 18);

    // The detail page's link (`?add=`) opens the dialog with the title; the pick is for the child.
    await page.goto(`/admin/picks?add=${pgs}`);
    const dialog = page.getByRole('dialog', { name: 'Empfehlung hinzufügen' });
    await expect(dialog.getByText(new RegExp(`^Gewählt: ${PGS_TITLE}`))).toBeVisible();
    const childBox = dialog.getByRole('checkbox', { name: 'E2E Picks 12' });
    await tabTo(page, childBox, 20);
    await page.keyboard.press('Space');
    await expect(childBox).toBeChecked();
    await expect(dialog.getByRole('checkbox', { name: 'Alle Profile' })).not.toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, dialog.getByRole('button', { name: 'Hinzufügen' }), 20);
    const created = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' && response.url().endsWith('/admin/picks'),
    );
    await page.keyboard.press('Enter');
    expect((await created).status()).toBe(201);
    await expect(dialog).toHaveCount(0);
    await expect(page).toHaveURL(/\/admin\/picks$/);
    const pgsRow = page
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader').getByText(PGS_TITLE, { exact: true }) });
    await expect(pgsRow).toContainText('E2E Picks 12');

    // The 18 title for everyone, first in the picks row.
    const [multiPick] = await createPick(owner, { itemId: multi });
    const order = ((await (await owner.get('/api/v1/admin/picks')).json()) as { id: number }[]).map(
      (pick) => pick.id,
    );
    const reordered = await owner.put('/api/v1/admin/picks/order', {
      headers: CSRF,
      data: { ids: [multiPick.id, ...order.filter((id) => id !== multiPick.id)] },
    });
    expect(reordered.status(), await reordered.text()).toBe(204);

    // The main profile sees the 18 title and the baseline pick, not the child's pick.
    const ownerPicks = await pickTitles(owner);
    expect(ownerPicks).toContain(MULTI_AUDIO_TITLE);
    expect(ownerPicks).toContain(H264_TITLE);
    expect(ownerPicks).not.toContain(PGS_TITLE);

    // The child sees only its own pick: no slide of the 18 title, none of the unrated film.
    await loginOwner(page.request);
    const selected = await page.request.post(`/api/v1/auth/profiles/${child.id}/select`, {
      headers: CSRF,
    });
    expect(selected.status(), await selected.text()).toBe(200);
    expect(await pickTitles(page.request)).toEqual([PGS_TITLE]);
    await page.goto('/');
    await expect(firstPick(page)).toContainText(PGS_TITLE);
    await expect(picksRow(page).getByText(MULTI_AUDIO_TITLE)).toHaveCount(0);
    await expect(picksRow(page).getByRole('list').getByRole('link')).toHaveCount(1);
  } finally {
    await setRating(owner, pgs, null);
    await setRating(owner, multi, null);
  }
});
