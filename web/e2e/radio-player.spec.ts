import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { CSRF, H264_TITLE } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  focused,
  resetFocus,
  tabTo,
} from './keyboard';

// The app-level radio player (concept 16, 10.3; plan W4) against the test station of
// radio-server.mjs: «Testradio Drei» is the Icecast AAC mount, which the relay repackages (copy
// mode, B-1), «Testradio Eins» the MP3 mount, which it transcodes; both send ICY titles
// («Slouchcat Testton - Titel n»). In Chromium the element must play (its time advances, no media
// error: MSE took the init segment with its AudioSpecificConfig). Firefox runs the whole file
// (FIREFOX_WHOLE_SPECS); its CI runner has no audio output (PR #18), so it checks state and text.
//
// The mini player is reached by Tab, Space pauses, Enter opens «Jetzt läuft», Escape closes it
// with the focus back on the mini player; the arrow keys lead from the page down into the mini
// player and up back into the page (AppLayout); navigation (also into /admin) and Back keep
// playing, a title on /play stops the radio and ends the listen session; a second page that
// starts radio pauses the first; no focus trap. Since concept 16 stage 2 the relay keeps a time
// shift window of 10 minutes (D-14: the radiotest build takes the disk buffer as a tmpfs): the
// slider of «Jetzt läuft» goes behind live by keyboard, «Live» returns, «Zuvor gespielt» lists the
// earlier titles.

const AAC_STATION = 'Testradio Drei';
const MP3_STATION = 'Testradio Eins';
const OTHER_STATION = 'Testradio Zwei';
/**
 * The ICY title «Slouchcat Testton - Titel n»: the server splits it into artist and title, the mini
 * player joins them with a dash, «Jetzt läuft» shows the title large and the artist below.
 */
const ICY_LINE = /^Slouchcat Testton – Titel \d+$/;
const ICY_TITLE = /^Titel \d+$/;
/** The title inside a longer text (a row of «Zuvor gespielt» with its time). */
const ICY_TITLE_PART = /Titel \d+/;
/** Relay start (ffmpeg, the first segments) plus the first title poll (10 s). */
const LIVE_TIMEOUT_MS = 30_000;

/** Ends every listen session the administration sees (the fixture knows only playback sessions). */
async function stopRadio(owner: APIRequestContext): Promise<void> {
  const response = await owner.get('/api/v1/admin/radio/relays');
  if (!response.ok()) return;
  const { relays } = (await response.json()) as { relays: { listeners: { listenId: string }[] }[] };
  for (const listener of relays.flatMap((relay) => relay.listeners)) {
    await owner.delete(`/api/v1/admin/radio/listeners/${listener.listenId}`, { headers: CSRF });
  }
}

/** No stored filter: /radio opens as it does for a new profile. */
async function resetRadioPreferences(owner: APIRequestContext): Promise<void> {
  const response = await owner.put('/api/v1/radio/preferences', {
    headers: CSRF,
    data: { defaultCountry: 'CH', lastFilter: {}, lastSort: 'popular' },
  });
  expect(response.status(), await response.text()).toBe(200);
}

const miniPlayer = (page: Page) => page.getByTestId('radio-mini-player');
const miniOpen = (page: Page) => page.getByTestId('radio-mini-open');
const audio = (page: Page) => page.getByTestId('radio-audio');

/** Opens /radio on `station` and plays it from its tile. */
async function playStation(page: Page, station: string): Promise<void> {
  await page.goto(`/radio?q=${encodeURIComponent(station)}`);
  const tile = page
    .getByRole('list', { name: 'Treffer' })
    .getByRole('button', { name: `${station} abspielen` });
  await tile.click();
  await expect(miniPlayer(page)).toBeVisible();
  await expect(miniOpen(page)).toHaveAccessibleName(`Jetzt läuft öffnen: ${station}`);
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live', {
    timeout: LIVE_TIMEOUT_MS,
  });
}

/** Presses `key` until the focus is inside `region`; fails after `max` presses. */
async function arrowInto(page: Page, region: Locator, key: string, max = 10): Promise<void> {
  for (let presses = 0; presses < max; presses += 1) {
    await page.keyboard.press(key);
    if (await region.evaluate((element) => element.contains(document.activeElement))) {
      await expectVisibleFocus(page);
      return;
    }
  }
  const now = await focused(page);
  throw new Error(
    `${key} did not reach the region within ${max} presses (at ${now.tag}:${now.name})`,
  );
}

/** The element's playback position (s) and its media error. */
const media = (element: Locator) =>
  element.evaluate((node: HTMLAudioElement) => ({
    time: node.currentTime,
    paused: node.paused,
    error: node.error?.code ?? null,
  }));

/** Chromium only: the radio's own element plays and its position advances by `seconds`. */
async function expectSounding(page: Page, browserName: string, seconds = 3): Promise<void> {
  if (browserName !== 'chromium') return;
  const start = (await media(audio(page))).time;
  await expect
    .poll(async () => (await media(audio(page))).time - start, { timeout: LIVE_TIMEOUT_MS })
    .toBeGreaterThanOrEqual(seconds);
  expect((await media(audio(page))).error).toBeNull();
}

test.beforeEach(async ({ owner }) => {
  await stopRadio(owner);
  await resetRadioPreferences(owner);
});

test.afterEach(async ({ owner }) => {
  await stopRadio(owner);
});

test('AAC in copy mode plays; mini player and «Jetzt läuft» by keyboard; navigation keeps it, /play stops it @keyboard', async ({
  page,
  browserName,
}) => {
  test.setTimeout(120_000);
  await playStation(page, AAC_STATION);
  await expectSounding(page, browserName);

  // Tab reaches the mini player; Space pauses and plays on wherever the focus is in it.
  await resetFocus(page);
  await tabTo(page, miniOpen(page), 80);
  await page.keyboard.press('Space');
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'paused');
  await expect(page.getByTestId('radio-mini-toggle')).toHaveAccessibleName('Weiter abspielen');
  if (browserName === 'chromium')
    await expect.poll(async () => (await media(audio(page))).paused).toBe(true);
  // Space on the open button did not open «Jetzt läuft».
  await expect(page.getByRole('dialog', { name: 'Jetzt läuft' })).toHaveCount(0);
  await page.keyboard.press('Space');
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live');
  await expectSounding(page, browserName, 1);

  // Enter opens «Jetzt läuft»; Escape closes it and the focus is back on the mini player.
  await expectFocusOn(page, miniOpen(page));
  await page.keyboard.press('Enter');
  const now = page.getByRole('dialog', { name: 'Jetzt läuft' });
  await expect(now).toBeVisible();
  await expect(now.getByTestId('radio-now-title')).toBeVisible();
  await expect(now.getByRole('button', { name: 'Live' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(now).toHaveCount(0);
  await expectFocusOn(page, miniOpen(page));
  await expectNoFocusTrap(page);

  // Arrow keys: down from the page's last row reaches the mini player, which floats over the
  // bottom of the content; up leads back to the tile the focus came from.
  const tile = page
    .getByRole('list', { name: 'Treffer' })
    .getByRole('button', { name: `${AAC_STATION} pausieren` });
  await resetFocus(page);
  await tabTo(page, tile, 60);
  await arrowInto(page, miniPlayer(page), 'ArrowDown');
  await arrowInto(page, page.getByTestId('radio-page'), 'ArrowUp');
  await expectFocusOn(page, tile);
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live');

  // Navigation inside the app, into the administration too, keeps playing.
  await page
    .getByRole('navigation', { name: 'Hauptnavigation' })
    .getByRole('link', { name: /^Verwaltung/ })
    .click();
  await expect(page).toHaveURL(/\/admin$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live');
  await expectSounding(page, browserName, 1);

  // Back returns to the radio page; the station plays on.
  await page.goBack();
  await expect(page).toHaveURL(/\/radio\?q=/);
  await expect(page.getByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live');
  await expectSounding(page, browserName, 1);

  // A title starts on /play (reached through the app: search, detail page, «Abspielen»): the
  // radio stops and ends its listen session on the server.
  await page.getByRole('link', { name: 'Suche öffnen' }).click();
  await page.getByRole('searchbox', { name: 'Suchbegriff' }).fill(H264_TITLE);
  await page
    .getByRole('link', { name: new RegExp(H264_TITLE) })
    .first()
    .click();
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  const ended = page.waitForRequest(
    (request) => request.method() === 'DELETE' && /\/api\/v1\/radio\/listen\//.test(request.url()),
  );
  await page.locator('main a[href^="/play/"]').first().click();
  await expect(page).toHaveURL(/\/play\//);
  await ended;
  await expect(audio(page)).toHaveCount(0);
});

test('the transcoded MP3 station shows the title of its stream', async ({ page, browserName }) => {
  test.setTimeout(90_000);
  await playStation(page, MP3_STATION);
  await expectSounding(page, browserName);
  await expect(page.getByTestId('radio-mini-line')).toHaveText(ICY_LINE, {
    timeout: LIVE_TIMEOUT_MS,
  });
  await miniOpen(page).click();
  const now = page.getByRole('dialog', { name: 'Jetzt läuft' });
  await expect(now.getByTestId('radio-now-title')).toHaveText(ICY_TITLE);
  // The artist under the title (the live region); «Zuvor gespielt» may name it again below.
  await expect(
    now.locator('[aria-live="polite"]').getByText('Slouchcat Testton', { exact: true }),
  ).toBeVisible();
  await expect(now.getByText('Über Slouchcat, Standort des Servers')).toBeVisible();
  await page.keyboard.press('Escape');
  // «Radio beenden» ends the session.
  const ended = page.waitForRequest(
    (request) => request.method() === 'DELETE' && /\/api\/v1\/radio\/listen\//.test(request.url()),
  );
  await page.getByTestId('radio-mini-close').click();
  await ended;
  await expect(miniPlayer(page)).toHaveCount(0);
});

test('a second page that starts radio pauses the first', async ({ page, context }) => {
  test.setTimeout(90_000);
  await playStation(page, MP3_STATION);
  const second = await context.newPage();
  try {
    await playStation(second, OTHER_STATION);
    await expect(miniPlayer(page)).toHaveAttribute('data-state', 'paused');
    await expect(miniPlayer(second)).toHaveAttribute('data-state', 'live');
  } finally {
    await second.close();
  }
});

test('«Jetzt läuft» by keyboard: the slider goes behind live, «Zuvor gespielt», «Live» returns @keyboard', async ({
  page,
}) => {
  test.setTimeout(150_000);
  await playStation(page, AAC_STATION);
  await expect(page.getByTestId('radio-mini-line')).toHaveText(ICY_LINE, {
    timeout: LIVE_TIMEOUT_MS,
  });

  await resetFocus(page);
  await tabTo(page, miniOpen(page), 80);
  await page.keyboard.press('Enter');
  const now = page.getByRole('dialog', { name: 'Jetzt läuft' });
  await expect(now).toBeVisible();

  // The test station names a new title every 10 s: once one followed the first, «Zuvor gespielt»
  // lists the earlier one (the timeline from the start of the window, 7.6).
  const played = now.getByTestId('radio-played');
  await expect(played.getByRole('listitem').first()).toContainText(ICY_TITLE_PART, {
    timeout: 60_000,
  });
  const titleNumber = (text: string) => Number(/Titel (\d+)/.exec(text)?.[1]);
  const current = titleNumber(await now.getByTestId('radio-now-title').innerText());
  const previous = titleNumber(await played.getByRole('listitem').first().innerText());
  expect(previous).toBeLessThan(current);

  // The slider (10 minutes in the suite, D-14): Tab reaches it with a visible ring, the arrow keys
  // move it in steps of 5 s, the player seeks once it rests.
  const slider = now.getByRole('slider', { name: 'Zeitversatz, bis 10 Minuten zurück' });
  await tabTo(page, slider, 20);
  await expect(slider).toHaveAttribute('aria-valuetext', 'Live');
  await page.keyboard.press('ArrowLeft');
  await page.keyboard.press('ArrowLeft');
  await expect(slider).toHaveAttribute('aria-valuetext', '10 Sekunden zurück');
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'shifted', { timeout: 15_000 });
  await expect(page.getByTestId('radio-mini-state')).toHaveText(/^−\d+:\d\d$/);
  const live = now.getByTestId('radio-now-live');
  await expect(live).toHaveAttribute('aria-pressed', 'false');

  // «Live» returns to the live position.
  await tabTo(page, live, 10);
  await page.keyboard.press('Enter');
  await expect(live).toHaveAttribute('aria-pressed', 'true', { timeout: 15_000 });
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live');
  await expect(slider).toHaveAttribute('aria-valuetext', 'Live');

  // Escape closes it with the focus back on the mini player; no focus trap.
  await page.keyboard.press('Escape');
  await expect(now).toHaveCount(0);
  await expectFocusOn(page, miniOpen(page));
  await expectNoFocusTrap(page);
});
