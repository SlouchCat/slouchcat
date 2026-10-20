import { NO_LOCK } from '@/features/offline/pin';
import type {
  DownloadManifest,
  DownloadOptions,
  DownloadRules,
  DownloadState,
  OfflineProfile,
  StoredDownload,
  StoredProfile,
} from '@/features/offline/types';
import { alien, alienTracks, episodes, heat, ids, seriesIds } from './fixtures';

// Contract-shaped offline fixtures (DownloadState, DownloadManifest, OfflineProfile) and the
// records the device stores of them. Download ids are valid ULIDs.

export const downloadIds = {
  alien: '01JB8Z6Q4M7N9P2R3S5T7V9D01',
  heat: '01JB8Z6Q4M7N9P2R3S5T7V9D02',
  e1: '01JB8Z6Q4M7N9P2R3S5T7V9D03',
  e2: '01JB8Z6Q4M7N9P2R3S5T7V9D04',
} as const;

/** Two 4-s segments after a 1000-byte init segment; 9000 bytes in all. */
export const manifest = (
  id: string = downloadIds.alien,
  itemId: string = ids.alien,
): DownloadManifest => ({
  id,
  itemId,
  sizeBytes: 9000,
  durationMs: 8000,
  mimeType: 'video/mp4',
  codecs: 'avc1.64001f,mp4a.40.2',
  width: 1280,
  height: 720,
  initLength: 1000,
  segments: [
    { startMs: 0, durationMs: 4000, offset: 1000, length: 4000 },
    { startMs: 4000, durationMs: 4000, offset: 5000, length: 4000 },
  ],
  fileUrl: `/downloads/${id}/file?dt=${id}.1.token`,
  audio: alienTracks.audio[0],
  subtitles: [
    {
      track: alienTracks.subtitles[0]!,
      url: `/downloads/${id}/subtitles/21.vtt?dt=${id}.1.token`,
    },
  ],
});

export const downloadState = (overrides: Partial<DownloadState> = {}): DownloadState => ({
  id: downloadIds.alien,
  state: 'ready',
  quality: 'medium',
  auto: false,
  allowedProfileIds: [1],
  estimatedBytes: 9000,
  sizeBytes: 9000,
  segmentsDone: 2,
  segmentsTotal: 2,
  createdAt: '2026-10-20T10:00:00Z',
  readyAt: '2026-10-20T10:05:00Z',
  item: alien,
  ...overrides,
});

export const storedDownload = (overrides: Partial<StoredDownload> = {}): StoredDownload => ({
  id: downloadIds.alien,
  accountId: 1,
  state: 'ready',
  quality: 'medium',
  auto: false,
  allowedProfileIds: [1],
  estimatedBytes: 9000,
  sizeBytes: 9000,
  segmentsDone: 2,
  segmentsTotal: 2,
  createdAt: '2026-10-20T10:00:00Z',
  readyAt: '2026-10-20T10:05:00Z',
  item: alien,
  manifest: manifest(),
  bytesWritten: 9000,
  subtitlesStored: [21],
  posterStored: true,
  complete: true,
  updatedAt: Date.parse('2026-10-20T10:10:00Z'),
  ...overrides,
});

/** A complete download of Heat that only profile 2 (FSK 18) may see. */
export const heatDownload = (overrides: Partial<StoredDownload> = {}) =>
  storedDownload({
    id: downloadIds.heat,
    item: heat,
    manifest: manifest(downloadIds.heat, ids.heat),
    allowedProfileIds: [2],
    createdAt: '2026-10-20T09:00:00Z',
    ...overrides,
  });

/** Episode downloads of season 1 (O-2). */
export const episodeDownload = (key: 'e1' | 'e2', overrides: Partial<StoredDownload> = {}) =>
  storedDownload({
    id: downloadIds[key],
    item: episodes[key],
    manifest: manifest(downloadIds[key], seriesIds[key]),
    ...overrides,
  });

export const offlineProfile = (overrides: Partial<OfflineProfile> = {}): OfflineProfile => ({
  id: 1,
  name: 'Marco',
  language: 'de',
  isPrimary: true,
  hasPin: false,
  ...overrides,
});

export const storedProfile = (overrides: Partial<StoredProfile> = {}): StoredProfile => ({
  id: overrides.snapshot?.id ?? 1,
  accountId: 1,
  snapshot: offlineProfile(),
  syncedAt: Date.parse('2026-10-20T10:00:00Z'),
  lock: NO_LOCK,
  ...overrides,
});

export const rules: DownloadRules = {
  enabled: false,
  nextEpisodes: true,
  newPodcastEpisodes: false,
  wifiOnly: true,
  maxCount: 3,
  quality: 'medium',
};

export const downloadOptions: DownloadOptions = {
  mediaFileId: 'mf-1',
  durationMs: 7_020_000,
  qualities: [
    {
      quality: 'high',
      width: 1920,
      height: 1080,
      videoBitrate: 6000,
      audioBitrate: 160,
      estimatedBytes: 5_480_000_000,
      copy: false,
    },
    {
      quality: 'medium',
      width: 1280,
      height: 720,
      videoBitrate: 3000,
      audioBitrate: 128,
      estimatedBytes: 2_800_000_000,
      copy: false,
    },
    {
      quality: 'low',
      width: 854,
      height: 480,
      videoBitrate: 1200,
      audioBitrate: 96,
      estimatedBytes: 1_140_000_000,
      copy: false,
    },
  ],
  audioTracks: alienTracks.audio,
  subtitles: [alienTracks.subtitles[0]!],
};
