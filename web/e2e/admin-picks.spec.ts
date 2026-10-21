import type { APIRequestContext, Page } from '@playwright/test';
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

// /admin/picks (M7, E-2) by keyboard (rule 8): add a title for every profile with a note, move it
// to the top of the carousel, give it a time window in the past and in the future, remove it; a
// pick for one profile reaches only that profile, and a title rated 18 never reaches a profile
// rated 12 (E-3). The carousel runs with reduced motion, so its first slide stays in front.

const SUBTITLES_TITLE = 'Slouchcat Test Subtitles';
const PGS_TITLE = 'Slouchcat Test PGS';
const MULTI_AUDIO_TITLE = 'Slouchcat Test Multi Audio';

const hero = (page: Page) => page.getByRole('region', { name: "Cat's Empfehlungen" });
const heroTitle = (page: Page) => hero(page).getByRole('heading', { level: 2 });

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

test.beforeEach(async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
});

test('picks page by keyboard: add with a note, move to the top, time window, remove', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const note = 'E2E: unbedingt mit Untertiteln schauen';
  await page.goto('/admin/picks');
  await expect(page.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" })).toBeVisible();
  await expect(
    page
      .getByRole('navigation', { name: 'Hauptnavigation' })
      .getByRole('link', { name: "Cat's Empfehlungen" }),
  ).toHaveAttribute('aria-current', 'page');
  const table = page.getByRole('table', { name: 'Empfehlungen in der Reihenfolge des Karussells' });
  const row = (title: string) =>
    table
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader').getByText(title, { exact: true }) });
  // The global setup's pick of the H264 film.
  await expect(row(H264_TITLE)).toHaveCount(1);

  // "Hinzufügen": search, choose, note, Enter.
  await resetFocus(page);
  const add = page.getByRole('button', { name: 'Hinzufügen' });
  await tabTo(page, add);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Empfehlung hinzufügen' });
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
  await expect(row(SUBTITLES_TITLE)).toContainText(note);
  await expect(row(SUBTITLES_TITLE)).toContainText('Alle');

  // ↓/↑ move between the rows (DataTable): to the first enabled control of the row above ("Nach
  // oben" is disabled in the first row).
  await row(SUBTITLES_TITLE)
    .getByRole('button', { name: `${SUBTITLES_TITLE} bearbeiten` })
    .focus();
  await arrowTo(
    page,
    row(H264_TITLE).getByRole('button', { name: `${H264_TITLE} nach unten` }),
    'ArrowUp',
  );
  await expectVisibleFocus(page);

  // "Nach oben" by keyboard: the new pick leads the carousel; at the top the focus moves to
  // "Nach unten" of the same title.
  const up = row(SUBTITLES_TITLE).getByRole('button', { name: `${SUBTITLES_TITLE} nach oben` });
  await tabTo(page, up, 30);
  const reordered = page.waitForResponse(
    (response) =>
      response.request().method() === 'PUT' && response.url().endsWith('/admin/picks/order'),
  );
  await page.keyboard.press('Enter');
  expect((await reordered).status()).toBe(204);
  await expectFocusOn(
    page,
    row(SUBTITLES_TITLE).getByRole('button', { name: `${SUBTITLES_TITLE} nach unten` }),
  );
  await expect(table.getByRole('rowheader').first()).toHaveText(new RegExp(`^${SUBTITLES_TITLE}`));
  expect((await pickTitles(owner)).slice(0, 2)).toEqual([SUBTITLES_TITLE, H264_TITLE]);

  // The start page: the new first pick with its note in the hero; back returns to the admin page.
  await page.goto('/');
  await expect(heroTitle(page)).toHaveText(SUBTITLES_TITLE);
  await expect(hero(page).getByText(note)).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/admin\/picks$/);

  // A time window in the past: the pick stays listed as expired and leaves the carousel.
  const edit = row(SUBTITLES_TITLE).getByRole('button', { name: `${SUBTITLES_TITLE} bearbeiten` });
  const editDialog = page.getByRole('dialog', { name: `${SUBTITLES_TITLE} bearbeiten` });
  const saveWindow = async (from: string, to: string) => {
    // From "Hinzufügen" (Firefox starts Tab after a blurred element, so no resetFocus here).
    await add.focus();
    await tabTo(page, edit, 20);
    await page.keyboard.press('Enter');
    await expect(editDialog).toBeVisible();
    await expectFocusOn(page, editDialog.getByRole('textbox', { name: 'Notiz' }));
    await editDialog.getByLabel('Sichtbar ab').fill(from);
    await editDialog.getByLabel('Sichtbar bis').fill(to);
    const patched = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' && /\/admin\/picks\/\d+$/.test(response.url()),
    );
    // Date inputs have a Tab stop per part (day, month, year, hours, minutes).
    await tabTo(page, editDialog.getByRole('button', { name: 'Speichern' }), 20);
    await page.keyboard.press('Enter');
    expect((await patched).status()).toBe(200);
    await expect(editDialog).toHaveCount(0);
  };
  await saveWindow(localInput(-2), localInput(-1));
  await expect(row(SUBTITLES_TITLE).getByText('Abgelaufen')).toBeVisible();
  expect(await pickTitles(owner)).not.toContain(SUBTITLES_TITLE);

  // A time window in the future: listed as scheduled, not shown yet.
  await saveWindow(localInput(1), localInput(2));
  await expect(row(SUBTITLES_TITLE).getByText('Geplant')).toBeVisible();
  expect(await pickTitles(owner)).not.toContain(SUBTITLES_TITLE);
  await page.goto('/');
  await expect(heroTitle(page)).toHaveText(H264_TITLE);
  await page.goBack();

  // Escape closes a dialog without saving; the focus returns to its button.
  await add.focus();
  await tabTo(page, edit, 20);
  await page.keyboard.press('Enter');
  await expect(editDialog).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(editDialog).toHaveCount(0);
  await expectFocusOn(page, edit);

  // "Entfernen" with its confirmation; the row goes, the focus to the heading.
  const remove = row(SUBTITLES_TITLE).getByRole('button', { name: `${SUBTITLES_TITLE} entfernen` });
  await tabTo(page, remove, 5);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', {
    name: `${SUBTITLES_TITLE} aus Cat's Empfehlungen entfernen?`,
  });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Entfernen' }));
  await page.keyboard.press('Enter');
  await expect(row(SUBTITLES_TITLE)).toHaveCount(0);
  await expect(page.getByText(`${SUBTITLES_TITLE} ist nicht mehr empfohlen.`)).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 1, name: "Cat's Empfehlungen" }).locator('..'),
  ).toBeFocused();
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

    // The 18 title for everyone, first in the carousel.
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
    await expect(heroTitle(page)).toHaveText(PGS_TITLE);
    await expect(hero(page).getByText(MULTI_AUDIO_TITLE)).toHaveCount(0);
    await expect(hero(page).getByRole('button', { name: /^Empfehlung \d+ von/ })).toHaveCount(0);
  } finally {
    await setRating(owner, pgs, null);
    await setRating(owner, multi, null);
  }
});
