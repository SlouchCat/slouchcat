import { expect, test } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';
import { arrowTo, recordEndedReason, startOf, videoState } from './playback';

/** DoD M1a: direct play starts in under 2 s in Chrome and Firefox. */
const START_BUDGET_MS = 2000;

let detail = '';

test.beforeEach(async ({ page }) => {
  detail ||= await itemPath(page, H264_TITLE);
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
});

test('Enter on play starts direct play in under 2 s and reports progress', async ({ page }) => {
  const progress = page.waitForRequest(
    (request) =>
      request.method() === 'POST' && /\/api\/v1\/items\/[^/]+\/progress$/.test(request.url()),
  );
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);

  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const startMs = Number(await player.getAttribute('data-start-ms'));
  test.info().annotations.push({ type: 'start-ms', description: String(startMs) });
  expect(startMs).toBeLessThan(START_BUDGET_MS);

  // Pausing sends a progress event right away.
  await page.keyboard.press('Space');
  const event = (await progress).postDataJSON() as { state: string; seq: number };
  expect(event.state).toBe('paused');
  expect(event.seq).toBeGreaterThanOrEqual(1);
});

test('player keys: Space, the zones of the bar (11.2), Tab and Escape', async ({ page }) => {
  // Enter on the detail page's play link opens the player with the focus on play/pause.
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const time = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
  const paused = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused);
  const row = page.getByRole('toolbar', { name: 'Wiedergabe' });
  const play = page.getByRole('button', { name: 'Abspielen' });
  const seek = page.getByRole('slider', { name: 'Wiedergabeposition' });

  // The play/pause button has focus; Space toggles.
  await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
  await page.keyboard.press('Space');
  await expect.poll(paused).toBe(true);
  await expectFocusOn(page, play);

  // In the button row ←/→ go from button to button and leave the position alone.
  const duration = await page
    .locator('video')
    .evaluate((video: HTMLVideoElement) => video.duration);
  const before = await time();
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: '30 Sekunden vor' }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, play);
  expect(await time()).toBe(before);

  // ↑ leads to the timeline, where ←/→ seek 10 s; ↓ leads back to the same button.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, seek);
  await page.keyboard.press('ArrowRight');
  // +10 s, or the end of the short test clip.
  await expect.poll(time).toBeGreaterThanOrEqual(Math.min(before + 9, duration - 0.1));
  await page.keyboard.press('ArrowLeft');
  await expect.poll(time).toBeLessThan(before + 5);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, play);

  // Controls stay visible while paused. The row is one Tab stop: Shift+Tab goes back to the
  // timeline, the time display and the head (picture in picture only where the browser has it);
  // Tab from the row leaves it, and the page has no focus trap.
  await expect(player).toHaveAttribute('data-controls', 'visible');
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, seek);
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('button', { name: /^\d+:\d+ \/ \d+:\d+$/ }));
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Vollbild' }));
  const back = page.getByRole('button', { name: 'Zurück', exact: true });
  for (
    let presses = 0;
    presses < 2 && !(await back.evaluate((b) => b === document.activeElement));
    presses += 1
  ) {
    await page.keyboard.press('Shift+Tab');
  }
  await expectFocusOn(page, back);
  await tabTo(page, play, 5);
  await page.keyboard.press('Tab');
  await expect(row.locator(':focus')).toHaveCount(0);
  await expectNoFocusTrap(page);

  // ? opens the shortcut help with the focus on its list (the video plays on beside it); Escape
  // closes it and stays in the player, the next Escape leaves.
  await page.locator('body').focus();
  await page.keyboard.press('?');
  const help = page.getByRole('dialog', { name: 'Tastenkürzel' });
  await expect(help).toBeVisible();
  await expectFocusOn(page, help.locator('dl'));
  await page.keyboard.press('Escape');
  await expect(help).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);

  await page.locator('body').focus();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test('panels and the gear by keyboard: open, arrows, a page and back, Escape; no trap', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Space');
  const rate = () =>
    page.locator('video').evaluate((video: HTMLVideoElement) => video.playbackRate);

  // The sound panel from the row: → walks to its button, Enter opens it on the track chosen; the
  // arrows stay in it, Escape gives the focus back to the button.
  const sound = page.getByRole('button', { name: /^Ton: / });
  await arrowTo(page, sound, 'ArrowRight');
  await page.keyboard.press('Enter');
  const soundPanel = page.getByRole('dialog', { name: 'Ton' });
  await expect(sound).toHaveAttribute('aria-expanded', 'true');
  await expectFocusOn(page, soundPanel.getByRole('option', { selected: true }));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, soundPanel.getByRole('option', { selected: true }));
  await page.keyboard.press('Escape');
  await expect(soundPanel).toHaveCount(0);
  await expectFocusOn(page, sound);

  // U opens the subtitle panel on «Aus».
  await page.keyboard.press('u');
  const subtitles = page.getByRole('dialog', { name: 'Untertitel' });
  await expectFocusOn(page, subtitles.getByRole('option', { name: 'Aus' }));
  await page.keyboard.press('Backspace');
  await expect(subtitles).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);

  // The gear: its first page leads to the speed page; Enter picks a speed; Backspace goes up,
  // Escape closes with the focus back on the gear.
  const gear = page.getByRole('button', { name: 'Einstellungen' });
  await arrowTo(page, gear, 'ArrowRight');
  await page.keyboard.press('Enter');
  const settings = page.getByRole('dialog', { name: 'Einstellungen' });
  const speedRow = settings.getByRole('option', { name: 'Geschwindigkeit' });
  await expect(speedRow).toHaveAccessibleDescription('1×');
  await arrowTo(page, speedRow);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const speed = page.getByRole('dialog', { name: 'Geschwindigkeit' });
  await expect(speed.getByRole('button', { name: 'Zurück' })).toBeVisible();
  await expectFocusOn(page, speed.getByRole('option', { name: '1×' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, speed.getByRole('option', { name: '1,25×' }));
  await page.keyboard.press('Enter');
  await expect.poll(rate).toBe(1.25);
  await page.keyboard.press('Backspace');
  await expect(speedRow).toHaveAccessibleDescription('1,25×');
  await expectVisibleFocus(page);

  // The sound options: Space switches night mode on (Web Audio, Chromium and Firefox).
  const soundRow = settings.getByRole('option', { name: 'Ton-Optionen' });
  await arrowTo(page, soundRow);
  await page.keyboard.press('Enter');
  const night = page.getByRole('dialog', { name: 'Ton-Optionen' }).getByRole('switch', {
    name: 'Nachtmodus',
  });
  await expect(night).toBeFocused();
  await page.keyboard.press('Space');
  await expect(night).toBeChecked();
  await page.keyboard.press('Backspace');
  await expect(soundRow).toHaveAccessibleDescription('Nachtmodus an');
  await page.keyboard.press('Escape');
  await expect(settings).toHaveCount(0);
  await expectFocusOn(page, gear);

  // A panel is no trap: Tab walks out of it and on to the end of the page.
  await page.keyboard.press('Enter');
  await expect(settings).toBeVisible();
  await expectNoFocusTrap(page);
});

test('the subtitle panel and its style page by the arrows alone: a remote has no Tab', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Space');

  // U → ↓ past the end of the list to «Untertitel-Stil» → Enter.
  await page.keyboard.press('u');
  const subtitles = page.getByRole('dialog', { name: 'Untertitel' });
  await expectFocusOn(page, subtitles.getByRole('option', { name: 'Aus' }));
  const styleButton = subtitles.getByRole('button', { name: 'Untertitel-Stil' });
  await arrowTo(page, styleButton);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');

  // ↓ through the four lists down to the offset; → to «später»; Enter moves the cues by 0.1 s.
  const style = page.getByRole('dialog', { name: 'Untertitel-Stil' });
  await expectFocusOn(page, style.getByRole('option', { name: 'M', exact: true }));
  await arrowTo(page, style.getByRole('button', { name: '0,1 s früher' }), 'ArrowDown', 16);
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowRight');
  const later = style.getByRole('button', { name: '0,1 s später' });
  await expectFocusOn(page, later);
  await page.keyboard.press('Enter');
  await expect(style.getByText('+0,1 s')).toBeVisible();
  await expectFocusOn(page, later);
  // ↑ goes back up into the lists; Backspace up to the subtitle panel, Escape closes it.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, style.getByRole('option', { name: 'Unten' }));
  await page.keyboard.press('Backspace');
  await expect(subtitles).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);
});

test('a volume above 100 % and night mode keep the video playing (Web Audio)', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  const video = page.locator('video');
  const paused = () => video.evaluate((element: HTMLVideoElement) => element.paused);
  const time = () => video.evaluate((element: HTMLVideoElement) => element.currentTime);

  // The slider goes up to 150 %; ↑ with the focus outside a zone raises the volume past 100 %
  // and says that it is boosted.
  const slider = page.getByRole('slider', { name: 'Lautstärke' });
  await expect(slider).toHaveAttribute('aria-valuemax', '150');
  await page.locator('body').focus();
  // From the level this device starts at (100 % in a fresh browser) up past 100 %.
  for (let presses = 0; presses < 30; presses += 1) {
    if (Number(await slider.getAttribute('aria-valuenow')) > 100) break;
    await page.keyboard.press('ArrowUp');
  }
  await expect(slider).toHaveAttribute('aria-valuenow', '105');
  await expect(player.getByRole('status')).toHaveText(
    'Lautstärke 105 %, verstärkt, kann verzerren',
  );
  await expect.poll(() => video.evaluate((element: HTMLVideoElement) => element.volume)).toBe(1);

  // Night mode through the gear, by keyboard: Tab to the row, → along it to the gear (review step
  // 5); the video plays on and nothing broke.
  const row = page.getByRole('toolbar', { name: 'Wiedergabe' });
  await tabTo(page, row.getByRole('button', { name: /^(Pause|Abspielen)$/ }), 10);
  const gear = row.getByRole('button', { name: /^Einstellungen/ });
  await arrowTo(page, gear, 'ArrowRight');
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  const settings = page.getByRole('dialog', { name: 'Einstellungen' });
  await arrowTo(page, settings.getByRole('option', { name: 'Ton-Optionen' }));
  await page.keyboard.press('Enter');
  const night = page.getByRole('dialog', { name: 'Ton-Optionen' }).getByRole('switch', {
    name: 'Nachtmodus',
  });
  await expect(night).toBeFocused();
  await page.keyboard.press('Space');
  await expect(night).toBeChecked();
  const before = await time();
  await expect.poll(time, { timeout: 5000 }).toBeGreaterThan(before + 0.5);
  expect(await paused()).toBe(false);
  await expect(player).toBeVisible();
  await expect(page.getByRole('alert')).toHaveCount(0);
});

test('the tab «Info» and the pause view by keyboard: ↓ to the tabs, Enter, ↓ into the text', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await page.keyboard.press('Space');
  const play = page.getByRole('button', { name: 'Abspielen' });
  await expectFocusOn(page, play);

  // Five seconds of pause without input: the pause view; it takes no focus, a key hides it.
  const pauseView = page.getByRole('region', { name: 'Pause' });
  await expect(pauseView).toBeVisible({ timeout: 8000 });
  await expect(pauseView.getByRole('heading', { name: H264_TITLE })).toBeVisible();
  await expectFocusOn(page, play);
  await page.keyboard.press('Shift');
  await expect(pauseView).toHaveCount(0);

  // ↓ from the row to the tab strip, → to «Info», Enter opens it, ↓ into its text.
  await page.keyboard.press('ArrowDown');
  const strip = page.getByRole('toolbar', { name: 'Mehr zum Titel' });
  const info = strip.getByRole('button', { name: 'Info' });
  await arrowTo(page, info, 'ArrowRight');
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(info).toHaveAttribute('aria-expanded', 'true');
  const panel = page.getByRole('region', { name: 'Info' });
  await expect(panel).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expect(panel.locator('[data-tab-content]')).toBeFocused();
  await expectVisibleFocus(page);
  // No pause view while a tab is open; the controls stay.
  await page.waitForTimeout(5500);
  await expect(pauseView).toHaveCount(0);
  await expect(player).toHaveAttribute('data-controls', 'visible');
  // ↑ to the strip, ↑ to the row; Tab from the row goes to the strip, then on: no trap.
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, info);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, play);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, info);
  await expectNoFocusTrap(page);
  // Escape closes the tab, the next one leaves the player.
  await page.locator('body').focus();
  await page.keyboard.press('Escape');
  await expect(panel).toHaveCount(0);
  await expect(page).toHaveURL(/\/play\//);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test('a slow connection suggests the rung below by keyboard and switches only on Enter', async ({
  page,
}) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  // Two stalls of over a second within a minute (V-18), as the element reports them.
  const video = page.locator('video');
  for (let stalls = 0; stalls < 2; stalls += 1) {
    await video.evaluate((element: HTMLVideoElement) =>
      element.dispatchEvent(new Event('waiting')),
    );
    await page.waitForTimeout(1300);
    await video.evaluate((element: HTMLVideoElement) =>
      element.dispatchEvent(new Event('playing')),
    );
  }
  const hint = page.getByRole('region', { name: 'Langsame Verbindung' });
  await expect(hint.getByRole('status')).toHaveText(/^Verbindung langsam – auf \d+p wechseln\?$/);
  const session = await player.getAttribute('data-playback-session-id');
  // It took no focus and switched nothing.
  await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
  expect(await player.getAttribute('data-playback-session-id')).toBe(session);
  const switchButton = hint.getByRole('button', { name: /^Auf \d+p wechseln$/ });
  // By the arrows alone (review step 5): ↑ to the timeline, ↑ to the hint, ←/→ between its
  // buttons, ↓ back to the timeline. Paused, the controls stay and the arrows move between zones.
  await video.evaluate((element: HTMLVideoElement) => element.pause());
  await expect(player).toHaveAttribute('data-controls', 'visible');
  const seek = page.getByRole('slider', { name: 'Wiedergabeposition' });
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, seek);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, switchButton);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, hint.getByRole('button', { name: 'Nicht jetzt' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, seek);
  await page.keyboard.press('ArrowUp');
  await expectFocusOn(page, switchButton);
  const rung = /^Auf (\d+p) wechseln$/.exec((await switchButton.textContent()) ?? '')?.[1];
  const next = await startOf(page, () => page.keyboard.press('Enter'));
  expect(next.request.quality).toBe(rung);
  await expect(hint).toHaveCount(0);
  await expect(player).toHaveAttribute('data-playback-session-id', next.body.playbackSessionId);
  // V-17 remembers only a choice in the quality panel, never the hint's.
  const stored = await page.evaluate(() => localStorage.getItem('slouchcat.player'));
  expect(JSON.parse(stored ?? '{}')).not.toHaveProperty('quality');
});

test('controls hide after 3 s of playback and come back on a key press', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  await expect(player).toHaveAttribute('data-controls', 'hidden', { timeout: 6000 });
  await page.keyboard.press('Shift');
  await expect(player).toHaveAttribute('data-controls', 'visible');
  await expectVisibleFocus(page);

  // Faded out, the bar has no zone: → seeks although play/pause has the focus, which stays.
  const time = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
  const duration = await page
    .locator('video')
    .evaluate((video: HTMLVideoElement) => video.duration);
  await page.keyboard.press('0');
  await expect(player).toHaveAttribute('data-controls', 'hidden', { timeout: 6000 });
  const before = await time();
  await page.keyboard.press('ArrowRight');
  await expect.poll(time).toBeGreaterThanOrEqual(Math.min(before + 9, duration - 0.1));
  await expectFocusOn(page, page.getByRole('button', { name: /^(Pause|Abspielen)$/ }));
  await expect(player).toHaveAttribute('data-controls', 'visible');
});

test('Backspace and browser back return to the detail page', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/play\//);
  await page.keyboard.press('Backspace');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await page.goForward();
  await expect(page).toHaveURL(/\/play\//);
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
});

test.describe('the tab «Besetzung»', () => {
  // The route below must see the request; a service worker would make it on its own.
  test.use({ serviceWorkers: 'block' });

  test('↓ to the tabs, Enter, ↓, Enter opens the person; back returns to the player (review step 5)', async ({
    page,
  }) => {
    // The e2e server has no TMDB key, so no title has a cast: the title's details get one here.
    const itemId = detail.split('/').pop()!;
    await page.route(`**/api/v1/items/${itemId}`, async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      await route.fulfill({
        response,
        json: { ...body, cast: [{ personId: 1, name: 'Ellen Ripley', role: 'cast' }] },
      });
    });
    const first = await startOf(page, async () => {
      await page.keyboard.press('ArrowDown');
      await page.keyboard.press('Enter');
    });
    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
    await page.keyboard.press('Space');
    const strip = page.getByRole('toolbar', { name: 'Mehr zum Titel' });
    const castTab = strip.getByRole('button', { name: 'Besetzung' });
    await expect(castTab).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await expect(strip.locator(':focus')).toHaveCount(1);
    await arrowTo(page, castTab, 'ArrowRight');
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    const person = page.getByRole('list', { name: 'Besetzung' }).getByRole('button', {
      name: /Ellen Ripley/,
    });
    await expectFocusOn(page, person);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/people\/1$/);
    // Without TMDB nobody has a credit: the person's page is the not-found notice (person.spec).
    await expect(page.getByText('Person nicht gefunden')).toBeVisible();
    // Back: the player opens again with a new session (leaving it ended the first) and plays.
    const endedReason = await recordEndedReason(page);
    const second = await startOf(page, () => page.keyboard.press('Escape'));
    await expect(page).toHaveURL(/\/play\//);
    expect(second.status).toBe(200);
    expect(second.body.playbackSessionId).not.toBe(first.body.playbackSessionId);
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await expect.poll(async () => (await videoState(page)).paused).toBe(false);
    const playing = (await videoState(page)).time;
    await expect.poll(async () => (await videoState(page)).time).toBeGreaterThan(playing);
    // Longer than one round of the stop watch (4 s): the new session is not stopped.
    await page.waitForTimeout(5000);
    expect(await endedReason()).toBeNull();
    await expect(player).not.toHaveAttribute('data-ended-reason', /.*/);
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });

  test('Escape right after Enter on the person, while its page loads, stays with the player', async ({
    page,
  }) => {
    const itemId = detail.split('/').pop()!;
    await page.route(`**/api/v1/items/${itemId}`, async (route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      await route.fulfill({
        response,
        json: { ...body, cast: [{ personId: 1, name: 'Ellen Ripley', role: 'cast' }] },
      });
    });
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
    await page.keyboard.press('Space');
    const strip = page.getByRole('toolbar', { name: 'Mehr zum Titel' });
    const castTab = strip.getByRole('button', { name: 'Besetzung' });
    await expect(castTab).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await expect(strip.locator(':focus')).toHaveCount(1);
    await arrowTo(page, castTab, 'ArrowRight');
    await page.keyboard.press('Enter');
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(
      page,
      page.getByRole('list', { name: 'Besetzung' }).getByRole('button', { name: /Ellen Ripley/ }),
    );
    // Escape without waiting for the person's page: the player had the key until it loaded.
    const personChunk = page.waitForResponse(/\/assets\/PersonPage-[^/]*\.js$/);
    await page.keyboard.press('Enter');
    await page.keyboard.press('Escape');
    // Once the page's code is there, the router has settled: back at the player, either still the
    // same one or opened again.
    await personChunk;
    await expect(page).toHaveURL(/\/play\//);
    await expect(player).toBeVisible();
    await expect(page.getByText('Person nicht gefunden')).toHaveCount(0);
    await page.unrouteAll({ behavior: 'ignoreErrors' });
  });
});

test.describe('in a narrow window with a mouse', () => {
  // Under 768 px the phone layout applies on a laptop as well (review step 5, M-1).
  test.use({ viewport: { width: 600, height: 800 } });

  test('a click on the picture pauses, a double click does not seek', async ({ page }) => {
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await expect(page.getByRole('toolbar', { name: 'Optionen' })).toBeVisible();
    const paused = () => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused);
    await expect.poll(paused).toBe(false);
    await page.mouse.click(150, 400);
    await expect.poll(paused).toBe(true);
    await page.mouse.click(150, 400);
    await expect.poll(paused).toBe(false);
    // A double click is two clicks and full screen, never 10 s back.
    const time = () =>
      page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);
    await page.locator('video').evaluate((video: HTMLVideoElement) => {
      video.currentTime = Math.min(video.duration - 1, 15);
    });
    const before = await time();
    await page.mouse.dblclick(150, 400);
    expect(await time()).toBeGreaterThanOrEqual(before - 0.5);
    await expect(player.getByText('−10 s')).toHaveCount(0);
  });
});

test.describe('on a phone', () => {
  test.use({ viewport: { width: 390, height: 844 }, hasTouch: true });

  test('the phone layout by keyboard: options at the top, transport in the middle; two taps seek', async ({
    page,
  }) => {
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('Enter');
    const player = page.getByRole('main', { name: 'Videoplayer' });
    await expect(player).toHaveAttribute('data-start-ms', /\d+/);
    await page.keyboard.press('Space');
    const play = page.getByRole('button', { name: 'Abspielen' });
    await expectFocusOn(page, play);
    const time = () =>
      page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);

    // No tabs and no shortcut help; −10 s, play and +10 s in the middle.
    await expect(page.getByRole('toolbar', { name: 'Mehr zum Titel' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Tastenkürzel' })).toHaveCount(0);
    const transport = page.getByRole('toolbar', { name: 'Wiedergabe' });
    await expect(transport.getByRole('button')).toHaveCount(3);
    const box = await transport.boundingBox();
    expect(Math.abs((box?.x ?? 0) + (box?.width ?? 0) / 2 - 195)).toBeLessThan(20);

    // ↑ to the options at the top right, → along them, ↓ back to the transport and to the
    // timeline, ↑ back up.
    const options = page.getByRole('toolbar', { name: 'Optionen' });
    await page.keyboard.press('ArrowUp');
    await expect(options.locator(':focus')).toHaveCount(1);
    await expectVisibleFocus(page);
    await arrowTo(page, options.getByRole('button', { name: /^Einstellungen/ }), 'ArrowRight');
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, play);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, page.getByRole('button', { name: '10 Sekunden vor' }));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, page.getByRole('slider', { name: 'Wiedergabeposition' }));
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, page.getByRole('button', { name: '10 Sekunden vor' }));

    // The gear opens as a sheet from the bottom; Escape closes it, the focus back on the gear.
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    const settings = page.getByRole('dialog', { name: 'Einstellungen' });
    await expect(settings).toBeVisible();
    const sheet = await settings.boundingBox();
    expect(Math.round((sheet?.y ?? 0) + (sheet?.height ?? 0))).toBe(844);
    await page.keyboard.press('Escape');
    await expect(settings).toHaveCount(0);
    await expectFocusOn(page, options.getByRole('button', { name: /^Einstellungen/ }));
    await expectNoFocusTrap(page);

    // Two taps on the left half: 10 s back; on the right half: 10 s on (the test clip is short,
    // so both start near an end).
    await page.locator('video').evaluate((video: HTMLVideoElement) => {
      video.currentTime = Math.max(0, video.duration - 1);
    });
    const before = await time();
    await page.touchscreen.tap(40, 422);
    await page.touchscreen.tap(40, 422);
    await expect.poll(time).toBeLessThan(before - 5);
    await expect(player.getByRole('status')).toHaveText('−10 s');
    const after = await time();
    await page.touchscreen.tap(350, 422);
    await page.touchscreen.tap(350, 422);
    await expect.poll(time).toBeGreaterThan(after + 5);
    // Two taps neither play nor go full screen.
    expect(await page.evaluate(() => document.fullscreenElement)).toBeNull();
    expect(await page.locator('video').evaluate((video: HTMLVideoElement) => video.paused)).toBe(
      true,
    );
  });
});
