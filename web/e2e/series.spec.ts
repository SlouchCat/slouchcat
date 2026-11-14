import { expect, test, type Page } from '@playwright/test';
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
// Specials (S00E01). Concept 21 P-6: the seasons are a wall of covers, the episodes a compact
// list whose lines open a row with «Abspielen» and «Details».

let series = '';

const shelf = (page: Page) => page.getByRole('list', { name: 'Staffeln' });
// The season cards, without the pens (concept 20 P-3) and the checks (P-5) on their covers.
const seasonCards = (page: Page) =>
  shelf(page).getByRole('button', { name: /^(?!.*: Metadaten bearbeiten$)(?!Gesehen: )/ });
const seasonPens = (page: Page) =>
  shelf(page).getByRole('button', { name: /: Metadaten bearbeiten$/ });
const seasonChecks = (page: Page) => shelf(page).getByRole('button', { name: /^Gesehen: / });
const episodes = (page: Page) => page.getByRole('list', { name: 'Folgen' });
// Each episode is one line, a button that opens its row: "S01E01 Pilot … 1 min".
const lines = (page: Page) => episodes(page).getByRole('button', { name: /^S\d\dE\d\d/ });
const line = (page: Page, code: string) =>
  episodes(page).getByRole('button', { name: new RegExp(`^${code} `) });
// The open row's buttons: play ("Abspielen: S01E01 · …" or "Weiterschauen: …") and «Details».
const playIn = (page: Page, code: string) =>
  episodes(page).getByRole('link', {
    name: new RegExp(`^(Abspielen|Weiterschauen): ${code} · `),
  });
const detailsIn = (page: Page, code: string) =>
  episodes(page).getByRole('link', { name: new RegExp(`^Details: ${code} · `) });
const detailsLinks = (page: Page) => episodes(page).getByRole('link', { name: /^Details: / });
// …and at the far right of each line the check mark «gesehen» (concept 20 P-5).
const episodeChecks = (page: Page) => episodes(page).getByRole('button', { name: /^Gesehen: / });
// The big button of a series names its nextUp episode (concept 17 3.3); other specs may have left
// progress behind, so any of the four reasons.
// The open row of the episode list plays too ("Abspielen: S01E01 · …"): not that one.
const seriesPlay = (page: Page) =>
  page
    .getByRole('link', {
      name: /^(Abspielen|Weiterschauen|Nächste Folge|Nochmals ansehen) (S\d\dE\d\d|Alles gesehen)/,
    })
    .and(page.locator('xpath=//*[not(ancestor::ul[@aria-label="Folgen"])]'));
const episodePlay = (page: Page) =>
  page.getByRole('link', { name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/ });

/** Index of the focused line in the episode list, or -1. */
const focusedEpisode = (page: Page) =>
  lines(page).evaluateAll((buttons) =>
    buttons.indexOf(document.activeElement as HTMLButtonElement),
  );

/** Closes every open row by pointer (a click on an open line closes it). */
async function closeRows(page: Page): Promise<void> {
  await expect(lines(page).first()).toBeVisible();
  const open = episodes(page).locator('button[aria-expanded="true"]');
  while ((await open.count()) > 0) await open.first().click();
}

/** Opens the row of `code` by pointer, if it is not open yet. */
async function openRow(page: Page, code: string): Promise<void> {
  const target = line(page, code);
  await expect(target).toBeVisible();
  if ((await target.getAttribute('aria-expanded')) !== 'true') await target.click();
  await expect(target).toHaveAttribute('aria-expanded', 'true');
}

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

/** Puts every episode of Season 1 back to not started and not watched. */
async function resetSeason(page: Page): Promise<void> {
  for (const id of await episodeIds(page.request, series.split('/').pop() ?? '')) {
    await setPosition(page, id, 0);
    await setWatched(page, id, false);
  }
}

test.beforeEach(async ({ page }) => {
  series ||= await itemPath(page, SHOW_TITLE);
  await page.goto('/');
  await page.goto(series);
  await expect(page.getByRole('heading', { level: 1, name: SHOW_TITLE })).toBeVisible();
  await expect(lines(page).first()).toBeVisible();
  await resetFocus(page);
});

test('the start page lists the series once and never its episodes', async ({ page }) => {
  await page.goto('/');
  const recent = page.getByRole('region', { name: 'Neu hinzugefügt' });
  await expect(recent.getByRole('link').filter({ hasText: SHOW_TITLE })).toHaveCount(1);
  await expect(recent.getByRole('link', { name: /S\d\dE\d\d|^Episode \d/ })).toHaveCount(0);
});

test('seasons: a wall with Season 1 first, specials last; episodes as lines; S01E03–E04 is one episode (concept 21 P-6)', async ({
  page,
}) => {
  await expect(seasonCards(page)).toHaveCount(2);
  await expect(seasonCards(page).first()).toHaveAttribute('aria-current', 'true');
  await expect(seasonCards(page).first()).toContainText('3 Folgen');
  await expect(seasonCards(page).last()).toContainText(/Specials|Season 0|Staffel 0/);
  // Two seasons fit into two rows: no «Alle … Staffeln anzeigen».
  const seasonsRegion = page.getByRole('region', { name: 'Staffeln' });
  await expect(seasonsRegion.getByRole('button', { name: /Staffeln anzeigen$/ })).toHaveCount(0);
  // The wall and the list take the whole width between the gutters (no 64rem box any more).
  const widths = await page.evaluate(() => {
    const list = document.querySelector('ul[aria-label="Folgen"]')!.getBoundingClientRect();
    const wall = document.querySelector('ul[aria-label="Staffeln"]')!.getBoundingClientRect();
    return { list: list.width, wall: wall.width, page: document.documentElement.clientWidth };
  });
  expect(widths.list).toBeGreaterThan(widths.page * 0.6);
  expect(Math.abs(widths.list - widths.wall)).toBeLessThan(2);
  // One line per episode; no «Details» but in an open row.
  await expect(lines(page)).toHaveCount(3);
  await expect(line(page, 'S01E01')).toBeVisible();
  await expect(line(page, 'S01E03–E04')).toContainText('3–4');
  await expect(detailsLinks(page)).toHaveCount(
    await episodes(page).locator('button[aria-expanded="true"]').count(),
  );
  // The big button plays the nextUp episode of the series (E-10).
  await expect(seriesPlay(page)).toHaveAttribute('href', /^\/play\//);
  await expect(page).toHaveTitle(`${SHOW_TITLE} · Slouchcat`);
});

test('the nextUp episode is open as the page opens, marked «Als Nächstes» (concept 21 P-6)', async ({
  page,
}) => {
  await resetSeason(page);
  try {
    await page.reload();
    const pilot = line(page, 'S01E01');
    await expect(pilot).toHaveAttribute('aria-expanded', 'true');
    await expect(pilot).toContainText('Als Nächstes');
    await expect(playIn(page, 'S01E01')).toHaveAccessibleName(/^Abspielen: S01E01 · /);
    await expect(detailsIn(page, 'S01E01')).toBeVisible();
    await expect(line(page, 'S01E02')).toHaveAttribute('aria-expanded', 'false');
    // One row open at a time: a click on another line moves it there.
    await line(page, 'S01E02').click();
    await expect(line(page, 'S01E02')).toHaveAttribute('aria-expanded', 'true');
    await expect(pilot).toHaveAttribute('aria-expanded', 'false');
    await expect(detailsLinks(page)).toHaveCount(1);
  } finally {
    await resetSeason(page);
  }
});

test('O-11: ↓ from the wall into the episodes, Enter opens a line, a second Enter plays (concept 21) @keyboard', async ({
  page,
}) => {
  await resetSeason(page);
  try {
    await page.reload();
    await closeRows(page);
    // From the chosen season of the wall (the way there from the big button: the test above).
    // (A focus by script after the clicks shows no ring yet; the keys below bring it back.)
    await seasonCards(page).first().focus();
    await expect(seasonCards(page).first()).toBeFocused();
    // ↓ from the wall lands on the line of the next episode, S01E01 after the reset.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, line(page, 'S01E01'));
    // Along the lines with ↓ and ↑ while every row is closed.
    const start = await focusedEpisode(page);
    await page.keyboard.press(start < 2 ? 'ArrowDown' : 'ArrowUp');
    await expect.poll(() => focusedEpisode(page)).toBe(start < 2 ? start + 1 : start - 1);
    await expectVisibleFocus(page);
    // Enter opens the line of S01E01 and moves the focus to «Abspielen».
    await line(page, 'S01E01').focus();
    await page.keyboard.press('Enter');
    await expect(line(page, 'S01E01')).toHaveAttribute('aria-expanded', 'true');
    await expectFocusOn(page, playIn(page, 'S01E01'));
    // Escape (a remote's back key) closes the row and stays on the page; Enter opens it again.
    await page.keyboard.press('Escape');
    await expect(line(page, 'S01E01')).toHaveAttribute('aria-expanded', 'false');
    await expectFocusOn(page, line(page, 'S01E01'));
    await expect(page).toHaveURL(new RegExp(`${series}$`));
    await page.keyboard.press('Enter');
    await expectFocusOn(page, playIn(page, 'S01E01'));
    // ↑ back to the line, ↓ to the row's play button again.
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, line(page, 'S01E01'));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, playIn(page, 'S01E01'));
    // The second Enter plays (not started: at once).
    const id = await firstEpisode(page);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/play/${id}$`));
    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL(new RegExp(`${series}$`));
  } finally {
    await page.goto('/');
    await resetSeason(page);
  }
});

test('arrows: Enter on a season of the wall shows the specials, ← back to Season 1 @keyboard', async ({
  page,
}) => {
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
  await expect(line(page, 'S00E01')).toBeVisible();
  await expect(lines(page)).toHaveCount(1);
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('Enter');
  await expect(lines(page)).toHaveCount(3);
});

test('a wall of many seasons: two rows, «Alle N Staffeln anzeigen», ↓ over the button into the episodes; the pen with the focus (concept 21 P-6, O-11) @keyboard', async ({
  page,
}) => {
  // The test media have two seasons: add twelve copies of the specials to the series' answer.
  await page.route(/\/api\/v1\/items\/[^/?]+(\?.*)?$/, async (route) => {
    const response = await route.fetch();
    const body = (await response.json()) as { kind?: string; seasons?: Record<string, unknown>[] };
    if (body.kind !== 'series' || !body.seasons) return route.fulfill({ response });
    const specials = body.seasons.at(-1)!;
    const extra = Array.from({ length: 12 }, (_, index) => ({
      ...specials,
      id: `01K21ZZZZ${String(index).padStart(17, '0')}`,
      title: `Staffel ${index + 2}`,
      seasonNumber: index + 2,
    }));
    await route.fulfill({ response, json: { ...body, seasons: [...body.seasons, ...extra] } });
  });
  try {
    await page.reload();
    await closeRows(page);
    const toggle = page.getByRole('button', { name: 'Alle 14 Staffeln anzeigen' });
    await expect(toggle).toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    // Folded: exactly two rows of the grid's columns.
    const columns = await shelf(page).evaluate(
      (list) => getComputedStyle(list).gridTemplateColumns.trim().split(/\s+/).length,
    );
    expect(columns).toBeGreaterThan(1);
    await expect(seasonCards(page)).toHaveCount(Math.min(14, columns * 2));

    // The pen of the chosen season shows only with the pointer or the focus.
    const penBox = seasonPens(page).first().locator('xpath=..');
    await page.mouse.move(0, 0);
    await expect.poll(() => penBox.evaluate((box) => getComputedStyle(box).opacity)).toBe('0');
    await seasonCards(page).first().focus();
    await expect.poll(() => penBox.evaluate((box) => getComputedStyle(box).opacity)).toBe('1');

    // ↓ from the wall's last row reaches the button, a further ↓ the episode list.
    await seasonCards(page).nth(columns).focus();
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, toggle);
    await page.keyboard.press('ArrowDown');
    await expect.poll(() => focusedEpisode(page)).toBeGreaterThanOrEqual(0);
    // ↑ walks back up the lines to the button, not past it into the wall.
    for (let presses = 0; presses < 6; presses += 1) {
      if (await toggle.evaluate((button) => button === document.activeElement)) break;
      expect(await focusedEpisode(page)).toBeGreaterThanOrEqual(0);
      await page.keyboard.press('ArrowUp');
    }
    await expectFocusOn(page, toggle);

    // Enter unfolds: every season, the button says «Weniger anzeigen».
    await toggle.focus();
    await page.keyboard.press('Enter');
    await expect(seasonCards(page)).toHaveCount(14);
    const fold = page.getByRole('button', { name: 'Weniger anzeigen' });
    await expectFocusOn(page, fold);
    await expect(fold).toHaveAttribute('aria-expanded', 'true');
    // Choose the last season, then fold: it stays in the wall, in place of the last cover.
    const last = seasonCards(page).filter({ hasText: 'Staffel 13' });
    await last.click();
    await expect(last).toHaveAttribute('aria-current', 'true');
    await fold.click();
    await expect(seasonCards(page)).toHaveCount(Math.min(14, columns * 2));
    await expect(last).toBeVisible();
    await expect(last).toHaveAttribute('aria-current', 'true');
    // By keyboard: Enter on the button unfolds and folds again, the focus stays on it.
    await toggle.focus();
    await page.keyboard.press('Enter');
    await expectFocusOn(page, fold);
    await page.keyboard.press('Enter');
    await expectFocusOn(page, toggle);
    await expectNoFocusTrap(page);
  } finally {
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  }
});

test('Details in the open row opens the episode page; Backspace and Escape walk back (concept 21 P-6) @keyboard', async ({
  page,
}) => {
  const id = await firstEpisode(page);
  await openRow(page, 'S01E01');
  const details = detailsIn(page, 'S01E01');
  // ← → move between the row's buttons; Tab from play reaches «Details» too.
  await playIn(page, 'S01E01').focus();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, details);
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, playIn(page, 'S01E01'));
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
  await expect(page.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
    'data-start-ms',
    /\d+/,
  );
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
    await openRow(page, 'S01E01');
    const row = playIn(page, 'S01E01');
    await expect(row).toHaveAccessibleName(/^Weiterschauen: S01E01 · /);
    await row.focus();
    await page.keyboard.press('Enter');
    const choice = page.getByRole('dialog', { name: /^S01E01 · / });
    await expect(choice).toBeVisible();
    await expect(choice.getByText('Du hast diese Folge angefangen.')).toBeVisible();
    await expectFocusOn(page, choice.getByRole('button', { name: 'Weiterschauen ab 0:04' }));
    await expect(page).toHaveURL(new RegExp(`${series}$`));

    // Escape: nothing plays, the focus is back on the play button.
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

test('Tab: header, back, actions, the chosen season, the lines and their checks; no focus trap @keyboard', async ({
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
  // Only the chosen season is a Tab stop of the wall (roving tabindex), then its pen and its
  // check (concept 20 P-5).
  await tabTo(page, seasonCards(page).first(), 4);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, seasonPens(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, seasonChecks(page).first());
  // Each episode: the line, then its check (concept 20 P-5); an open row adds its buttons.
  await page.keyboard.press('Tab');
  await expectFocusOn(page, lines(page).first());
  await page.keyboard.press('Tab');
  await expectFocusOn(page, episodeChecks(page).first());
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
    // Tab from the line reaches its check, outside the line's button.
    await line(page, 'S01E02').focus();
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
  await expect(lines(page)).toHaveCount(3);
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
  await tabTo(page, lines(page).first(), 6);
  await expectNoFocusTrap(page);

  // Enter opens a line and moves to its play button, the second Enter plays (concept 21 O-11).
  const id = await firstEpisode(page);
  await setPosition(page, id, 0);
  await page.reload();
  await expect(lines(page).first()).toBeVisible();
  await closeRows(page);
  await line(page, 'S01E01').focus();
  await page.keyboard.press('Enter');
  await expectFocusOn(page, playIn(page, 'S01E01'));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/play/${id}$`));
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`${seasonPath}$`));

  // Enter on the series link leads to the series.
  await seriesLink.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${series}$`));
});
