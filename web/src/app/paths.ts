// Route paths as plain strings for links (AppLink hands them to the router).
export const paths = {
  home: '/',
  login: '/login',
  setup: '/setup',
  profiles: '/profiles',
  profileSettings: '/settings/profiles',
  adminLibraries: '/admin/libraries',
  adminAccess: '/admin/access',
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
  notifications: '/notifications',
  collection: (id: string) => `/collections/${encodeURIComponent(id)}`,
  adminPicks: '/admin/picks',
  adminCollections: '/admin/collections',
  adminHomeRows: '/admin/home-rows',
  person: (id: number) => `/people/${id}`,
  adminItem: (id: string) => `/admin/items/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
