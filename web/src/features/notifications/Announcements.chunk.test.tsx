import type { NotificationPage, ProfileAnnouncement } from '@slouchcat/api-client';
import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

// The dialog's chunk cannot load (an old tab after an update dropped it): no dialog, and the
// page stays. In its own file because the module mock holds for the whole file.

const attempts = vi.hoisted(() => ({ count: 0 }));

vi.mock('./AnnouncementDialog', () => {
  attempts.count += 1;
  throw new Error('Failed to fetch dynamically imported module');
});

const emptyInbox: NotificationPage = {
  notifications: [],
  total: 0,
  unread: 0,
  offset: 0,
  limit: 50,
};

const maintenance: ProfileAnnouncement = {
  id: 8,
  title: 'Wartung am Samstag',
  body: 'Von 8 bis 10 Uhr ist Slouchcat weg.',
  publishedAt: '2026-10-21T08:00:00Z',
};

describe('announcements whose dialog does not load', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows no dialog and keeps the page instead of the error view', async () => {
    mockApi({
      ...signedIn,
      'GET /notifications': () => json(emptyInbox),
      'GET /announcements': () => json([maintenance]),
    });
    renderApp('/notifications');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Benachrichtigungen' }),
    ).toBeVisible();
    await waitFor(() => expect(attempts.count).toBeGreaterThan(0));
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(screen.queryByText('Slouchcat ist gerade nicht erreichbar')).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: 'Benachrichtigungen' })).toBeVisible();
  });
});
