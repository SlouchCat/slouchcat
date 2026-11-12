import { expect, test, type Locator, type Page } from '@playwright/test';
import { CSRF, itemPath, SHOW_TITLE } from './catalog';
import { episodeIds, resetProgress } from './playback';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Series, seasons and episodes (B-1, B-2, C-2) by keyboard: start page → series → season →
// episode → player and back. Test media: Season 01 (S01E01, S01E02, S01E03-E04 as one file) and
// Specials (S00E01).

let series = '';

const shelf = (page: Page) => page.getByRole('list', { name: 'Staffeln' });
// The season cards, without the pens (concept 20 P-3) and the checks (P-5) on their covers.
const seasonCards = (page: Page) =>
  shelf(page).getByRole('button', { name: /^(?!.*: Metadaten bearbeiten$)(?!Gesehen: )/ });
const seasonPens = (page: Page) =>
  shelf(page).getByRole('button', { name: /: Metadaten bearbeiten$/ });
const seasonChecks = (page: Page) => shelf(page).getByRole('button', { name: /^Gesehen: / });
const episodes = (page: Page) => page.getByRole('list', { name: 'Folgen' });
// Each episode row: one link that plays it, then «Details» to its page (concept 20 P-4).
const playLinks = (page: Page) => episodes(page).getByRole('link', { name: /^Abspielen/ });
const detailsLinks = (page: Page) => episodes(page).getByRole('link', { name: /^Details: / });
// …and at its far right the check mark «gesehen» (concept 20 P-5).
const episodeChecks = (page: Page) => episodes(page).getByRole('button', { name: /^Gesehen: / });
// The big button of a series names its nextUp episode (concept 17 3.3); other specs may have left
// progress behind, so any of the four reasons.
// The rows of the episode list play too ("Abspielen S01E01 …", concept 20 P-4): not those.
const seriesPlay = (page: Page) =>
  page
    .getByRole('link', {
      name: /^(Abspielen|Weiterschauen|Nächste Folge|Nochmals ansehen) (S\d\dE\d\d|Alles gesehen)/,
    })
    .and(page.locator('xpath=//*[not(ancestor::ul[@aria-label="Folgen"])]'));
const episodePlay = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/ });

/**
 * Whether the season card stands wholly inside the shelf's scroll box (concept 20 P-1: the chosen
 * season moves into view by itself), whatever the page's own scroll position.
 */
const inShelf = (card: Locator) =>
  card.evaluate((element) => {
    const box = element.closest('ul')?.parentElement?.getBoundingClientRect();
    const place = element.getBoundingClientRect();
    return box !== undefined && place.left >= box.left - 1 && place.right <= box.right + 1;
  });

/** Index of the focused row (its play link) in the episode list, or -1. */
const focusedEpisode = (page: Page) =>
  playLinks(page).evaluateAll((links) =>
    links.indexOf(document.activeElement as HTMLAnchorElement),
  );

/** Sets the profile's position of an episode, so it counts as started (or as new at 0). */
async function setPosition(page: Page, itemId: string, positionMs: number): Promise<void> {
  if (positionMs === 0) return resetProgress(page.request, itemId, 10_000);
  const response = await page.request.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs,
      durationMs: 10_000,
      playedMs: positionMs,
      seq: 0,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

/** Sets or removes the check mark of a title (setItemWatched), as the round button does. */
async function setWatched(page: Page, itemId: string, watched: boolean): Promise<void> {
  const response = await page.request.put(`/api/v1/items/${itemId}/watched`, {
    headers: CSRF,
    data: { watched },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

/** The id of the first episode of Season 1. */
const firstEpisode = async (page: Page) =>
  (await episodeIds(page.request, series.split('/').pop() ?? ''))[0] ?? '';

test.beforeEach(async ({ page }) => {
  series ||= await itemPath(page, SHOW_TITLE);
  await page.goto('/');
  await page.goto(series);
  await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
  await expect(playLinks(page).first()).toBeVisible();
  await resetFocus(page);
});

test('the start page lists the series once and never its episodes', async ({ page }) => {
  await page.goto('/');
  const recent = page.getByRole('region', { name: 'Neu hinzugefügt' });
  await expect(recent.getByRole('link').filter({ hasText: SHOW_TITLE })).toHaveCount(1);
  await expect(recent.getByRole('link', { name: /S\d\dE\d\d|^Episode \d/ })).toHaveCount(0);
});

test('seasons: a shelf with Season 1 first, specials last; S01E03–E04 is one episode', async ({
  page,
}) => {
  await expect(seasonCards(page)).toHaveCount(2);
  await expect(seasonCards(page).first()).toHaveAttribute('aria-current', 'true');
  expect(await inShelf(seasonCards(page).first())).toBe(true);
  // Two seasons fit: the shelf's fixed paging buttons are gone (concept 20 P-1).
  const seasonsRegion = page.getByRole('region', { name: 'Staffeln' });
  await expect(
    seasonsRegion.getByRole('button', { name: /^(Zurück|Weiter)blättern$/ }),
  ).toHaveCount(0);
  await expect(seasonCards(page).first()).toContainText('3 Folgen');
  await expect(seasonCards(page).last()).toContainText(/Specials|Season 0|Staffel 0/);
  await expect(playLinks(page)).toHaveCount(3);
  await expect(detailsLinks(page)).toHaveCount(3);
  await expect(episodes(page).getByText('S01E01', { exact: true })).toBeVisible();
  await expect(episodes(page).getByText('S01E03–E04', { exact: true })).toBeVisible();
  // The big button plays the nextUp episode of the series (E-10).
  await expect(seriesPlay(page)).toHaveAttribute('href', /^\/play\//);
  await expect(page).toHaveTitle(`${SHOW_TITLE} · Slouchcat`);
});

test('arrows: play → round actions → season shelf → episodes; Enter on a season shows the specials', async ({
  page,
}) => {
  // The start focus is the big button (E-10).
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));

  for (let presses = 0; presses < 4; presses += 1) {
    if (await shelf(page).locator(':focus').count()) break;
    await page.keyboard.press('ArrowDown');
  }
  await expectFocusOn(page, seasonCards(page).first());
  await page.keyboard.press('ArrowRight');
  const specials = seasonCards(page).last();
  await expectFocusOn(page, specials);
  await page.keyboard.press('Enter');
  await expect(specials).toHaveAttribute('aria-current', 'true');
  expect(await inShelf(specials)).toBe(true);
  await expect(episodes(page).getByText('S00E01', { exact: true })).toBeVisible();
  await expect(playLinks(page)).toHaveCount(1);

  // Back to Season 1, then down into the episodes and along them.
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('Enter');
  await expect(playLinks(page)).toHaveCount(3);
  await page.keyboard.press('ArrowDown');
  await expect.poll(() => focusedEpisode(page)).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);
  const start = await focusedEpisode(page);
  await page.keyboard.press(start < 2 ? 'ArrowDown' : 'ArrowUp');
  await expect.poll(() => focusedEpisode(page)).toBe(start < 2 ? start + 1 : start - 1);
  await expectVisibleFocus(page);
});

test('Enter on an episode plays it; ← → and Tab reach «Details», which opens its page (concept 20 P-4) @keyboard', async ({
  page,
}) => {
  // Not started: the row plays at once.
  const id = await firstEpisode(page);
  await setPosition(page, id, 0);
  await page.reload();
  const first = playLinks(page).first();
  await expect(first).toHaveAttribute('href', `/play/${id}`);
  await first.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/play/${id}$`));
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`${series}$`));

  // → from the row to its «Details», ← back; Tab from the row reaches «Details» too.
  const details = detailsLinks(page).first();
  await expect(details).toHaveAccessibleName(/^Details: S01E01 · /);
  await playLinks(page).first().focus();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, details);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, playLinks(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, details);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/items/${id}$`));
  await expect(page.getByText(`S01E01 · ${SHOW_TITLE}`)).toBeVisible();

  // The episode page plays it as before; Backspace and Escape walk back.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, episodePlay(page));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`/items/${id}$`));
  await expect(page.getByText(`S01E01 · ${SHOW_TITLE}`)).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
});

test('a started episode asks first: resume has the focus, ↑ ↓ and Tab move, Escape gives the focus back, Ctrl-click opens a tab (concept 20 P-4) @keyboard', async ({
  page,
}) => {
  const id = await firstEpisode(page);
  await setPosition(page, id, 4000);
  try {
    await page.reload();
    const row = playLinks(page).first();
    await row.focus();
    await page.keyboard.press('Enter');
    const choice = page.getByRole('dialog', { name: /^S01E01 · / });
    await expect(choice).toBeVisible();
    await expect(choice.getByText('Du hast diese Folge angefangen.')).toBeVisible();
    await expectFocusOn(page, choice.getByRole('button', { name: 'Weiterschauen ab 0:04' }));
    await expect(page).toHaveURL(new RegExp(`${series}$`));

    // Escape: nothing plays, the focus is back on the row.
    await page.keyboard.press('Escape');
    await expect(choice).toHaveCount(0);
    await expectFocusOn(page, row);
    await expect(page).toHaveURL(new RegExp(`${series}$`));

    // Ctrl-click (Cmd on a Mac) is the browser's: a new tab, no question. The tab never loads,
    // so it reports no progress.
    const context = page.context();
    const blank = /\/play\/[^/?]+$/;
    await context.route(blank, (route) =>
      route.request().resourceType() === 'document' ? route.abort() : route.fallback(),
    );
    const [tab] = await Promise.all([
      context.waitForEvent('page'),
      row.click({ modifiers: ['ControlOrMeta'] }),
    ]);
    await tab.close();
    await context.unroute(blank);
    await expect(choice).toHaveCount(0);
    await expect(page).toHaveURL(new RegExp(`${series}$`));

    // Tab stays in the dialog: «Von vorn starten», «Abbrechen», back to the default.
    await row.focus();
    await page.keyboard.press('Enter');
    await expect(choice).toBeVisible();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Von vorn starten' }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Weiterschauen ab 0:04' }));
    await page.keyboard.press('Shift+Tab');
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Von vorn starten' }));
    // ↓ and ↑ move between the buttons too, without wrapping.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Weiterschauen ab 0:04' }));
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Weiterschauen ab 0:04' }));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, choice.getByRole('button', { name: 'Von vorn starten' }));
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/play/${id}\\?at=0$`));
    // From the start, not from 0:04 (the player may begin at the first keyframe, a few ms in).
    await expect(page.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
      'data-start-ms',
      /^\d{1,3}$/,
    );
  } finally {
    await page.goto('/');
    await setPosition(page, id, 0);
  }
});

test('Tab: header, back, actions, the chosen season, episodes; no focus trap @keyboard', async ({
  page,
}) => {
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 3);
  // The pen and the watchlist button on the cover, play, then the round actions: "Merken",
  // "Gesehen", "Empfehlen" and "Verwalten" (the favourite is gone, concept 20 P-2; «Gesehen» took
  // its place).
  await tabTo(page, seriesPlay(page), 4);
  await tabTo(page, page.getByRole('button', { name: 'Gesehen', exact: true }), 4);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Empfehlen' }));
  // Only the chosen season is a Tab stop of the shelf (roving tabindex), then its pen and its
  // check (concept 20 P-5).
  await tabTo(page, seasonCards(page).first(), 4);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, seasonPens(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, seasonChecks(page).first());
  // Each episode: the row that plays, «Details» (concept 20 P-4), the check (P-5).
  await page.keyboard.press('Tab');
  await expectFocusOn(page, playLinks(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, detailsLinks(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, episodeChecks(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, playLinks(page).nth(1));
  await expectNoFocusTrap(page);
});

test('the check of an episode: Tab reaches it, Space sets it, Enter removes it without a question (concept 20 P-5) @keyboard', async ({
  page,
}) => {
  const id = (await episodeIds(page.request, series.split('/').pop() ?? ''))[1] ?? '';
  await setWatched(page, id, false);
  try {
    await page.reload();
    const check = episodeChecks(page).nth(1);
    await expect(check).toHaveAccessibleName(/^Gesehen: S01E02 · /);
    await expect(check).toHaveAttribute('aria-pressed', 'false');
    // Tab from the row's «Details» reaches its check, outside the play link.
    await detailsLinks(page).nth(1).focus();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, check);
    await expectVisibleFocus(page);
    await page.keyboard.press('Space');
    await expect(check).toHaveAttribute('aria-pressed', 'true');
    await expectFocusOn(page, check);
    // The server kept it: after a reload the check is still set.
    await page.reload();
    await expect(episodeChecks(page).nth(1)).toHaveAttribute('aria-pressed', 'true');
    await episodeChecks(page).nth(1).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('alertdialog')).toHaveCount(0);
    await expect(episodeChecks(page).nth(1)).toHaveAttribute('aria-pressed', 'false');
    await expectFocusOn(page, episodeChecks(page).nth(1));
  } finally {
    await setWatched(page, id, false);
  }
});

test('the check of a season: set without a question, removal asks, Escape gives the focus back (concept 20 P-5) @keyboard', async ({
  page,
}) => {
  // The specials: one episode, so the test leaves Season 1 alone.
  const specials = seasonCards(page).last();
  const specialsId = await page.evaluate(async (path) => {
    const response = await fetch(`/api/v1/items/${path.split('/').pop() ?? ''}`);
    const body = (await response.json()) as { seasons?: { id: string }[] };
    return body.seasons?.at(-1)?.id ?? '';
  }, series);
  await setWatched(page, specialsId, false);
  try {
    await page.reload();
    const check = seasonChecks(page).last();
    await expect(check).toHaveAttribute('tabindex', '-1');
    // Choose the specials: their card, pen and check become the shelf's Tab stops.
    await specials.click();
    await expect(specials).toHaveAttribute('aria-current', 'true');
    await expect(check).not.toHaveAttribute('tabindex');
    await specials.focus();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, seasonPens(page).last());
    await page.keyboard.press('Tab');
    await expectFocusOn(page, check);
    await expect(check).toHaveAttribute('aria-pressed', 'false');
    await page.keyboard.press('Enter');
    await expect(check).toHaveAttribute('aria-pressed', 'true');
    await expect(page.getByRole('alertdialog')).toHaveCount(0);
    // Its episode shows the check too, once the season is read again.
    await expect(episodeChecks(page).first()).toHaveAttribute('aria-pressed', 'true');

    // Removing asks; «Abbrechen» has the focus; Escape closes and gives the focus back.
    await check.focus();
    await page.keyboard.press('Space');
    const ask = page.getByRole('alertdialog');
    await expect(ask).toBeVisible();
    await expect(ask).toContainText('Bei 1 Folge fällt der Haken.');
    await expectFocusOn(page, ask.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Escape');
    await expect(ask).toHaveCount(0);
    await expectFocusOn(page, check);
    await expect(check).toHaveAttribute('aria-pressed', 'true');

    // Again, then Tab to «Alle Haken entfernen» and Enter: the check falls.
    await page.keyboard.press('Enter');
    await expect(ask).toBeVisible();
    await page.keyboard.press('Tab');
    await expectFocusOn(page, ask.getByRole('button', { name: 'Alle Haken entfernen' }));
    await page.keyboard.press('Enter');
    await expect(ask).toHaveCount(0);
    await expect(check).toHaveAttribute('aria-pressed', 'false');
    await expectFocusOn(page, check);
  } finally {
    await setWatched(page, specialsId, false);
  }
});

test('the pen on a season cover opens its editor on «Bilder»; Escape returns to the pen (concept 20 P-3) @keyboard', async ({
  page,
}) => {
  // A pen on every season; only the chosen season's pen is a Tab stop.
  await expect(seasonPens(page)).toHaveCount(2);
  await expect(seasonPens(page).last()).toHaveAttribute('tabindex', '-1');
  const pen = seasonPens(page).first();
  const season = ((await pen.getAttribute('aria-label')) ?? '').replace(
    /: Metadaten bearbeiten$/,
    '',
  );
  await tabTo(page, seasonCards(page).first(), 40);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, pen);

  await page.keyboard.press('Enter');
  const editor = page.getByRole('dialog', { name: season });
  await expect(editor).toBeVisible();
  await expect(
    editor
      .getByRole('tablist', { name: 'Bereiche des Editors' })
      .getByRole('tab', { selected: true }),
  ).toHaveText(/^Bilder/);
  await page.keyboard.press('Escape');
  await expect(editor).toHaveCount(0);
  await expectFocusOn(page, pen);
});

test('season page by keyboard: the series link, play, the episodes; no focus trap @keyboard', async ({
  page,
}) => {
  // The season page of Season 1 (its id from the series' detail).
  const seasonPath = await page.evaluate(async (path) => {
    const response = await fetch(`/api/v1/items/${path.split('/').pop() ?? ''}`);
    const body = (await response.json()) as { seasons?: { id: string }[] };
    return `/items/${body.seasons?.[0]?.id ?? ''}`;
  }, series);
  await page.goto(seasonPath);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(/Season 1|Staffel 1/);
  const seriesLink = page.getByRole('link', { name: SHOW_TITLE, exact: true });
  await expect(seriesLink).toHaveAttribute('href', series);
  await expect(playLinks(page)).toHaveCount(3);
  // No watchlist for a season (concept 17 E-15).
  await expect(page.getByRole('button', { name: /^(Merken|Gemerkt)$/ })).toHaveCount(0);

  // Arrows: the start focus on play, ↑ to the series link, ↓ back, ↓ into the episodes.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, seriesLink);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seriesPlay(page));
  for (let presses = 0; presses < 4; presses += 1) {
    if ((await focusedEpisode(page)) >= 0) break;
    await page.keyboard.press('ArrowDown');
  }
  await expect.poll(() => focusedEpisode(page)).toBeGreaterThanOrEqual(0);
  await expectVisibleFocus(page);

  // Tab from the top: the back arrow, the series link, play, then the episodes.
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 3);
  await tabTo(page, seriesLink, 3);
  await tabTo(page, seriesPlay(page), 2);
  await tabTo(page, playLinks(page).first(), 6);
  await expectNoFocusTrap(page);

  // Enter on the series link leads to the series.
  await seriesLink.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
});
