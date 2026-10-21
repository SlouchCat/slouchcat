import { createProfile, loginOwner } from './accounts';
import {
  audioPlayer,
  audioState,
  episodeTitle,
  feedEpisodes,
  feedKey,
  feedUrl,
  PODCAST_TITLE,
  playAudio,
  subscribeFeed,
  unsubscribe,
  visibleEpisodes,
  waitForEpisodes,
} from './audio';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// H-2: a podcast subscribed by feed URL shows its episodes; the feed gets a third episode, the
// admin's "Jetzt abrufen" fetches it, and it appears. The third episode is flagged explicit: a
// profile with FSK 12 sees it neither in the list nor by id (404). An episode plays through the
// server (the enclosure proxy), never from the feed's host. Every test subscribes a feed of its
// own (feed-server.mjs /f/<key>/), so the specs do not depend on each other's order.

test('H-2: the new episode appears after "Jetzt abrufen"; explicit stays hidden from FSK 12', async ({
  page,
  owner,
  browser,
  baseURL,
  extraHTTPHeaders,
}, testInfo) => {
  test.setTimeout(120_000);
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  try {
    expect(podcast).toMatchObject({ title: PODCAST_TITLE, feedUrl: feedUrl(key), episodeCount: 2 });
    await page.goto(`/items/${podcast.id}`);
    await expect(page.getByRole('heading', { level: 1, name: PODCAST_TITLE })).toBeVisible();
    // The container has no play button of its own; every episode has one.
    await expect(page.getByRole('link', { name: /^(Abspielen|Weiterhören)/ })).toHaveCount(0);
    const list = page.getByRole('list', { name: 'Episoden' });
    const plays = list.getByRole('link', { name: / abspielen$/ });
    await expect(plays).toHaveCount(2);
    // Newest first, with date and duration.
    await expect(plays.first()).toHaveAccessibleName(`${episodeTitle(2)} abspielen`);
    await expect(list.getByText('13.10.2025 · 10 s')).toBeVisible();
    await expect(list.getByText('06.10.2025 · 8 s')).toBeVisible();

    // The feed gets episode 3; the admin fetches it now (keyboard on /admin/podcasts).
    await feedEpisodes(key, 3);
    await page.goto('/admin/podcasts');
    const row = page
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader').filter({ hasText: feedUrl(key) }) });
    await expect(row).toBeVisible();
    const fetchButton = row.getByRole('button', { name: `${PODCAST_TITLE} jetzt abrufen` });
    await tabTo(page, fetchButton, 60);
    await page.keyboard.press('Enter');
    await expect(page.getByText(`${PODCAST_TITLE} wird abgerufen.`)).toBeVisible();
    const episodes = await waitForEpisodes(owner, podcast.id, 3);
    expect(episodes.map((episode) => episode.title)).toEqual([
      episodeTitle(3),
      episodeTitle(2),
      episodeTitle(1),
    ]);
    await expect(row.getByRole('cell').filter({ hasText: /^3$/ })).toBeVisible({
      timeout: 15_000,
    });

    await page.goto(`/items/${podcast.id}`);
    await expect(plays).toHaveCount(3);
    await expect(plays.first()).toHaveAccessibleName(`${episodeTitle(3)} abspielen`);
    const explicitId = episodes[0].id;

    // A profile with FSK 12: the explicit episode is in neither the list nor reachable by id.
    const profile = await createProfile(owner, { name: 'E2E Zwoelf', maxRating: 12 });
    const kid = await browser.newContext({
      baseURL,
      locale: 'de-CH',
      extraHTTPHeaders,
      storageState: { cookies: [], origins: [] },
    });
    try {
      await loginOwner(kid.request);
      const selected = await kid.request.post(`/api/v1/auth/profiles/${profile.id}/select`, {
        headers: CSRF,
      });
      expect(selected.status(), await selected.text()).toBe(200);
      const seen = await visibleEpisodes(kid.request, podcast.id);
      expect(seen.total).toBe(2);
      expect(seen.episodes.map((episode) => episode.title)).toEqual([
        episodeTitle(2),
        episodeTitle(1),
      ]);
      expect((await kid.request.get(`/api/v1/items/${explicitId}`)).status()).toBe(404);
      const view = await kid.newPage();
      await view.goto(`/items/${podcast.id}`);
      await expect(
        view.getByRole('list', { name: 'Episoden' }).getByRole('link', { name: / abspielen$/ }),
      ).toHaveCount(2);
      await expect(view.getByText(episodeTitle(3))).toHaveCount(0);
      await view.goto(`/items/${explicitId}`);
      await expect(view.getByText('Titel nicht gefunden')).toBeVisible();
    } finally {
      await kid.close();
    }
  } finally {
    await unsubscribe(owner, podcast.id);
  }
});

test('an episode plays through the server, not from the feed host', async ({
  page,
  owner,
  baseURL,
}, testInfo) => {
  test.setTimeout(60_000);
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  try {
    const [newest] = await waitForEpisodes(owner, podcast.id, 2);
    const info = await playAudio(page, newest.id);
    expect(info.streamUrl).toMatch(/^\/(api\/v1\/)?items\/[^/]+\/stream\?st=/);
    await expect
      .poll(async () => (await audioState(page)).time, { timeout: 15_000 })
      .toBeGreaterThan(0.5);
    const { src } = await audioState(page);
    expect(src.startsWith(`${baseURL}/api/v1/items/`), src).toBe(true);
    const player = audioPlayer(page);
    await expect(player.getByRole('heading', { name: episodeTitle(2) })).toBeVisible();
    await expect(player.getByText(PODCAST_TITLE)).toBeVisible();
    // No chapters in an episode: no chapter buttons.
    await expect(player.getByRole('button', { name: 'Nächstes Kapitel' })).toHaveCount(0);
  } finally {
    await unsubscribe(owner, podcast.id);
  }
});

test('podcast page by keyboard: episodes, play, "Mehr laden" absent, no focus trap', async ({
  page,
  owner,
}, testInfo) => {
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  try {
    await page.goto(`/items/${podcast.id}`);
    const list = page.getByRole('list', { name: 'Episoden' });
    const firstPlay = list.getByRole('link', { name: `${episodeTitle(2)} abspielen` });
    await tabTo(page, firstPlay, 30);
    // ↓ moves to the next episode's play button (arrow navigation of the detail page).
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, list.getByRole('link', { name: `${episodeTitle(1)} abspielen` }));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(
      page,
      list.getByRole('link', { name: new RegExp(`^${episodeTitle(1)}`) }).first(),
    );
    // Two episodes: everything is there.
    await expect(page.getByRole('button', { name: 'Mehr laden' })).toHaveCount(0);
    await expectNoFocusTrap(page);
    // Enter on an episode opens it; Backspace comes back.
    await list.getByRole('link', { name: `${episodeTitle(2)} abspielen` }).focus();
    await page.keyboard.press('Enter');
    await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL(new RegExp(`/items/${podcast.id}$`));
  } finally {
    await unsubscribe(owner, podcast.id);
  }
});

test('episode page by keyboard: play, download, "Zum Podcast", back; no focus trap', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(60_000);
  const key = feedKey(testInfo);
  const podcast = await subscribeFeed(owner, key, 2);
  try {
    const [newest] = await waitForEpisodes(owner, podcast.id, 2);
    // From the podcast page, so that back leads there.
    await page.goto(`/items/${podcast.id}`);
    await page.goto(`/items/${newest.id}`);
    await expect(page.getByRole('heading', { level: 1, name: episodeTitle(2) })).toBeVisible();
    await resetFocus(page);
    const play = page.getByRole('link', {
      name: /^(Abspielen|Weiterhören ab \d{2}:\d{2}:\d{2})$/,
    });
    const back = page.getByRole('button', { name: 'Zurück' });
    const download = page.getByRole('button', { name: 'Herunterladen', exact: true });
    const toPodcast = page.getByRole('link', { name: 'Zum Podcast' });
    await expect(toPodcast).toHaveAttribute('href', `/items/${podcast.id}`);
    // The download button loads with the offline chunk.
    await expect(download).toBeVisible();

    // Arrows: ↓ lands on play, → back and the download, ← returns, ↓ to "Zum Podcast".
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, play);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, back);
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, download);
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, play);
    // The row under the actions (watchlist, M7) comes first.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, page.getByRole('button', { name: 'Merken' }));
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, toPodcast);

    // Tab from the top: the header, play, back, the download, then "Zum Podcast"; no trap.
    await resetFocus(page);
    await tabTo(page, page.getByRole('button', { name: 'Abmelden' }));
    await tabTo(page, play, 2);
    await tabTo(page, download, 3);
    // "Verwalten", then "Merken" and "Zu Cat's Empfehlungen" (M7).
    await tabTo(page, toPodcast, 6);
    await expectNoFocusTrap(page);

    // Enter on "Zum Podcast" opens the podcast; Backspace returns to the episode.
    await toPodcast.focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/items/${podcast.id}$`));
    await expect(page.getByRole('heading', { level: 1, name: PODCAST_TITLE })).toBeVisible();
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL(new RegExp(`/items/${newest.id}$`));
    // Enter on play starts the episode; Backspace leaves the player for the episode again.
    await play.focus();
    await page.keyboard.press('Enter');
    await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
    await page.keyboard.press('Backspace');
    await expect(page).toHaveURL(new RegExp(`/items/${newest.id}$`));
  } finally {
    await unsubscribe(owner, podcast.id);
  }
});
