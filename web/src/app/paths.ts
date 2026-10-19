// Route paths as plain strings for links (AppLink hands them to the router).
export const paths = {
  home: '/',
  login: '/login',
  setup: '/setup',
  profiles: '/profiles',
  profileSettings: '/settings/profiles',
  adminLibraries: '/admin/libraries',
  adminAccess: '/admin/access',
  item: (id: string) => `/items/${encodeURIComponent(id)}`,
  play: (id: string) => `/play/${encodeURIComponent(id)}`,
  invite: (token: string) => `/invite/${encodeURIComponent(token)}`,
} as const;
