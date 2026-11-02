import type { Dashboard } from '@slouchcat/api-client';
import { screen } from '@testing-library/react';
import { json, renderApp, type Routes } from './api';

/**
 * A quiet dashboard with every field the overview and the admin sidebar read (contract 1.7.0):
 * nothing plays, nothing failed, nothing needs care, so the sidebar shows counts but no pills.
 */
export const dashboardFixture = (overrides: Partial<Dashboard> = {}): Dashboard => ({
  streams: [],
  jobs: { queued: 0, running: 0, failed: 0, recentFailed: [], active: [], failedByType: [] },
  storage: {
    config: { path: '/config', totalBytes: 100e9, freeBytes: 60e9 },
    cache: { path: '/cache', totalBytes: 500e9, freeBytes: 450e9 },
  },
  integrations: [
    { key: 'tmdb', state: 'configured' },
    { key: 'trakt', state: 'missing' },
  ],
  webhooks: { total: 0, eventsLast24h: 0, silent: [] },
  errors: [],
  update: { enabled: true, currentVersion: '0.9.0', development: false, available: false },
  attention: { abortedScans: [] },
  counts: { titles: 1200, accounts: 3, maintenanceOpen: 0 },
  activity: [],
  ...overrides,
});

/** The dashboard behind the admin sidebar's numbers (AdminShellNav), for a page test's routes. */
export const adminShell: Routes = { 'GET /admin/dashboard': () => json(dashboardFixture()) };

/**
 * Renders an admin page in the whole app (router, shell, page) and waits for the shell's admin
 * sidebar, which loads lazily (AppLayout).
 */
export async function renderAdminPage(path: string) {
  const view = renderApp(path);
  const nav = await screen.findByRole('navigation', { name: 'Verwaltung' });
  return { ...view, nav };
}
