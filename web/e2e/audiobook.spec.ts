import type { Page } from '@playwright/test';
import {
  AUDIO_AUTHOR,
  AUDIO_NARRATOR,
  AUDIO_SERIES,
  audioDetail,
  audioItemId,
  audioPlayer,
  audioState,
  M4B_TITLE,
  MP3_TITLE,
  playAudio,
  setAudioProgress,
} from './audio';
import { STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// H-1: the M4B book has its 12 chapter marks ("Kapitel 1" … "Kapitel 12", every 5 s, ±1 s), the
// MP3 folder is one book with a chapter per file; author, narrator and cover show on the detail
// page; a chapter starts the player there. H-3: the speed is kept per profile and title: after a
// reload and on a second device the player starts at 2.0, at the position ±2 s. The start page
// lists the book under "Neu hinzugefügt" with a square cover.

const time = async (page: Page) => (await audioState(page)).time;

let m4b = '';
let mp3 = '';

test.beforeEach(async ({ owner }) => {
  m4b ||= await audioItemId(owner, M4B_TITLE);
  mp3 ||= await audioItemId(owner, MP3_TITLE);
});

test('the M4B book: 12 chapters every 5 s, author, narrator, series and cover (H-1)', async ({
  page,
  owner,
}) => {
  const detail = await audioDetail(owner, m4b, 12);
  expect(detail.chapters!.map((chapter) => chapter.title)).toEqual(
    Array.from({ length: 12 }, (_, index) => `Kapitel ${index + 1}`),
  );
  detail.chapters!.forEach((chapter, index) => {
    expect(Math.abs(chapter.startMs - index * 5000), `start of chapter ${index + 1}`).toBeLessThan(
      1000,
    );
  });
  expect(detail).toMatchObject({ author: AUDIO_AUTHOR, narrator: AUDIO_NARRATOR });

  await page.goto(`/items/${m4b}`);
  await expect(page.getByRole('heading', { level: 1, name: M4B_TITLE })).toBeVisible();
  await expect(page.getByText(`von ${AUDIO_AUTHOR}`)).toBeVisible();
  await expect(page.getByText(`Gelesen von ${AUDIO_NARRATOR}`)).toBeVisible();
  await expect(page.getByText(new RegExp(`^${AUDIO_SERIES}`))).toBeVisible();
  // The cover (embedded picture of the M4B) loads and is square.
  const cover = page.locator(`img[src*="/items/${m4b}/images/poster"][height="400"]`);
  await expect(cover).toBeVisible();
  await expect
    .poll(() => cover.evaluate((image: HTMLImageElement) => image.naturalWidth))
    .toBeGreaterThan(0);
  const chapters = page.getByRole('list', { name: 'Kapitel' });
  await expect(chapters.getByRole('link')).toHaveCount(12);
  await expect(chapters.getByRole('link', { name: 'Kapitel 1 ab 0:00 abspielen' })).toBeVisible();
  await expect(chapters.getByRole('link', { name: 'Kapitel 12 ab 0:55 abspielen' })).toBeVisible();
});

test('the MP3 folder is one book with a chapter per file (H-1)', async ({ page, owner }) => {
  const detail = await audioDetail(owner, mp3, 10);
  expect(detail.chapters!.map((chapter) => chapter.title)).toEqual(
    Array.from({ length: 10 }, (_, index) => `Kapitel ${index + 1}`),
  );
  detail.chapters!.forEach((chapter, index) => {
    expect(Math.abs(chapter.startMs - index * 6000), `start of part ${index + 1}`).toBeLessThan(
      1000,
    );
  });
  expect(detail).toMatchObject({ author: AUDIO_AUTHOR, narrator: AUDIO_NARRATOR });
  await page.goto(`/items/${mp3}`);
  await expect(page.getByRole('heading', { level: 1, name: MP3_TITLE })).toBeVisible();
  await expect(page.getByRole('list', { name: 'Kapitel' }).getByRole('link')).toHaveCount(10);
  await expect(page.locator(`img[src*="/items/${mp3}/images/poster"][height="400"]`)).toBeVisible();
});

test('a chapter link on the detail page starts the player at that chapter (±1 s)', async ({
  page,
  owner,
}) => {
  await audioDetail(owner, m4b, 12);
  await setAudioProgress(page.request, m4b, 0, 60_000);
  await page.goto(`/items/${m4b}`);
  const link = page.getByRole('link', { name: 'Kapitel 3 ab 0:10 abspielen' });
  await link.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`/play/${m4b}`));
  await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
  const started = await time(page);
  expect(Math.abs(started - 10), `started at ${started} s`).toBeLessThanOrEqual(1);
  await expect(audioPlayer(page).getByText(/^Kapitel 3 · /)).toBeVisible();
  // The start position leaves the URL: a reload resumes from the saved position.
  await expect(page).toHaveURL(new RegExp(`/play/${m4b}$`));
});

test('the book’s detail page by keyboard: arrows into the chapters, Tab, Enter, back', async ({
  page,
  owner,
}) => {
  await audioDetail(owner, m4b, 12);
  await setAudioProgress(page.request, m4b, 0, 60_000);
  // From the start page, so that back leads there (detail.spec).
  await page.goto('/');
  await page.goto(`/items/${m4b}`);
  await expect(page.getByRole('heading', { level: 1, name: M4B_TITLE })).toBeVisible();
  await resetFocus(page);
  const play = page.getByRole('link', { name: /^(Abspielen|Weiterhören ab \d{2}:\d{2}:\d{2})$/ });
  const chapters = page.getByRole('list', { name: 'Kapitel' });
  const chapter = (n: number) =>
    chapters.getByRole('link', { name: new RegExp(`^Kapitel ${n} ab 0:\\d\\d abspielen$`) });

  // ↓ lands on play, → reaches back, ← returns; ↓ goes down into the chapter list.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, play);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, chapter(1));
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, chapter(3));
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, chapter(2));

  // Tab from the top: the header, then play and back; the chapters follow; no focus trap. A fresh
  // load starts the Tab sequence at the top in every browser (Firefox continues from the last
  // focused element after a blur).
  await page.goto(`/items/${m4b}`);
  await expect(page.getByRole('heading', { level: 1, name: M4B_TITLE })).toBeVisible();
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
  await tabTo(page, play, 2);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }), 2);
  await tabTo(page, chapter(1), 6);
  await expectNoFocusTrap(page);

  // Enter on a chapter starts the player there; Backspace returns to the book, Escape home.
  await chapter(2).focus();
  await page.keyboard.press('Enter');
  await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
  await expect(audioPlayer(page).getByText(/^Kapitel 2 · /)).toBeVisible();
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`/items/${m4b}$`));
  await expect(page.getByRole('heading', { level: 1, name: M4B_TITLE })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(/\/$/);
  await setAudioProgress(page.request, m4b, 0, 60_000);
});

test('the chapter menu (C) and Shift+→ jump to chapter starts in the player (±1 s)', async ({
  page,
  owner,
}) => {
  await audioDetail(owner, m4b, 12);
  await setAudioProgress(page.request, m4b, 0, 60_000);
  const info = await playAudio(page, m4b);
  expect(info.chapters).toHaveLength(12);
  const player = audioPlayer(page);
  // Paused, so the position stays where the jump put it.
  await expectFocusOn(page, player.getByRole('button', { name: 'Pause' }));
  await page.keyboard.press('k');
  await expect.poll(async () => (await audioState(page)).paused).toBe(true);

  await page.keyboard.press('c');
  const menu = page.getByRole('menu', { name: 'Kapitel' });
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await expect(menu.getByRole('menuitemradio')).toHaveCount(12);
  await arrowTo(page, menu.getByRole('menuitemradio', { name: /^Kapitel 5 · / }));
  await page.keyboard.press('Enter');
  await expect(menu).toHaveCount(0);
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(19);
  expect(await time(page)).toBeLessThanOrEqual(21);
  await expect(player.getByText(/^Kapitel 5 · 0:0\d \/ 0:05$/)).toBeVisible();

  // Shift+→: the next chapter; Shift+← near its start: the one before.
  await page.keyboard.press('Shift+ArrowRight');
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(24);
  expect(await time(page)).toBeLessThanOrEqual(26);
  await expect(player.getByText(/^Kapitel 6 · /)).toBeVisible();
  await page.keyboard.press('Shift+ArrowLeft');
  await expect.poll(() => time(page)).toBeLessThanOrEqual(21);
  expect(await time(page)).toBeGreaterThanOrEqual(19);
  // ←/→ move 15 s.
  await page.keyboard.press('ArrowRight');
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(34);
  // Escape closes a menu only (once it is open and focused); then it leaves the player.
  await page.keyboard.press('c');
  await expect(menu).toBeVisible();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(menu).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);
  await page.keyboard.press('Escape');
  await expect(page).not.toHaveURL(/\/play\//);
});

test('H-3: speed 2.0 stays after a reload and on a second device, the position ±2 s', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(90_000);
  await audioDetail(owner, m4b, 12);
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
  await playAudio(page, m4b);
  const player = audioPlayer(page);
  await expect(player).toHaveAttribute('data-rate', '1');

  // Ten presses of ] from 1.0: 2.0. The next progress event carries it.
  const rated = page.waitForRequest(
    (request) =>
      request.method() === 'POST' &&
      request.url().endsWith(`/items/${m4b}/progress`) &&
      (request.postDataJSON() as { playbackRate?: number }).playbackRate === 2,
    { timeout: 20_000 },
  );
  for (let press = 0; press < 10; press += 1) await page.keyboard.press(']');
  await expect(player).toHaveAttribute('data-rate', '2');
  expect((await audioState(page)).rate).toBe(2);
  // Mid-chapter (chapter 6 starts at 25 s), then pause: the pause reports position and speed.
  await page.locator('audio').evaluate((audio: HTMLAudioElement) => {
    audio.currentTime = 27.5;
  });
  await page.keyboard.press('k');
  await rated;
  await expect
    .poll(async () => {
      const item = (await (await owner.get(`/api/v1/items/${m4b}`)).json()) as {
        progress?: { positionMs: number; playbackRate?: number };
      };
      return item.progress;
    })
    .toMatchObject({ playbackRate: 2 });

  // Reload: the player starts at 2.0 where it stopped.
  await page.reload();
  await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
  await expect(player).toHaveAttribute('data-rate', '2');
  expect((await audioState(page)).rate).toBe(2);
  const resumed = await time(page);
  expect(Math.abs(resumed - 27.5), `resumed at ${resumed} s`).toBeLessThanOrEqual(2);
  await page.keyboard.press('k');

  // A second device of the account: the same speed and position.
  const second = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    const view = await second.newPage();
    await playAudio(view, m4b);
    await expect(audioPlayer(view)).toHaveAttribute('data-rate', '2');
    expect((await audioState(view)).rate).toBe(2);
    const there = await time(view);
    expect(Math.abs(there - 27.5), `second device at ${there} s`).toBeLessThanOrEqual(2);
    await expect(audioPlayer(view).getByText(/^Kapitel 6 · /)).toBeVisible();
  } finally {
    await second.close();
  }
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
});

test('audio player by keyboard: controls with a visible ring, menus, no focus trap, back', async ({
  page,
  owner,
}) => {
  await audioDetail(owner, m4b, 12);
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
  await playAudio(page, m4b);
  const player = audioPlayer(page);
  await expectFocusOn(page, player.getByRole('button', { name: 'Pause' }));
  // Space on the focused button pauses; Tab walks the transport and the extras.
  await page.keyboard.press('Space');
  await expect.poll(async () => (await audioState(page)).paused).toBe(true);
  await tabTo(page, player.getByRole('button', { name: '30 Sekunden vor' }), 3);
  await page.keyboard.press('Enter');
  await expect.poll(() => time(page)).toBeGreaterThanOrEqual(29);
  await tabTo(page, player.getByRole('button', { name: /^Geschwindigkeit: / }), 6);
  await page.keyboard.press('Enter');
  const speed = page.getByRole('menu', { name: /^Geschwindigkeit: / });
  await expect(speed).toBeVisible();
  await arrowTo(page, speed.getByRole('menuitemradio', { name: '1,5×' }));
  await page.keyboard.press('Enter');
  await expect(player).toHaveAttribute('data-rate', '1.5');
  await expectFocusOn(page, player.getByRole('button', { name: 'Geschwindigkeit: 1,5×' }));
  // T: the sleep timer menu; 15 minutes.
  await page.keyboard.press('t');
  const sleep = page.getByRole('menu', { name: 'Sleep-Timer' });
  await expect(sleep).toBeVisible();
  await arrowTo(page, sleep.getByRole('menuitemradio', { name: '15 Minuten' }));
  await page.keyboard.press('Enter');
  await expect(
    player.getByRole('button', { name: /^Sleep-Timer: noch 1[45]:\d\d$/ }),
  ).toBeVisible();
  await expectNoFocusTrap(page);
  // Backspace (a remote's back key) leaves the player.
  await player.getByRole('button', { name: /^(Pause|Abspielen)$/ }).focus();
  await page.keyboard.press('Backspace');
  await expect(page).not.toHaveURL(/\/play\//);
  await setAudioProgress(page.request, m4b, 0, 60_000, 1);
});

test('the start page lists the book under "Neu hinzugefügt" with a square cover', async ({
  page,
}) => {
  await page.goto('/');
  const row = page.getByRole('region', { name: 'Neu hinzugefügt' });
  const card = row
    .getByRole('link')
    .filter({ has: page.getByText(M4B_TITLE, { exact: true }) })
    .first();
  await expect(card).toBeVisible();
  await expect(card).toHaveAttribute('href', `/items/${m4b}`);
  await expect(card.getByText(AUDIO_AUTHOR)).toBeVisible();
  // The card's frame (its first child) holds the cover, square for audio titles.
  const frame = await card.evaluate((link) => {
    const box = link.firstElementChild!.getBoundingClientRect();
    return { width: box.width, height: box.height };
  });
  expect(Math.abs(frame.width - frame.height)).toBeLessThanOrEqual(1);
});
