import { type Page } from '@playwright/test';
import { CSRF, EXTRAS_TITLE, H264_TITLE, SHOW_TITLE } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { episodeIds, itemIdByTitle, useFreshProfile } from './playback';

// Concept 20 P-14: the page «Gesehen», one list per profile. Each test uses a fresh profile, so
// its check marks are its own.

const grid = (page: Page) => page.getByRole('list', { name: 'Gesehen' });
const cards = (page: Page) => grid(page).getByRole('link');
const check = (page: Page, title: string) =>
  grid(page).getByRole('button', { name: `Gesehen: ${title}`, exact: true });
const nav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });

/** Sets or removes the check mark of a title for the session's profile (setItemWatched). */
async function setWatched(page: Page, itemId: string, watched: boolean): Promise<void> {
  const response = await page.request.put(`/api/v1/items/${itemId}/watched`, {
    headers: CSRF,
    data: { watched },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

test('the page by keyboard: empty text, filter and sort chips, cards, the check takes a film off, the half check asks @keyboard', async ({
  page,
  owner,
}) => {
  await useFreshProfile(page, owner, 'E2E Gesehen');
  const h264 = await itemIdByTitle(owner, H264_TITLE);
  const extras = await itemIdByTitle(owner, EXTRAS_TITLE);
  const show = await itemIdByTitle(owner, SHOW_TITLE);
  const [pilot] = await episodeIds(page.request, show);

  // The entry below «Radio» leads to the page; nothing watched yet.
  await page.goto('/');
  await nav(page).getByRole('link', { name: 'Gesehen', exact: true }).click();
  await expect(page).toHaveURL(/\/watched$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Gesehen' })).toBeVisible();
  await expect(nav(page).getByRole('link', { name: 'Gesehen', exact: true })).toHaveAttribute(
    'aria-current',
    'page',
  );
  await expect(page.getByText('Noch nichts gesehen.')).toBeVisible();
  await expect(page.getByText('Was du zu Ende schaust oder hörst, erscheint hier.')).toBeVisible();

  // Two films and one episode of the show: the show stands once, half checked.
  await setWatched(page, h264, true);
  await setWatched(page, extras, true);
  await setWatched(page, pilot, true);
  await page.reload();
  await expect(page.getByText('3 Titel')).toBeVisible();
  // Newest first (the default «Zuletzt gesehen»).
  await expect(cards(page)).toHaveCount(3);
  await expect(cards(page).first()).toContainText(SHOW_TITLE);
  await expect(cards(page).first()).toContainText(/1 von \d+ Folgen · zuletzt /);
  await expect(cards(page).nth(1)).toContainText(EXTRAS_TITLE);
  await expect(cards(page).nth(1)).toContainText(/gesehen am /);
  await expect(check(page, SHOW_TITLE)).toHaveAttribute('aria-pressed', 'mixed');
  await expect(check(page, H264_TITLE)).toHaveAttribute('aria-pressed', 'true');

  // Tab: back, the kind chips, the sort chips, then the first card and its check.
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 40);
  const kinds = page.getByRole('group', { name: 'Medienart' });
  await tabTo(page, kinds.getByRole('button', { name: 'Filme' }), 3);
  await page.keyboard.press('Enter');
  await expect(kinds.getByRole('button', { name: 'Filme' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(cards(page)).toHaveCount(2);
  await expect(page.getByText('2 Titel')).toBeVisible();
  // ← to «Alle» (arrow keys walk the chips), Enter.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, kinds.getByRole('button', { name: 'Alle' }));
  await page.keyboard.press('Enter');
  await expect(cards(page)).toHaveCount(3);
  const sorts = page.getByRole('group', { name: 'Sortierung' });
  await tabTo(page, sorts.getByRole('button', { name: 'Titel A–Z' }), 8);
  await page.keyboard.press('Space');
  await expect(cards(page).first()).toContainText(EXTRAS_TITLE);
  await expect(cards(page).last()).toContainText(SHOW_TITLE);

  // The card, then its check (no pen for this profile), then its watchlist button.
  await tabTo(page, cards(page).first(), 6);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, check(page, EXTRAS_TITLE));
  // Arrows walk the cards.
  await cards(page).first().focus();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards(page).nth(1));

  // A film's full check goes without a question; the card leaves, the focus moves to the next.
  await check(page, EXTRAS_TITLE).focus();
  await page.keyboard.press('Enter');
  await expect(page.getByRole('alertdialog')).toHaveCount(0);
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).first()).toContainText(H264_TITLE);
  await expect(cards(page).first()).toBeFocused();
  await expectVisibleFocus(page);
  await expect(page.getByText('2 Titel')).toBeVisible();

  // The half check of the show asks: mark all (focused) or remove all; Escape gives the focus
  // back.
  const showCheck = check(page, SHOW_TITLE);
  await showCheck.focus();
  await page.keyboard.press('Enter');
  const ask = page.getByRole('alertdialog', { name: `«${SHOW_TITLE}» ist teilweise gesehen` });
  await expect(ask).toBeVisible();
  await expectFocusOn(page, ask.getByRole('button', { name: 'Alle Folgen als gesehen markieren' }));
  await page.keyboard.press('Escape');
  await expect(ask).toHaveCount(0);
  await expectFocusOn(page, showCheck);
  // Again: «Alle Haken entfernen» takes the show off the list.
  await page.keyboard.press('Space');
  await expect(ask).toBeVisible();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, ask.getByRole('button', { name: 'Alle Haken entfernen' }));
  await page.keyboard.press('Enter');
  await expect(ask).toHaveCount(0);
  await expect(cards(page)).toHaveCount(1);
  await expect(page.getByText('1 Titel')).toBeVisible();
  await expectNoFocusTrap(page);

  // Enter on a card opens the title's page.
  await cards(page).first().focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/items/${h264}$`));
});

test('a series marked whole stands with a full check and opens the series page', async ({
  page,
  owner,
}) => {
  await useFreshProfile(page, owner, 'E2E Gesehen Serie');
  const show = await itemIdByTitle(owner, SHOW_TITLE);
  await setWatched(page, show, true);
  await page.goto('/watched');
  await expect(cards(page)).toHaveCount(1);
  await expect(check(page, SHOW_TITLE)).toHaveAttribute('aria-pressed', 'true');
  // Every episode: «n von n Folgen» with a full bar.
  const caption = await cards(page).first().textContent();
  const [, seen, total] = /(\d+) von (\d+) Folgen/.exec(caption ?? '') ?? [];
  expect(seen).toBeDefined();
  expect(seen).toBe(total);
  await cards(page).first().click();
  await expect(page).toHaveURL(new RegExp(`/items/${show}$`));
  await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
});

test('kind and order stay on the way back from a title', async ({ page, owner }) => {
  await useFreshProfile(page, owner, 'E2E Gesehen Zurück');
  await setWatched(page, await itemIdByTitle(owner, H264_TITLE), true);
  await page.goto('/watched');
  const kinds = page.getByRole('group', { name: 'Medienart' });
  const sorts = page.getByRole('group', { name: 'Sortierung' });
  await kinds.getByRole('button', { name: 'Filme' }).click();
  await sorts.getByRole('button', { name: 'Titel A–Z' }).click();
  await expect(page).toHaveURL(/\/watched\?(content=movie&sort=title|sort=title&content=movie)$/);
  await cards(page).first().click();
  await expect(page).toHaveURL(/\/items\//);
  await page.goBack();
  await expect(kinds.getByRole('button', { name: 'Filme' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(sorts.getByRole('button', { name: 'Titel A–Z' })).toHaveAttribute(
    'aria-pressed',
    'true',
  );
  await expect(cards(page)).toHaveCount(1);
});
