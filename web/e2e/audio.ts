import {
  expect,
  type APIRequestContext,
  type Locator,
  type Page,
  type TestInfo,
} from '@playwright/test';
import { CSRF } from './catalog';
import { FEED_BASE } from './env';

// Helpers of the M6 specs (audiobooks, podcasts, audio downloads). Test media from
// testdata/generate-media.sh: the M4B book (60 s, 12 chapters of 5 s, cover embedded) and the MP3
// book (10 files of 6 s, cover.jpg) by the same author, narrator and series; the podcast of
// testdata/podcasts/feed.xml.tmpl served by feed-server.mjs.

export const M4B_TITLE = 'Slouchcat Test Audiobook M4B';
export const MP3_TITLE = 'Slouchcat Test Audiobook MP3';
export const AUDIO_AUTHOR = 'Slouchcat Test Author';
export const AUDIO_NARRATOR = 'Slouchcat Test Narrator';
export const AUDIO_SERIES = 'Slouchcat Test Audio Series';
export const PODCAST_TITLE = 'Slouchcat Test Podcast';
/** Episode n of the test feed; the third one is flagged explicit (FSK 18, AN-2). */
export const episodeTitle = (n: 1 | 2 | 3) =>
  n === 3 ? 'Slouchcat Test Episode 3 (explicit)' : `Slouchcat Test Episode ${n}`;
/** Scans probe the audio files after the walk: chapters and durations come a little later. */
export const AUDIO_READY_MS = 60_000;

/** The audio player (landmark "Audioplayer"). */
export const audioPlayer = (page: Page): Locator => page.getByRole('main', { name: 'Audioplayer' });

/** State of the <audio> element. */
export const audioState = (page: Page) =>
  page.locator('audio').evaluate((audio: HTMLAudioElement) => ({
    time: audio.currentTime,
    paused: audio.paused,
    rate: audio.playbackRate,
    src: audio.currentSrc,
  }));

export interface ChapterData {
  startMs: number;
  endMs: number;
  title?: string;
}

export interface AudioItemData {
  id: string;
  kind: string;
  title: string;
  author?: string;
  narrator?: string;
  seriesName?: string;
  seriesIndex?: number;
  durationMs?: number;
  chapters?: ChapterData[];
  episodes?: { id: string; title: string }[];
  episodeCount?: number;
  progress?: { positionMs: number; playbackRate?: number };
}

/**
 * Id of an audiobook (or podcast) by its title. Audio titles are matched from their own tags
 * (never "unmatched"), so they are looked up on the start page's rows; the global setup adds them
 * after the video libraries, so they are among the newest titles.
 */
export async function audioItemId(api: APIRequestContext, title: string): Promise<string> {
  let found: string | undefined;
  await expect
    .poll(
      async () => {
        const home = (await (await api.get('/api/v1/home')).json()) as {
          rows: { key: string; items: { id: string; title: string }[] }[];
        };
        found = home.rows.flatMap((row) => row.items).find((item) => item.title === title)?.id;
        return found;
      },
      { message: `"${title}" on the start page`, timeout: AUDIO_READY_MS, intervals: [1000] },
    )
    .toBeDefined();
  return found!;
}

/** The detail of a title once its probes are through (chapters of a book, `count` of them). */
export async function audioDetail(
  api: APIRequestContext,
  id: string,
  chapters: number,
): Promise<AudioItemData> {
  let detail: AudioItemData | undefined;
  await expect
    .poll(
      async () => {
        detail = (await (await api.get(`/api/v1/items/${id}`)).json()) as AudioItemData;
        return detail.chapters?.length ?? 0;
      },
      { message: `${chapters} chapters of ${id}`, timeout: AUDIO_READY_MS, intervals: [1000] },
    )
    .toBe(chapters);
  return detail!;
}

/** Sets the profile's position and speed of a title (an online progress event). */
export async function setAudioProgress(
  api: APIRequestContext,
  itemId: string,
  positionMs: number,
  durationMs: number,
  playbackRate = 1,
): Promise<void> {
  const response = await api.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs,
      durationMs,
      playedMs: 0,
      seq: 0,
      state: 'paused',
      clientTime: new Date().toISOString(),
      playbackRate,
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

/** Opens the player of `itemId` and waits until it plays; returns the started playback. */
export async function playAudio(
  page: Page,
  itemId: string,
  search = '',
): Promise<{ mediaKind: string; streamUrl: string; chapters: ChapterData[] }> {
  const start = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      new URL(response.url()).pathname === `/api/v1/items/${itemId}/play`,
  );
  await page.goto(`/play/${itemId}${search}`);
  const answer = await start;
  const body = (await answer.json()) as {
    mediaKind: string;
    streamUrl: string;
    chapters: ChapterData[];
  };
  expect(answer.status(), JSON.stringify(body)).toBe(200);
  expect(body.mediaKind).toBe('audio');
  await expect(audioPlayer(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
  return body;
}

/** A feed of its own for one test and browser (feed-server.mjs `/f/<key>/`). */
export function feedKey(testInfo: TestInfo): string {
  const slug = testInfo.title.replace(/[^A-Za-z0-9]+/g, '-').slice(0, 40);
  return `${testInfo.project.name}-${slug}-${Date.now().toString(36)}`;
}

export const feedUrl = (key: string) => `${FEED_BASE}/f/${key}/feed.xml`;

/** How many episodes the feed `key` lists from now on (1 … 3). */
export async function feedEpisodes(key: string, episodes: number): Promise<void> {
  const response = await fetch(`${FEED_BASE}/__control`, {
    method: 'POST',
    body: JSON.stringify({ feed: key, episodes }),
  });
  expect(response.status, 'feed server control').toBe(204);
}

/** Id of the library "Podcasts" (global setup). */
export async function podcastLibraryId(api: APIRequestContext): Promise<number> {
  const libraries = (await (await api.get('/api/v1/admin/libraries')).json()) as {
    id: number;
    name: string;
    kind: string;
  }[];
  const library = libraries.find((entry) => entry.kind === 'podcasts' && entry.name === 'Podcasts');
  expect(library, 'the podcast library of the global setup').toBeDefined();
  return library!.id;
}

export interface AdminPodcastData {
  id: string;
  title: string;
  feedUrl: string;
  episodeCount: number;
  lastCheckedAt?: string;
  lastError?: string;
}

/** Subscribes the library "Podcasts" to the feed `key` with `episodes` episodes (synchronous). */
export async function subscribeFeed(
  owner: APIRequestContext,
  key: string,
  episodes = 2,
): Promise<AdminPodcastData> {
  await feedEpisodes(key, episodes);
  const response = await owner.post('/api/v1/admin/podcasts', {
    headers: CSRF,
    data: { libraryId: await podcastLibraryId(owner), feedUrl: feedUrl(key) },
    timeout: 30_000,
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()) as AdminPodcastData;
}

/** Removes a subscription (and its episodes) again; tolerates one that is gone already. */
export async function unsubscribe(owner: APIRequestContext, podcastId: string): Promise<void> {
  const response = await owner.delete(`/api/v1/admin/podcasts/${podcastId}`, { headers: CSRF });
  expect([204, 404]).toContain(response.status());
}

/** "Jetzt abrufen" through the API. */
export async function fetchNow(owner: APIRequestContext, podcastId: string): Promise<void> {
  const response = await owner.post(`/api/v1/admin/podcasts/${podcastId}/refresh`, {
    headers: CSRF,
  });
  expect(response.status(), await response.text()).toBe(202);
}

/** The episodes the request context's profile sees, newest first (listPodcastEpisodes). */
export async function visibleEpisodes(
  api: APIRequestContext,
  podcastId: string,
): Promise<{ total: number; episodes: { id: string; title: string }[] }> {
  const response = await api.get(`/api/v1/items/${podcastId}/episodes?limit=200`);
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as { total: number; episodes: { id: string; title: string }[] };
}

/** Waits until the profile of `api` sees `count` episodes of the podcast. */
export async function waitForEpisodes(
  api: APIRequestContext,
  podcastId: string,
  count: number,
): Promise<{ id: string; title: string }[]> {
  let episodes: { id: string; title: string }[] = [];
  await expect
    .poll(
      async () => {
        ({ episodes } = await visibleEpisodes(api, podcastId));
        return episodes.length;
      },
      { message: `${count} episodes of ${podcastId}`, timeout: 30_000, intervals: [1000] },
    )
    .toBe(count);
  return episodes;
}
