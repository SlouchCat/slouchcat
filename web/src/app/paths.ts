// Route paths as plain strings for links (AppLink hands them to the router).
export const paths = {
  home: '/',
  login: '/login',
  setup: '/setup',
  profiles: '/profiles',
  profileSettings: '/settings/profiles',
  /** «Meine Wiedergabe»: languages and subtitles of the active profile (concept 13 11.4). */
  playbackSettings: '/settings/playback',
  adminLibraries: '/admin/libraries',
  adminMaintenance: '/admin/maintenance',
  adminDevices: '/admin/devices',
  adminStreams: '/admin/streams',
  /** Downloads of the active profile on this device (M5). */
  downloads: '/downloads',
  /** The offline library: works without a session and without network (C-1, O-4). */
  offline: '/offline',
  /**
   * The player of a download. From the offline library it plays for the profile chosen there;
   * `session` (from /downloads) plays for the session's active profile instead (W1, K9).
   */
  offlinePlay: (downloadId: string, from?: 'session') =>
    `/offline/play/${encodeURIComponent(downloadId)}${from ? `?as=${from}` : ''}`,
  item: (id: string) => `/items/${encodeURIComponent(id)}`,
  /** The player; `file` picks a version (media file) of the title (B-5). */
  play: (id: string, file?: string) =>
    `/play/${encodeURIComponent(id)}${file ? `?file=${encodeURIComponent(file)}` : ''}`,
  /**
   * The player from a position (ms): a chapter of an audiobook (M6), "Von vorne" and "Nochmals
   * ansehen" at 0 (concept 17 D-2); `file` as in `play`.
   */
  playAt: (id: string, positionMs: number, file?: string) =>
    `/play/${encodeURIComponent(id)}?at=${Math.max(0, Math.round(positionMs))}${
      file ? `&file=${encodeURIComponent(file)}` : ''
    }`,
  adminPodcasts: '/admin/podcasts',
  /** Webradio in the administration (concept 16, 11): catalog, relays, stations, settings. */
  adminRadio: '/admin/radio',
  /** Discover and curation (M7). */
  search: '/search',
  watchlist: '/watchlist',
  /** Webradio (concept 16): the station catalog with filters, search and favourites. */
  radio: '/radio',
  /** A library with every title the active profile may see (concept 08). */
  library: (id: number) => `/libraries/${id}`,
  /** An audiobook series of a library with its volumes (concept 19 D4.3). */
  audioSeries: (libraryId: number, seriesId: string) =>
    `/libraries/${libraryId}/series/${encodeURIComponent(seriesId)}`,
  /** All of Cat's recommendations for the active profile (concept 08). */
  picks: '/picks',
  notifications: '/notifications',
  collection: (id: string) => `/collections/${encodeURIComponent(id)}`,
  adminPicks: '/admin/picks',
  adminCollections: '/admin/collections',
  adminHomeRows: '/admin/home-rows',
  /** The spotlight list of the start page (M11, ADR 0011). */
  adminSpotlight: '/admin/spotlight',
  person: (id: number) => `/people/${id}`,
  /** Administration (M8): the overview with the dashboard and the cards of every admin page. */
  admin: '/admin',
  adminIntegrations: '/admin/integrations',
  adminMetadata: '/admin/metadata',
  /** «Hörbücher: Quellen» (concept 19 D6.6): the book sources, under «Verwaltung › Metadaten». */
  adminAudiobookSources: '/admin/metadata/audiobooks',
  /** Background jobs (concept 09): job types, their runs, schedules, pause and cancel. */
  adminJobs: '/admin/jobs',
  adminAccounts: '/admin/accounts',
  adminAnnouncements: '/admin/announcements',
  /** Usage statistics; `days` picks the period (7, 30, 90 or 365, default 30). */
  adminStats: '/admin/stats',
  adminBackup: '/admin/backup',
  /** Notification channels of the signed-in account (M8), for every account with the app's column. */
  notificationSettings: '/settings/notifications',
  /** The same page inside the administration (concept 15, P-4), with the admin sidebar. */
  adminNotifications: '/admin/notifications',
  adminItem: (id: string) => `/admin/items/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
