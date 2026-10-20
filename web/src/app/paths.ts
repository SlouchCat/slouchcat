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
  offlinePlay: (downloadId: string) => `/offline/play/${encodeURIComponent(downloadId)}`,
  item: (id: string) => `/items/${encodeURIComponent(id)}`,
  /** The player; `file` picks a version (media file) of the title (B-5). */
  play: (id: string, file?: string) =>
    `/play/${encodeURIComponent(id)}${file ? `?file=${encodeURIComponent(file)}` : ''}`,
  person: (id: number) => `/people/${id}`,
  adminItem: (id: string) => `/admin/items/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
