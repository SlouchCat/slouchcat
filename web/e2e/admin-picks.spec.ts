import type { APIRequestContext, Page } from '@playwright/test';
import { createProfile, loginOwner } from './accounts';
import { CSRF, H264_TITLE } from './catalog';
import { deletePick, recommendItem, titleId } from './curation';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// /admin/picks «User-Empfehlungen» (concept 18 part B, 14.7, ADR 0028) by keyboard (rule 8): the
// retention in days (7 to 365), the titles the profiles of the owner's account recommend now,
// newest first, anonymous (count and day only), and «Entfernen» after a question. A pick reaches
// every profile that may see the title, a child profile too, but never one above its rating (E-3).

const SUBTITLES_TITLE = 'Slouchcat Test Subtitles';
const PGS_TITLE = 'Slouchcat Test PGS';
const MULTI_AUDIO_TITLE = 'Slouchcat Test Multi Audio';

const picksRow = (page: Page) => page.getByRole('region', { name: 'User-Empfehlungen' });
const table = (page: Page) => page.getByRole('table', { name: 'Aktuelle Empfehlungen' });
const rowOf = (page: Page, title: string) =>
  table(page)
    .getByRole('row')
    .filter({ has: page.getByRole('rowheader').getByText(title, { exact: true }) });

/** Titles of the picks row the session's profile gets from `GET /home`. */
async function pickTitles(api: APIRequestContext): Promise<string[]> {
  const response = await api.get('/api/v1/home');
  expect(response.status(), await response.text()).toBe(200);
  const home = (await response.json()) as {
    rows: { key: string; items: { title: string }[] }[];
  };
  return home.rows.find((row) => row.key === 'picks')?.items.map((item) => item.title) ?? [];
}

async function setRating(owner: APIRequestContext, itemId: string, rating: number | null) {
  const response = await owner.put(`/api/v1/admin/items/${itemId}/rating`, {
    headers: CSRF,
    data: { rating },
  });
  expect(response.status(), await response.text()).toBe(200);
}

async function setRetention(owner: APIRequestContext, retentionDays: number) {
  const response = await owner.put('/api/v1/admin/picks/settings', {
    headers: CSRF,
    data: { retentionDays },
  });
  expect(response.status(), await response.text()).toBe(200);
}

test('user picks page by keyboard: retention, the list, remove after a question @keyboard', async ({
  page,
  owner,
}) => {
  test.setTimeout(90_000);
  const subtitles = await titleId(owner, SUBTITLES_TITLE);
  await recommendItem(owner, subtitles);
  try {
    await page.goto('/admin/picks');
    const title = page.getByRole('heading', { level: 1, name: 'User-Empfehlungen' });
    await expect(title).toBeVisible();
    // Newest first, one pick, 60 days; no profile is named.
    await expect(table(page).getByRole('rowheader').first()).toContainText(SUBTITLES_TITLE);
    await expect(rowOf(page, SUBTITLES_TITLE)).toContainText('1 Empfehlung');
    await expect(rowOf(page, SUBTITLES_TITLE)).toContainText('in 60 Tagen');
    await expect(rowOf(page, H264_TITLE)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Hinzufügen' })).toHaveCount(0);

    // Tab to the retention, a value outside 7..365 is refused at the field, 30 is stored.
    await resetFocus(page);
    const field = page.getByRole('spinbutton', { name: 'Empfehlungen bleiben' });
    await tabTo(page, field, 40);
    await expectVisibleFocus(page);
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type('400');
    await page.keyboard.press('Enter');
    await expect(field).toHaveAttribute('aria-invalid', 'true');
    await page.keyboard.press('ControlOrMeta+a');
    await page.keyboard.type('30');
    await page.keyboard.press('Enter');
    await expect(page.getByText('Empfehlungen bleiben jetzt 30 Tage.')).toBeAttached();
    await expect(rowOf(page, SUBTITLES_TITLE)).toContainText('in 30 Tagen');

    // «Entfernen» of the row: a question first; Escape keeps the pick and the focus.
    const remove = page.getByRole('button', { name: `${SUBTITLES_TITLE} entfernen` });
    await tabTo(page, remove, 40);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', {
      name: `${SUBTITLES_TITLE} aus den User-Empfehlungen entfernen?`,
    });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toHaveCount(0);
    await expectFocusOn(page, remove);
    await page.keyboard.press('Enter');
    await tabTo(page, dialog.getByRole('button', { name: 'Entfernen' }), 4);
    await page.keyboard.press('Enter');
    await expect(rowOf(page, SUBTITLES_TITLE)).toHaveCount(0);
    await expect(page.getByText(`${SUBTITLES_TITLE} ist nicht mehr empfohlen.`)).toBeAttached();
    // The row and its button are gone: the h1 holds the focus, Tab goes on from there.
    await expect(title).toBeFocused();
    await expectNoFocusTrap(page);
    expect(await pickTitles(owner)).not.toContain(SUBTITLES_TITLE);
  } finally {
    await setRetention(owner, 60);
    await deletePick(owner, subtitles);
  }
});

test('a pick reaches a child profile that may see the title, never one above its rating (E-3)', async ({
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
    await recommendItem(owner, pgs);
    await recommendItem(owner, multi);
    const ownerPicks = await pickTitles(owner);
    expect(ownerPicks.slice(0, 2)).toEqual([MULTI_AUDIO_TITLE, PGS_TITLE]);

    // The child: the PGS film first, never the 18 title; its star shows (a profile of the owner's
    // account, U-F6), and it has not recommended the film itself.
    await loginOwner(page.request);
    const selected = await page.request.post(`/api/v1/auth/profiles/${child.id}/select`, {
      headers: CSRF,
    });
    expect(selected.status(), await selected.text()).toBe(200);
    const childPicks = await pickTitles(page.request);
    expect(childPicks[0]).toBe(PGS_TITLE);
    expect(childPicks).not.toContain(MULTI_AUDIO_TITLE);
    await page.goto('/');
    await expect(picksRow(page).getByRole('list').getByRole('link').first()).toContainText(
      PGS_TITLE,
    );
    await expect(picksRow(page).getByText(MULTI_AUDIO_TITLE)).toHaveCount(0);
    const hidden = await page.request.put(`/api/v1/items/${multi}/pick`, { headers: CSRF });
    expect(hidden.status()).toBe(404);
  } finally {
    await deletePick(owner, pgs);
    await deletePick(owner, multi);
    await setRating(owner, pgs, null);
    await setRating(owner, multi, null);
  }
});
