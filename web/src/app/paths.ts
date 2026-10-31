// Route paths as plain strings for links (AppLink hands them to the router).
export const paths = {
  home: '/',
  login: '/login',
  setup: '/setup',
  profiles: '/profiles',
  profileSettings: '/settings/profiles',
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
  /** The player from a position (ms): a chapter of an audiobook (M6). */
  playAt: (id: string, positionMs: number) =>
    `/play/${encodeURIComponent(id)}?at=${Math.max(0, Math.round(positionMs))}`,
  adminPodcasts: '/admin/podcasts',
  /** Discover and curation (M7). */
  search: '/search',
  watchlist: '/watchlist',
  /** A library with every title the active profile may see (concept 08). */
  library: (id: number) => `/libraries/${id}`,
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
  /** Background jobs (concept 09): job types, their runs, schedules, pause and cancel. */
  adminJobs: '/admin/jobs',
  adminAccounts: '/admin/accounts',
  adminAnnouncements: '/admin/announcements',
  /** Usage statistics; `days` picks the period (7, 30, 90 or 365, default 30). */
  adminStats: '/admin/stats',
  adminBackup: '/admin/backup',
  /** Notification channels of the signed-in account (M8). */
  notificationSettings: '/settings/notifications',
  adminItem: (id: string) => `/admin/items/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
