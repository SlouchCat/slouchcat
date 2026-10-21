import { constants, copyFileSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { type Page, type TestInfo } from '@playwright/test';
import {
  episodeTitle,
  feedEpisodes,
  feedKey,
  fetchNow,
  subscribeFeed,
  waitForEpisodes,
} from './audio';
import { introSeason, setFavorite, unreadNotifications } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, resetFocus } from './keyboard';
import { INTRO_SHOW_TITLE, itemIdByTitle } from './playback';

// E-8: a profile marks a series as favourite; a new episode in the library notifies it in the app.
// The episode is a copy of an existing file under a new episode number, written into the series
// folder of testdata/media/TV (the library watcher picks it up) and removed after the test.

/** Time the badge may take (E-8 measurement in docs/perf/M7.md: copy to badge). */
const BADGE_TIMEOUT_MS = 120_000;

const badge = (page: Page) =>
  page.getByRole('link', { name: /^Benachrichtigungen, \d+ ungelesen$/ });
const inbox = (page: Page) => page.getByRole('list', { name: 'Benachrichtigungen' });

/**
 * Source and target episode per browser and retry. The library takes a file whose content matches a
 * missing one for that file moved (B-9): the copy of an earlier run, deleted after it, would come
 * back as the old episode. So each run copies another episode (S01E01 … S01E05) to a new number.
 */
function episodes(testInfo: TestInfo): { source: string; target: string } {
  const project = Math.max(0, ['chromium', 'firefox', 'webkit'].indexOf(testInfo.project.name));
  const pad = (n: number) => String(n).padStart(2, '0');
  return {
    source: pad(Math.min(5, 1 + project * 2 + testInfo.retry)),
    target: pad(10 + project * 5 + testInfo.retry),
  };
}

test('a new episode of a favourite series shows in the badge and the inbox (E-8)', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(BADGE_TIMEOUT_MS + 60_000);
  const series = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
  await setFavorite(owner, series, true);
  await page.goto('/');
  await expect(page.getByRole('link', { name: 'Benachrichtigungen', exact: true })).toBeVisible();

  const season = introSeason();
  const { source: from, target: number } = episodes(testInfo);
  const source = readdirSync(season).find((name) => name.match(/S01E(\d\d)\.\w+$/)?.[1] === from);
  if (!source) throw new Error(`no S01E${from} in ${season}`);
  const target = join(season, source.replace(`S01E${from}`, `S01E${number}`));
  const started = Date.now();
  // A file at the target is a leftover the global setup did not remove (another run on the same
  // test media): fail here, not 120 s later at the badge.
  copyFileSync(join(season, source), target, constants.COPYFILE_EXCL);
  try {
    // The page asks every 30 s; no reload.
    await expect(badge(page)).toHaveAccessibleName('Benachrichtigungen, 1 ungelesen', {
      timeout: BADGE_TIMEOUT_MS,
    });
    const seconds = Math.round((Date.now() - started) / 1000);
    testInfo.annotations.push({ type: 'E-8 copy to badge', description: `${seconds} s` });

    await badge(page).click();
    await expect(page).toHaveURL(/\/notifications$/);
    const entry = inbox(page).getByRole('link', { name: new RegExp(`^Ungelesen: S01E${number}`) });
    await expect(entry).toContainText(`Neue Folge von ${INTRO_SHOW_TITLE}`);
    // Opening it marks it read and shows the episode.
    await entry.click();
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page).toHaveURL(/\/items\//);
    await expect.poll(() => unreadNotifications(owner)).toBe(0);
    await expect(page.getByRole('link', { name: 'Benachrichtigungen', exact: true })).toBeVisible();
  } finally {
    rmSync(target, { force: true });
  }
});

test('inbox by keyboard: arrows, Enter, "Alle als gelesen markieren"; no focus trap', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(120_000);
  // A favourite podcast with a new episode: the feed of its own gets episode 3 (H-2).
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  await waitForEpisodes(owner, podcast.id, 2);
  await setFavorite(owner, podcast.id, true);
  await feedEpisodes(key, 3);
  await fetchNow(owner, podcast.id);
  await expect.poll(() => unreadNotifications(owner), { timeout: 60_000 }).toBe(1);

  await page.goto('/notifications');
  await expect(page.getByRole('heading', { level: 1, name: 'Benachrichtigungen' })).toBeVisible();
  await expect(page.getByText('1 ungelesen')).toBeVisible();
  const entries = inbox(page).getByRole('link');
  const entry = inbox(page).getByRole('link', {
    // A string matches as a substring; the title holds parentheses.
    name: `Ungelesen: ${episodeTitle(3)}`,
  });
  await expect(entry).toBeVisible();

  // The first arrow lands on the newest entry; up reaches the buttons above.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entries.first());
  await page.keyboard.press('ArrowUp');
  await expectVisibleFocus(page);
  // The buttons are one row: "Zurück", then "Alle als gelesen markieren".
  const readAll = page.getByRole('button', { name: 'Alle als gelesen markieren' });
  for (let presses = 0; presses < 3; presses += 1) {
    if (await readAll.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, readAll);
  await page.keyboard.press('Enter');
  await expect(readAll).toHaveCount(0);
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await expect(page.getByText('0 ungelesen')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Benachrichtigungen', exact: true })).toBeVisible();

  // Enter on an entry opens the episode; the browser's back returns to the inbox.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, entries.first());
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/items\//);
  await page.goBack();
  await expect(page).toHaveURL(/\/notifications$/);
  await expectNoFocusTrap(page);

  // Escape goes back; opened directly, that is the start page.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL((url) => url.pathname === '/');
});
