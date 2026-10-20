import { createProfile } from './accounts';
import { expect, test } from './fixtures';
import { expectFocusOn } from './keyboard';
import {
  createDownload,
  freshDevice,
  offlinePlayer,
  openOfflineLibrary,
  prepareOffline,
  removeDeviceDownloads,
  storedDownloads,
  syncNow,
  waitForComplete,
} from './offline';
import { episodeIds, INTRO_SHOW_TITLE, itemIdByTitle } from './playback';

// AN-18 (K-5 offline): a profile with a daily limit of 1 minute plays a download in flight mode;
// the device subtracts the played time by its own clock and stops at 0 with the hint of the online
// player (±30 s). Back online the events go up and the server books the minute (the watch-time
// report shows it).

test('a 1-min limit stops offline playback after a minute and is booked later', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  test.setTimeout(360_000);
  const profile = await createProfile(owner, {
    name: 'E2E Offline-Limit',
    maxRating: 18,
    limits: { watchLimitMinutes: 1 },
  });
  const seriesId = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
  const [episode] = await episodeIds(owner, seriesId);
  const { context, page } = await freshDevice(browser, {
    baseURL,
    extraHTTPHeaders,
    profileId: profile.id,
  });
  try {
    await page.goto('/downloads');
    await createDownload(page, episode);
    const [download] = await waitForComplete(page);
    await prepareOffline(page);

    await context.setOffline(true);
    await openOfflineLibrary(page);
    await page.getByRole('button', { name: /E2E Offline-Limit/ }).click();
    const card = page.getByRole('list', { name: 'Heruntergeladene Titel' }).getByRole('link');
    await expectFocusOn(page, card);
    await page.keyboard.press('Enter');
    const player = offlinePlayer(page);
    await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
    const started = Date.now();
    await expect(player).toHaveAttribute('data-ended-reason', 'limit', { timeout: 120_000 });
    const playedSeconds = (Date.now() - started) / 1000;
    expect(playedSeconds, `stopped after ${playedSeconds} s`).toBeGreaterThanOrEqual(30);
    expect(playedSeconds).toBeLessThanOrEqual(90);
    await expect(page.getByRole('alert')).toContainText('Schauzeit für heute aufgebraucht');
    await expect(page.getByRole('alert')).toContainText(
      'Dieses Profil darf 1 Minute pro Tag schauen.',
    );
    await expect
      .poll(() => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
      .toBe(true);
    // The hint's own back button has the focus; Escape leaves the player.
    await expectFocusOn(page, page.getByRole('alert').getByRole('button', { name: 'Zurück' }));
    await page.keyboard.press('Escape');
    await expect(page).toHaveURL(/\/offline$/);

    // Opened again the same day: the hint right away, no playback.
    await card.focus();
    await page.keyboard.press('Enter');
    await expect(player).toHaveAttribute('data-ended-reason', 'limit');
    await expect(player.getByRole('button', { name: 'Abspielen' })).toBeDisabled();
    await page.keyboard.press('Escape');

    // Back online: the server books the offline minute (the report shows the overrun, AN-18).
    await context.setOffline(false);
    await syncNow(page);
    await expect
      .poll(
        async () => {
          await syncNow(page);
          const report = (await (
            await owner.get(`/api/v1/profiles/${profile.id}/watch-time?days=1`)
          ).json()) as { days: { minutes: number }[] };
          return report.days.at(-1)?.minutes ?? 0;
        },
        { timeout: 60_000, intervals: [2000] },
      )
      .toBeGreaterThanOrEqual(1);
    expect((await storedDownloads(page)).map((entry) => entry.id)).toEqual([download.id]);
  } finally {
    await removeDeviceDownloads(context.request).catch(() => undefined);
    await context.close();
  }
});
