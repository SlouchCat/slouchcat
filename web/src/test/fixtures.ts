import type { HomePage, ItemDetail, ItemRef, Library, PlaybackInfo } from '@slouchcat/api-client';

// Contract-shaped fixtures (api/openapi.yaml). IDs are valid ULIDs.
export const ids = {
  heat: '01JB8Z6Q4M7N9P2R3S5T7V9W1X',
  alien: '01JB8Z6Q4M7N9P2R3S5T7V9W2Y',
  arrival: '01JB8Z6Q4M7N9P2R3S5T7V9W3Z',
} as const;

const images = (id: string) => ({
  poster: { url: `/items/${id}/images/poster/w500?v=abc`, width: 500, height: 750 },
  backdrop: { url: `/items/${id}/images/backdrop/w1280?v=def`, width: 1280, height: 720 },
});

export const heat: ItemRef = {
  id: ids.heat,
  kind: 'movie',
  title: 'Heat',
  year: 1995,
  runtimeMinutes: 170,
  rating: 16,
  images: images(ids.heat),
};

export const alien: ItemRef = {
  id: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  runtimeMinutes: 117,
  rating: 16,
  images: images(ids.alien),
  progress: {
    positionMs: 2_533_000,
    durationMs: 7_020_000,
    percent: 36,
    completed: false,
    updatedAt: '2026-10-18T19:00:00Z',
  },
};

export const arrival: ItemRef = {
  id: ids.arrival,
  kind: 'movie',
  title: 'Arrival',
  year: 2016,
  runtimeMinutes: 116,
  images: {},
};

export const homePage: HomePage = {
  rows: [
    { key: 'picks', items: [heat, arrival] },
    { key: 'continue', items: [alien] },
    { key: 'recent', items: [arrival, heat] },
  ],
};

export const alienDetail: ItemDetail = {
  ...alien,
  originalTitle: 'Alien',
  tagline: 'Im Weltraum hört dich niemand schreien.',
  overview: 'Die Besatzung der Nostromo empfängt ein Notsignal.',
  releaseDate: '1979-05-25',
  genres: ['Horror', 'Science-Fiction'],
  mediaFiles: [
    {
      id: 'mf-1',
      container: 'mp4',
      mimeType: 'video/mp4; codecs="avc1.640028, mp4a.40.2"',
      videoCodec: 'h264',
      audioCodec: 'aac',
      width: 1920,
      height: 1080,
      durationMs: 7_020_000,
      sizeBytes: 4_200_000_000,
      versionLabel: '1080p',
      available: true,
    },
  ],
};

export const playback: PlaybackInfo = {
  mode: 'direct',
  item: alien,
  mediaFile: alienDetail.mediaFiles[0]!,
  streamUrl: `/items/${ids.alien}/stream?file=mf-1`,
  resumePositionMs: 2_533_000,
};

export const library = (overrides: Partial<Library> = {}): Library => ({
  id: 1,
  name: 'Filme',
  kind: 'movies',
  createdAt: '2026-10-18T18:00:00Z',
  roots: [{ id: 1, path: '/media/Filme', fsType: 'ext4', watch: 'inotify' }],
  scan: { state: 'done', itemCount: 12, fileCount: 14, finishedAt: '2026-10-18T18:05:00Z' },
  ...overrides,
});
