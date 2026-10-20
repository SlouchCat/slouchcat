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
  item: (id: string) => `/items/${encodeURIComponent(id)}`,
  /** The player; `file` picks a version (media file) of the title (B-5). */
  play: (id: string, file?: string) =>
    `/play/${encodeURIComponent(id)}${file ? `?file=${encodeURIComponent(file)}` : ''}`,
  person: (id: number) => `/people/${id}`,
  adminItem: (id: string) => `/admin/items/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
