import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { STORAGE_STATE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { H264_TITLE, itemIdByTitle, play, player, startOf, useFreshProfile } from './playback';

// K-5: a profile with a daily limit of 1 minute. Progress events with 60 s of playing time use it
// up; the next start answers 403 watch_limit_reached and the player shows the hint, a running
// player stops with the hint, and the primary profile's report shows the minute.

// The worker registers on the first page load and takes the page over (clientsClaim) within
// seconds; from then on it sends every GET under /api to the network itself, where the page's
// routes do not see it. The stopping test switches the stop watch off by aborting its GET through
// a route, which would stop working in the middle of the test. The player needs no worker.
test.use({ serviceWorkers: 'block' });

interface ProgressAnswer {
  watchTime?: { limitMinutes: number; usedMinutes: number; remainingMinutes: number };
}

/** Six `playing` events of 10 s each: one minute of watch time for the session's profile. */
/**
 * One minute as another device of the profile would report it: a single first event of 60 s.
 * The server trusts the first event of a playback (device, file, session) as reported and caps
 * every later one to the time that really passed since the previous event, so six quick events of
 * 10 s from one browser would count as about 10 s.
 */
async function watchOneMinute(api: APIRequestContext, itemId: string): Promise<ProgressAnswer> {
  const response = await api.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 6000,
      durationMs: 10_000,
      playedMs: 60_000,
      seq: 1,
      state: 'playing',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as ProgressAnswer;
}

test('one minute played: the start answers 403 with the hint; the report shows the minute', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  const profile = await useFreshProfile(page, owner, 'E2E Schauzeit', { watchLimitMinutes: 1 });

  const answer = await watchOneMinute(page.request, itemId);
  expect(answer.watchTime).toMatchObject({ limitMinutes: 1, remainingMinutes: 0 });
  expect(answer.watchTime?.usedMinutes).toBeGreaterThanOrEqual(1);

  // Enter on the detail page's play link opens the hint (concept 13, 11.13: the back button takes
  // the focus through focusOnMount, react-aria's autoFocus waited for the cancelled transition).
  await page.goto(`/items/${itemId}`);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  const playLink = page.getByRole('link', {
    name: /^(Abspielen|Fortsetzen|Nochmals ansehen)( |$)/,
  });
  await playLink.focus();
  const start = await startOf(page, () => page.keyboard.press('Enter'));
  expect(start.status).toBe(403);
  expect(start.body.code).toBe('watch_limit_reached');
  expect(start.body.extensions?.limitMinutes).toBe(1);
  await expect(page.getByText('Schauzeit für heute aufgebraucht')).toBeVisible();
  await expect(
    page.getByText('Dieses Profil darf 1 Minute pro Tag schauen. Morgen geht es weiter.'),
  ).toBeVisible();
  await expect(page.locator('main')).toHaveAttribute('data-error-code', 'watch_limit_reached');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));

  // The report through the API (primary profile only) and on the settings page.
  const report = (await (
    await owner.get(`/api/v1/profiles/${profile.id}/watch-time?days=30`)
  ).json()) as { limitMinutes?: number; days: { date: string; minutes: number }[] };
  expect(report.limitMinutes).toBe(1);
  expect(report.days).toHaveLength(30);
  expect(report.days.at(-1)?.minutes ?? 0).toBeGreaterThanOrEqual(1);

  const admin = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: STORAGE_STATE,
  });
  try {
    const settings = await admin.newPage();
    await settings.goto('/settings/profiles');
    const card = settings.getByRole('article', { name: 'E2E Schauzeit' });
    await expect(card).toBeVisible();
    await expect(card.getByText('1 min pro Tag')).toBeVisible();
    // Keyboard: Tab to the report button, Enter opens the report.
    await resetFocus(settings);
    const toggle = card.getByRole('button', { name: 'Schauzeit anzeigen' });
    await tabTo(settings, toggle, 60);
    await settings.keyboard.press('Enter');
    const section = card.getByRole('region', { name: 'Schauzeit von E2E Schauzeit' });
    await expect(section).toBeVisible();
    await expect(section.getByText(/^Heute: [1-9]\d* min$/)).toBeVisible();
    await expect(section.getByText('Limit: 1 min pro Tag')).toBeVisible();
    await expect(section.getByRole('table', { name: 'Schauzeit pro Tag' })).toBeAttached();
    await expectFocusOn(settings, card.getByRole('button', { name: 'Schauzeit ausblenden' }));
    await expectNoFocusTrap(settings);
  } finally {
    await admin.close();
  }
});

test('a running player stops with the hint once the limit is reached', async ({ page, owner }) => {
  test.setTimeout(60_000);
  const itemId = await itemIdByTitle(owner, H264_TITLE);
  await useFreshProfile(page, owner, 'E2E Stopp', { watchLimitMinutes: 1 });
  // This test covers the answer to the progress event. The 10 s title is fully loaded at once, so
  // its stream asks nothing more and no 401 can come; the stop watch (concept 13) would tell the
  // player within 4 s and is switched off here. The 401 of a media request is covered by
  // admin-streams.spec.ts (partly buffered, without the stop watch).
  await page.route(/\/api\/v1\/playback\/sessions\/[^/]+\/state$/, (route) => route.abort());
  await play(page, itemId);

  // Another device of the profile used up the minute; the player learns it from the answer to
  // its next progress event (every 10 s, at the latest on pause).
  const answer = await watchOneMinute(page.request, itemId);
  expect(answer.watchTime?.remainingMinutes).toBe(0);
  await page.locator('video').evaluate((video: HTMLVideoElement) => {
    video.loop = true;
  });
  await expect(player(page)).toHaveAttribute('data-ended-reason', 'limit', { timeout: 15_000 });
  await expect(page.getByRole('alert')).toContainText('Schauzeit für heute aufgebraucht');
  await expect
    .poll(() => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
    .toBe(true);
  // The hint's own back button takes the focus; Escape leaves the player.
  await expectFocusOn(page, page.getByRole('alert').getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`/items/${itemId}$`));
});
