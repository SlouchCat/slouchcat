import { expect, type APIRequestContext, type Locator, type Page } from '@playwright/test';
import { createProfile, loginOwner, type ProfileData } from './accounts';
import { CSRF } from './catalog';

// Helpers of the M4 playback specs (tracks, quality, limits, chapters, admin streams).

// M4 test media (testdata/generate-media.sh, streams A and C of M4).
/** H.264 + AAC de (default) + AC3 en + SRT de/en (W-4). */
export const MULTI_AUDIO_TITLE = 'Slouchcat Test Multi Audio';
/** H.264/AAC + a PGS bitmap subtitle: burned in everywhere (10.3). */
export const PGS_TITLE = 'Slouchcat Test PGS';
/** 10 s with three chapters at 0, 3 and 6.5 s ("Kapitel 1: Anfang" …). */
export const CHAPTERS_TITLE = 'Slouchcat Test Chapters';
/** Folder of CHAPTERS_TITLE below testdata/media/Movies, for the sidecar test (I-5). */
export const CHAPTERS_FOLDER = 'Slouchcat Test Chapters (2024)';
/** Series with the same 20-s intro and outro theme in S01E01–E05 (W-9). */
export const INTRO_SHOW_TITLE = 'Slouchcat Test Intro Show';
/** HEVC 10-bit HDR10 2160p: transcode with tonemapping in the embedded browsers (10.3). */
export const HDR_TITLE = 'Slouchcat Test HDR10 4K';
/** H.264 + DTS 5.1 and H.264 + AC3 5.1: audio transcode (10.3). */
export const DTS_TITLE = 'Slouchcat Test DTS';
export const AC3_TITLE = 'Slouchcat Test AC3 51';
/** The plain H.264/AAC MP4 (direct play). */
export { H264_TITLE } from './catalog';

/** The parts of PlaybackInfo the specs read. */
export interface PlaybackData {
  mode: 'direct' | 'hls';
  playbackSessionId: string;
  streamUrl: string;
  item: { id: string; title: string };
  mediaFile: { id: string };
  transcoding?: {
    mode: 'remux' | 'transcode';
    reason: string;
    hardware: boolean;
    backend?: string;
    quality?: string;
    width?: number;
    height?: number;
    bitrate?: number;
    tonemap?: boolean;
  };
  tracks: { audio: TrackData[]; subtitles: TrackData[] };
  selection: { audioTrack?: number; subtitleTrack?: number; subtitleDelivery?: 'text' | 'burn' };
  subtitleUrl?: string;
  qualities: { quality: string; available: boolean; current: boolean; height?: number }[];
  chapters: { startMs: number; endMs: number; title?: string }[];
  segments: {
    intro?: { startMs: number; endMs: number };
    outro?: { startMs: number; endMs: number };
  };
  nextEpisode?: { id: string; title: string };
}

export interface TrackData {
  id: number;
  index: number;
  kind: 'audio' | 'subtitle';
  codec: string;
  language?: string;
  title?: string;
  text: boolean;
  source: 'embedded' | 'sidecar';
}

export const player = (page: Page): Locator => page.getByRole('main', { name: 'Videoplayer' });

const isStart = (url: string, method: string) =>
  method === 'POST' && /\/api\/v1\/items\/[^/]+\/play$/.test(url);

/** What the client sent with startPlayback (PlaybackRequest). */
export interface PlaybackRequestData {
  mediaFileId?: string;
  audioTrack?: number;
  subtitleTrack?: number | null;
  subtitleDelivery?: string;
  quality?: string;
  positionMs?: number;
  capabilities: {
    containers?: string[];
    video: string[];
    audio: string[];
    hls: string;
    maxWidth?: number;
    maxHeight?: number;
    hdr?: boolean;
    name?: string;
    type?: string;
  };
}

/**
 * Runs `action` and returns the startPlayback it causes: the request body, the status and the
 * answer (PlaybackInfo, or a problem with `code` and `extensions`).
 */
export async function startOf(
  page: Page,
  action: () => Promise<unknown>,
): Promise<{
  request: PlaybackRequestData;
  status: number;
  body: PlaybackData & { code?: string; extensions?: Record<string, number> };
}> {
  const response = page.waitForResponse((r) => isStart(r.url(), r.request().method()));
  await action();
  const answer = await response;
  return {
    request: answer.request().postDataJSON() as PlaybackRequestData,
    status: answer.status(),
    body: (await answer.json()) as PlaybackData & {
      code?: string;
      extensions?: Record<string, number>;
    },
  };
}

/** Presses ArrowDown (or ArrowUp) until `target` has the focus; fails after `max` presses. */
export async function arrowTo(
  page: Page,
  target: Locator,
  key: 'ArrowDown' | 'ArrowUp' = 'ArrowDown',
  max = 12,
): Promise<void> {
  for (let presses = 0; presses <= max; presses += 1) {
    if (await target.evaluate((element) => element === document.activeElement)) return;
    await page.keyboard.press(key);
  }
  throw new Error(`${key} did not reach the element within ${max} presses`);
}

/** Opens the player of `itemId` and returns the started playback; waits until it plays. */
export async function play(page: Page, itemId: string, search = ''): Promise<PlaybackData> {
  const { status, body } = await startOf(page, () => page.goto(`/play/${itemId}${search}`));
  expect(status, JSON.stringify(body)).toBe(200);
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
  return body;
}

/**
 * Id of a film or series by its title. Without TMDB every title stays unmatched, so the
 * maintenance list of the owner names them all (the start page row stops at 20 titles).
 */
export async function itemIdByTitle(owner: APIRequestContext, title: string): Promise<string> {
  const response = await owner.get('/api/v1/admin/maintenance/issues?kind=unmatched&limit=200');
  expect(response.status(), await response.text()).toBe(200);
  const { items } = (await response.json()) as { items: { itemId: string; title: string }[] };
  const item = items.find((candidate) => candidate.title === title);
  expect(item, `title "${title}" in the library`).toBeDefined();
  return item!.itemId;
}

/** Ids of the episodes of the first season of a series, in order. */
export async function episodeIds(api: APIRequestContext, seriesId: string): Promise<string[]> {
  const series = (await (await api.get(`/api/v1/items/${seriesId}`)).json()) as {
    seasons?: { id: string; seasonNumber?: number }[];
  };
  const season = series.seasons?.find((candidate) => candidate.seasonNumber === 1);
  expect(season, 'season 1').toBeDefined();
  const detail = (await (await api.get(`/api/v1/items/${season!.id}`)).json()) as {
    episodes?: { id: string }[];
  };
  return (detail.episodes ?? []).map((episode) => episode.id);
}

/** Sets the profile's position of a title back to 0, so the player starts at the beginning. */
export async function resetProgress(
  api: APIRequestContext,
  itemId: string,
  durationMs = 60_000,
): Promise<void> {
  const response = await api.post(`/api/v1/items/${itemId}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 0,
      durationMs,
      playedMs: 0,
      seq: 0,
      state: 'paused',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

/**
 * A profile of its own for a spec that sets limits (W-8, K-5): created by the owner (the `owner`
 * fixture deletes it after the test) and selected in a fresh session of `page`'s context.
 */
export async function useFreshProfile(
  page: Page,
  owner: APIRequestContext,
  name: string,
  limits: Record<string, number> = {},
): Promise<ProfileData> {
  const profile = await createProfile(owner, { name, maxRating: 18, limits });
  await loginOwner(page.request);
  const selected = await page.request.post(`/api/v1/auth/profiles/${profile.id}/select`, {
    headers: CSRF,
  });
  expect(selected.status(), await selected.text()).toBe(200);
  return profile;
}

/** Replaces the limits of a profile (the object replaces all of them). */
export async function setLimits(
  owner: APIRequestContext,
  profileId: number,
  limits: Record<string, number>,
): Promise<void> {
  const response = await owner.patch(`/api/v1/profiles/${profileId}`, {
    headers: CSRF,
    data: { limits },
  });
  expect(response.status(), await response.text()).toBe(200);
}

/** State of the video element. */
export const videoState = (page: Page) =>
  page.locator('video').evaluate((video: HTMLVideoElement) => ({
    time: video.currentTime,
    duration: video.duration,
    paused: video.paused,
    tracks: [...video.textTracks].map((track) => ({ kind: track.kind, mode: track.mode })),
  }));
