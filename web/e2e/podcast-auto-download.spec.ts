import { createProfile } from './accounts';
import {
  episodeTitle,
  feedEpisodes,
  feedKey,
  fetchNow,
  subscribeFeed,
  unsubscribe,
  waitForEpisodes,
} from './audio';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectNoFocusTrap, expectVisibleFocus, resetFocus, tabTo } from './keyboard';
import {
  downloadRow,
  freshDevice,
  removeDeviceDownloads,
  storedDownloads,
  syncNow,
  waitForComplete,
} from './offline';

// O-3 for podcasts (M6): with "Neue Episoden gehörter Podcasts" on, a podcast the profile listened
// to (progress on episode 2) gets its new episode 3 downloaded at the next sync after the feed was
// fetched (auto). The profile is FSK 18: episode 3 is explicit (AN-2).

test('a new episode of a podcast the profile listens to is downloaded by itself (O-3)', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}, testInfo) => {
  test.setTimeout(240_000);
  const profile = await createProfile(owner, { name: 'E2E Podcast Auto', maxRating: 18 });
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  const { context, page } = await freshDevice(browser, {
    baseURL,
    extraHTTPHeaders,
    profileId: profile.id,
  });
  try {
    // The rule by keyboard: Space toggles the checkboxes, Tab moves, Enter saves.
    await page.goto('/downloads');
    const rules = page.getByRole('region', { name: 'Automatische Downloads' });
    const enabled = rules.getByRole('checkbox', {
      name: 'Automatische Downloads auf diesem Gerät',
    });
    const podcasts = rules.getByRole('checkbox', { name: 'Neue Episoden gehörter Podcasts' });
    await expect(enabled).not.toBeChecked();
    await expect(podcasts).not.toBeChecked();
    await resetFocus(page);
    await tabTo(page, enabled, 40);
    await page.keyboard.press('Space');
    await expect(enabled).toBeChecked();
    await tabTo(page, podcasts, 3);
    await page.keyboard.press('Space');
    await expect(podcasts).toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, rules.getByRole('button', { name: 'Regel speichern' }), 8);
    await page.keyboard.press('Enter');
    await expect(rules.getByText(/Gespeichert/)).toBeVisible();
    const stored = (await (await page.request.get('/api/v1/offline/rules')).json()) as {
      enabled: boolean;
      newPodcastEpisodes: boolean;
    };
    expect(stored).toMatchObject({ enabled: true, newPodcastEpisodes: true });
    await expectNoFocusTrap(page);

    // The profile listened to episode 2 (the newest so far).
    const [second, first] = await waitForEpisodes(page.request, podcast.id, 2);
    expect(second.title).toBe(episodeTitle(2));
    expect(first.title).toBe(episodeTitle(1));
    const listened = await page.request.post(`/api/v1/items/${second.id}/progress`, {
      headers: CSRF,
      data: {
        positionMs: 3000,
        durationMs: 10_000,
        playedMs: 3000,
        seq: 1,
        state: 'paused',
        clientTime: new Date().toISOString(),
      },
    });
    expect(listened.ok(), await listened.text()).toBe(true);
    // Nothing new yet: no auto-download.
    await syncNow(page);
    expect((await storedDownloads(page)).filter((download) => download.auto)).toHaveLength(0);

    // The feed gets episode 3, the server fetches it, the next sync downloads it.
    await feedEpisodes(key, 3);
    await fetchNow(owner, podcast.id);
    const [newest] = await waitForEpisodes(page.request, podcast.id, 3);
    expect(newest.title).toBe(episodeTitle(3));
    const [download] = await waitForComplete(
      page,
      1,
      (entry) => entry.auto && entry.item?.id === newest.id,
    );
    expect(download.auto).toBe(true);
    // Episode 1 is older than the one listened to: not downloaded.
    expect((await storedDownloads(page)).some((entry) => entry.item?.id === first.id)).toBe(false);
    await page.reload();
    const row = downloadRow(page, episodeTitle(3));
    await expect(row.getByText('Automatisch')).toBeVisible();
    await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
  } finally {
    await removeDeviceDownloads(context.request).catch(() => undefined);
    await context.close();
    await unsubscribe(owner, podcast.id);
  }
});
