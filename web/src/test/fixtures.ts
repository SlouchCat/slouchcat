import type {
  HomePage,
  ItemDetail,
  ItemRef,
  Library,
  LibrarySummary,
  PlaybackInfo,
  Profile,
  ProfileSummary,
  TrackList,
} from '@slouchcat/api-client';

// Contract-shaped fixtures (api/openapi.yaml). IDs are valid ULIDs.
export const ids = {
  heat: '01JB8Z6Q4M7N9P2R3S5T7V9W1X',
  alien: '01JB8Z6Q4M7N9P2R3S5T7V9W2Y',
  arrival: '01JB8Z6Q4M7N9P2R3S5T7V9W3Z',
} as const;

const images = (id: string) => ({
  poster: { url: `/items/${id}/images/poster/w500?v=abc`, width: 500, height: 750 },
  backdrop: { url: `/items/${id}/images/backdrop/w1280?v=def`, width: 1280, height: 720 },
  backdropSmall: { url: `/items/${id}/images/backdrop/w780?v=ghi`, width: 780, height: 439 },
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

/** Tracks of the Alien file: two audio tracks, a text and a bitmap subtitle (W-4). */
export const alienTracks: TrackList = {
  mediaFileId: 'mf-1',
  audio: [
    {
      id: 11,
      index: 1,
      kind: 'audio',
      codec: 'aac',
      language: 'ger',
      default: true,
      forced: false,
      source: 'embedded',
      text: false,
      channels: 2,
    },
    {
      id: 12,
      index: 2,
      kind: 'audio',
      codec: 'ac3',
      language: 'eng',
      default: false,
      forced: false,
      source: 'embedded',
      text: false,
      channels: 6,
    },
  ],
  subtitles: [
    {
      id: 21,
      index: 3,
      kind: 'subtitle',
      codec: 'subrip',
      language: 'ger',
      title: 'Deutsch (SRT)',
      default: false,
      forced: false,
      source: 'embedded',
      text: true,
    },
    {
      id: 22,
      index: 4,
      kind: 'subtitle',
      codec: 'hdmv_pgs_subtitle',
      language: 'eng',
      default: false,
      forced: false,
      source: 'embedded',
      text: false,
    },
  ],
};

export const playback: PlaybackInfo = {
  mode: 'direct',
  mediaKind: 'video',
  item: alien,
  mediaFile: alienDetail.mediaFiles[0]!,
  streamUrl: `/items/${ids.alien}/stream?st=ps-1.token`,
  resumePositionMs: 2_533_000,
  playbackSessionId: 'ps-1',
  tracks: alienTracks,
  selection: { audioTrack: 1 },
  qualities: [
    { quality: 'auto', available: true, current: true },
    {
      quality: '1080p',
      width: 1920,
      height: 1080,
      bitrate: 8_000_000,
      available: true,
      current: false,
    },
    {
      quality: '720p',
      width: 1280,
      height: 720,
      bitrate: 4_000_000,
      available: true,
      current: false,
    },
    {
      quality: '480p',
      width: 854,
      height: 480,
      bitrate: 2_000_000,
      available: true,
      current: false,
    },
    {
      quality: '360p',
      width: 640,
      height: 360,
      bitrate: 1_000_000,
      available: false,
      current: false,
    },
  ],
  chapters: [],
  segments: {},
};

/** The same title as HLS (remux of an MKV), master playlist with the stream token. */
export const hlsPlayback: PlaybackInfo = {
  ...playback,
  mode: 'hls',
  streamUrl: '/api/v1/playback/sessions/ps-2/master.m3u8?st=ps-2.token',
  playbackSessionId: 'ps-2',
  transcoding: { mode: 'remux', reason: 'container', hardware: false },
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

/** Profile summaries as the picker lists them. */
export const summaries: ProfileSummary[] = [
  { id: 1, name: 'Marco', accent: 'teal', hasPin: false, isPrimary: true },
  { id: 2, name: 'Lena', accent: 'rose', hasPin: true, isPrimary: false },
  { id: 3, name: 'Noah', accent: 'sky', hasPin: false, isPrimary: false },
];

/** The same profiles with their settings, as `GET /profiles` returns them. */
export const profiles: Profile[] = [
  {
    id: 1,
    name: 'Marco',
    accent: 'teal',
    maxRating: 18,
    language: 'de',
    allLibraries: true,
    libraryIds: [],
    hasPin: false,
    hasOfflinePin: false,
    isPrimary: true,
    limits: {},
    effectiveLimits: {},
  },
  {
    id: 2,
    name: 'Lena',
    accent: 'rose',
    maxRating: 12,
    language: 'en',
    allLibraries: false,
    libraryIds: [1],
    hasPin: true,
    hasOfflinePin: false,
    isPrimary: false,
    limits: { maxHeight: 720, watchLimitMinutes: 60 },
    effectiveLimits: { maxHeight: 720, watchLimitMinutes: 60 },
  },
];

/** `GET /libraries`: what the account may hand out (all of them for the main account). */
export const accountLibraries: LibrarySummary[] = [
  { id: 1, name: 'Filme', kind: 'movies' },
  { id: 2, name: 'Serien', kind: 'series' },
];

// M3: a series with two seasons, an episode with its neighbours, a film with versions and extras.
export const seriesIds = {
  show: '01JB8Z6Q4M7N9P2R3S5T7V9W4A',
  season1: '01JB8Z6Q4M7N9P2R3S5T7V9W4B',
  specials: '01JB8Z6Q4M7N9P2R3S5T7V9W4C',
  e1: '01JB8Z6Q4M7N9P2R3S5T7V9W4D',
  e2: '01JB8Z6Q4M7N9P2R3S5T7V9W4E',
  e34: '01JB8Z6Q4M7N9P2R3S5T7V9W4F',
  s0e1: '01JB8Z6Q4M7N9P2R3S5T7V9W4G',
  extra: '01JB8Z6Q4M7N9P2R3S5T7V9W4H',
} as const;

const showRef: ItemRef = {
  id: seriesIds.show,
  kind: 'series',
  title: 'Slouchcat Test Show',
  year: 2024,
  images: {},
  childCount: 2,
};

const seasonRef = (
  id: string,
  title: string,
  seasonNumber: number,
  childCount: number,
): ItemRef => ({
  id,
  kind: 'season',
  title,
  seasonNumber,
  parentId: seriesIds.show,
  seriesTitle: showRef.title,
  childCount,
  images: {},
});

const episodeRef = (
  id: string,
  seasonNumber: number,
  episodeNumber: number,
  title: string,
  extra: Partial<ItemRef> = {},
): ItemRef => ({
  id,
  kind: 'episode',
  title,
  seasonNumber,
  episodeNumber,
  parentId: seasonNumber === 0 ? seriesIds.specials : seriesIds.season1,
  seriesTitle: showRef.title,
  runtimeMinutes: 45,
  images: {
    backdropSmall: { url: `/items/${id}/images/backdrop/w780?v=s`, width: 780, height: 439 },
  },
  ...extra,
});

export const episodes = {
  e1: episodeRef(seriesIds.e1, 1, 1, 'Pilot', {
    progress: {
      positionMs: 600_000,
      durationMs: 2_700_000,
      percent: 22,
      completed: false,
      updatedAt: '2026-10-18T19:00:00Z',
    },
  }),
  e2: episodeRef(seriesIds.e2, 1, 2, 'Die lange Nacht'),
  e34: episodeRef(seriesIds.e34, 1, 3, 'Episode 3', { episodeEnd: 4 }),
  s0e1: episodeRef(seriesIds.s0e1, 0, 1, 'Episode 1'),
};

const seasons = [
  seasonRef(seriesIds.season1, 'Season 1', 1, 3),
  seasonRef(seriesIds.specials, 'Specials', 0, 1),
];

const file = (id: string, label: string, height: number): ItemDetail['mediaFiles'][number] => ({
  id,
  container: 'mp4',
  mimeType: 'video/mp4; codecs="avc1.640028, mp4a.40.2"',
  videoCodec: 'h264',
  audioCodec: 'aac',
  width: Math.round((height * 16) / 9),
  height,
  durationMs: 2_700_000,
  sizeBytes: 900_000_000,
  versionLabel: label,
  available: true,
});

export const showDetail: ItemDetail = {
  ...showRef,
  genres: ['Drama'],
  mediaFiles: [],
  seasons,
  extras: [
    {
      id: seriesIds.extra,
      kind: 'extra',
      title: 'Making Of',
      extraKind: 'featurette',
      parentId: seriesIds.show,
      images: {},
    },
  ],
  cast: [
    { personId: 7, name: 'Lena Muster', role: 'cast', character: 'Kommissarin Weber' },
    { personId: 8, name: 'Noah Beispiel', role: 'creator' },
  ],
  trailerCount: 0,
};

export const season1Detail: ItemDetail = {
  ...seasons[0]!,
  genres: [],
  mediaFiles: [],
  series: showRef,
  episodes: [episodes.e1, episodes.e2, episodes.e34],
};

export const specialsDetail: ItemDetail = {
  ...seasons[1]!,
  genres: [],
  mediaFiles: [],
  series: showRef,
  episodes: [episodes.s0e1],
};

export const episode2Detail: ItemDetail = {
  ...episodes.e2,
  overview: 'Die zweite Folge.',
  genres: [],
  mediaFiles: [file('mf-e2', '1080p', 1080)],
  series: showRef,
  season: seasons[0],
  nextEpisode: episodes.e34,
};

export const extraDetail: ItemDetail = {
  id: seriesIds.extra,
  kind: 'extra',
  title: 'Making Of',
  extraKind: 'featurette',
  parentId: seriesIds.show,
  images: {},
  genres: [],
  mediaFiles: [file('mf-extra', '720p', 720)],
};

/** Alien with two versions (B-5), extras (B-7), cast and two trailers (W-10). */
export const alienVersions: ItemDetail = {
  ...alienDetail,
  mediaFiles: [alienDetail.mediaFiles[0]!, file('mf-2', '720p', 720)],
  extras: [
    {
      id: '01JB8Z6Q4M7N9P2R3S5T7V9W5A',
      kind: 'extra',
      title: 'Alien Trailer',
      extraKind: 'trailer',
      parentId: ids.alien,
      images: {},
    },
    {
      id: '01JB8Z6Q4M7N9P2R3S5T7V9W5B',
      kind: 'extra',
      title: 'On Set',
      extraKind: 'behind_the_scenes',
      parentId: ids.alien,
      images: {},
    },
  ],
  cast: [{ personId: 3, name: 'Sigourney Weaver', role: 'cast', character: 'Ellen Ripley' }],
  trailerCount: 2,
};
