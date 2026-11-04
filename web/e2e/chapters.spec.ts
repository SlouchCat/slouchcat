import type { Page } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus, tabTo } from './keyboard';
import {
  arrowTo,
  CHAPTERS_TITLE,
  episodeIds,
  INTRO_SHOW_TITLE,
  itemIdByTitle,
  play,
  player,
  resetProgress,
  startOf,
  videoState,
} from './playback';

// W-9: a chapter jump lands within ±1 s of the chapter start; "Intro überspringen" shows during
// the detected intro and jumps to its end; from the outro (or the end) a 10-s countdown plays the
// next episode, unless a key cancels it.

const time = async (page: Page) => (await videoState(page)).time;

test('the chapter tab by keyboard jumps to the chapter start (±1 s); Escape closes it', async ({
  page,
  owner,
}) => {
  const itemId = await itemIdByTitle(owner, CHAPTERS_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const info = await play(page, itemId);
  expect(info.chapters.map((chapter) => chapter.title)).toEqual([
    'Kapitel 1: Anfang',
    'Kapitel 2: Mitte',
    'Kapitel 3: Ende',
  ]);
  // Paused, so the position stays where the jump put it.
  await page.keyboard.press('Space');
  await expect.poll(async () => (await videoState(page)).paused).toBe(true);

  // C opens the tab «Kapitel» (11.2) with the focus on the chapter playing.
  await page.keyboard.press('c');
  const cards = page.getByRole('list', { name: 'Kapitel' });
  await expect(cards.getByRole('button')).toHaveCount(3);
  const first = cards.getByRole('button', { name: /^Kapitel 1: Anfang/ });
  await expect(first).toHaveAttribute('aria-current', 'true');
  await expectFocusOn(page, first);
  // → walks the cards (the position stays), Enter jumps.
  await arrowTo(page, cards.getByRole('button', { name: /^Kapitel 3: Ende/ }), 'ArrowRight');
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const third = info.chapters[2].startMs / 1000;
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(third - 1);
  expect(await time(page)).toBeLessThanOrEqual(third + 1);
  await expect(cards.getByRole('button', { name: /^Kapitel 3: Ende/ })).toHaveAttribute(
    'aria-current',
    'true',
  );

  // ← back to the second chapter, Enter.
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards.getByRole('button', { name: /^Kapitel 2: Mitte/ }));
  await page.keyboard.press('Enter');
  const second = info.chapters[1].startMs / 1000;
  await expect.poll(() => time(page)).toBeLessThanOrEqual(second + 1);
  expect(await time(page)).toBeGreaterThanOrEqual(second - 1);

  // ↑ goes to the tab strip, ↑ again to the button row.
  const tab = page.getByRole('toolbar', { name: 'Mehr zum Titel' }).getByRole('button', {
    name: 'Kapitel',
  });
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, tab);
  await expect(tab).toHaveAttribute('aria-expanded', 'true');
  await page.keyboard.press('ArrowUp');
  await expect(page.getByRole('toolbar', { name: 'Wiedergabe' }).locator(':focus')).toHaveCount(1);

  // Escape closes the tab only, the focus on its tab; the player stays.
  await page.keyboard.press('c');
  await expect(cards).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(cards).toHaveCount(0);
  await expectFocusOn(page, tab);
  await expect(page).toHaveURL(/\/play\//);
});

test.describe('intro show', () => {
  let episodes: string[] = [];

  test.beforeEach(async ({ owner }) => {
    if (episodes.length === 0) {
      const seriesId = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
      episodes = await episodeIds(owner, seriesId);
      expect(episodes.length).toBeGreaterThanOrEqual(2);
    }
  });

  test('"Intro überspringen" appears during the intro and jumps to its end', async ({ page }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    const info = await play(page, episodes[0]);
    const intro = info.segments.intro;
    expect(intro, 'the detected intro of S01E01').toBeDefined();

    const skip = page.getByRole('button', { name: 'Intro überspringen' });
    // The intro starts a few seconds in; jump close to it instead of waiting.
    await page.locator('video').evaluate(
      (video: HTMLVideoElement, at: number) => {
        video.currentTime = at;
      },
      intro!.startMs / 1000 + 1,
    );
    await expect(skip).toBeVisible({ timeout: 10_000 });
    await page.keyboard.press('s');
    const end = intro!.endMs / 1000;
    await expect.poll(() => time(page)).toBeGreaterThanOrEqual(end - 1);
    expect(await time(page)).toBeLessThanOrEqual(end + 2);
    await expect(skip).toHaveCount(0);
  });

  test('the countdown from the outro plays S01E02 (auto-next)', async ({ page }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    await resetProgress(page.request, episodes[1]);
    const info = await play(page, episodes[0]);
    expect(info.nextEpisode?.id).toBe(episodes[1]);
    // Binge in full screen: the next episode stays in full screen (W-2).
    const inFullscreen = () => page.evaluate(() => document.fullscreenElement !== null);
    await page.keyboard.press('f');
    await expect.poll(inFullscreen).toBe(true);
    const at = info.segments.outro ? info.segments.outro.startMs / 1000 + 0.5 : undefined;
    await page.locator('video').evaluate((video: HTMLVideoElement, seconds: number | undefined) => {
      video.currentTime = seconds ?? Math.max(0, video.duration - 2);
    }, at);

    const countdown = page.getByRole('region', { name: 'Nächste Folge' });
    await expect(countdown).toBeVisible({ timeout: 10_000 });
    await expect(player(page)).toHaveAttribute('data-autonext', 'countdown');
    await expect(countdown.getByText(/^Startet in \d+ Sekunden?$/)).toBeVisible();
    await expect(countdown.getByText(/^S01E02 · /)).toBeVisible();

    const next = await startOf(page, () =>
      page.waitForURL(`**/play/${episodes[1]}`, { timeout: 20_000 }),
    );
    expect(next.status).toBe(200);
    expect(next.body.item.id).toBe(episodes[1]);
    await expect(player(page)).toHaveAttribute('data-autonext', 'navigated');
    await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
    await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
    expect(await inFullscreen()).toBe(true);
    await expect(page.getByRole('button', { name: 'Vollbild beenden' })).toBeAttached();
  });

  test('Escape cancels the countdown; N plays the next episode at once', async ({ page }) => {
    test.setTimeout(60_000);
    // The page's timers run on Playwright's clock, so the ten seconds can be skipped below.
    await page.clock.install();
    await resetProgress(page.request, episodes[0]);
    const info = await play(page, episodes[0]);
    await page.locator('video').evaluate((video: HTMLVideoElement) => {
      video.currentTime = Math.max(0, video.duration - 2);
    });
    const countdown = page.getByRole('region', { name: 'Nächste Folge' });
    await expect(countdown).toBeVisible({ timeout: 10_000 });
    await expect(countdown.getByText(/^Startet in \d+ Sekunden?$/)).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(countdown).toHaveCount(0);
    await expect(player(page)).not.toHaveAttribute('data-autonext', /.+/);
    // Cancelled: no navigation after the ten seconds (skipped on the clock; a running countdown
    // would navigate within its next 250-ms tick), and Escape did not leave the player.
    await page.clock.runFor(11_000);
    await expect(player(page)).not.toHaveAttribute('data-autonext', /.+/);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[0]}$`));

    const next = await startOf(page, () => page.keyboard.press('n'));
    expect(next.body.item.id).toBe(info.nextEpisode?.id);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[1]}$`));
  });

  test('the next-episode card by keyboard: Tab reaches its button, Enter plays', async ({
    page,
  }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[0]);
    await resetProgress(page.request, episodes[1]);
    await play(page, episodes[0]);
    await page.locator('video').evaluate((video: HTMLVideoElement) => {
      video.currentTime = Math.max(0, video.duration - 2);
    });
    const card = page.getByRole('region', { name: 'Nächste Folge' });
    await expect(card).toBeVisible({ timeout: 10_000 });
    await expect(card.getByText(/^S01E02 · /)).toBeVisible();
    // Tab moves on without cancelling; the card's buttons come after the controls.
    const playNow = card.getByRole('button', { name: 'Jetzt ansehen' });
    await tabTo(page, playNow);
    await expectVisibleFocus(page);
    await expect(player(page)).toHaveAttribute('data-autonext', 'countdown');
    const next = await startOf(page, () => page.keyboard.press('Enter'));
    expect(next.body.item.id).toBe(episodes[1]);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[1]}$`));
  });

  test('«Als Nächstes» by keyboard: ↓ to the tabs, → to the tab, ↓ to the next episode, Enter', async ({
    page,
  }) => {
    test.setTimeout(60_000);
    await resetProgress(page.request, episodes[1]);
    await play(page, episodes[0]);
    await page.keyboard.press('Space');
    // From play/pause ↓ goes to the tab strip, → along it.
    await page.keyboard.press('ArrowDown');
    const strip = page.getByRole('toolbar', { name: 'Mehr zum Titel' });
    await expect(strip.locator(':focus')).toHaveCount(1);
    const tab = strip.getByRole('button', { name: 'Als Nächstes' });
    await arrowTo(page, tab, 'ArrowRight');
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    const cards = page.getByRole('list', { name: 'Als Nächstes' });
    await expect(cards.getByRole('button', { name: /^S01E01/ })).toHaveAttribute(
      'aria-current',
      'true',
    );
    // ↓ into the row, on the episode after this one.
    await page.keyboard.press('ArrowDown');
    const second = cards.getByRole('button', { name: /^S01E02/ });
    await expectFocusOn(page, second);
    // ←/→ stay in the row; ↑ goes back to the strip; no trap.
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, cards.getByRole('button', { name: /^S01E01/ }));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, second);
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, tab);
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, second);
    const next = await startOf(page, () => page.keyboard.press('Enter'));
    expect(next.body.item.id).toBe(episodes[1]);
    await expect(page).toHaveURL(new RegExp(`/play/${episodes[1]}$`));
    // A chosen episode is no auto-next.
    await expect(player(page)).not.toHaveAttribute('data-autonext', /.+/);
  });
});
