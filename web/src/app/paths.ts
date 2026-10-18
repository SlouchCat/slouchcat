// Route paths as plain strings for links (react-aria Link hands them to the router).
export const paths = {
  home: '/',
  login: '/login',
  setup: '/setup',
  adminLibraries: '/admin/libraries',
  item: (id: string) => `/items/${encodeURIComponent(id)}`,
  play: (id: string) => `/play/${encodeURIComponent(id)}`,
} as const;
