import type { Notification, NotificationPage } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { episodes, homePage, seriesIds } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

const show = {
  id: seriesIds.show,
  kind: 'series' as const,
  title: 'Slouchcat Test Show',
  images: {},
};

const entries: Notification[] = [
  {
    id: 2,
    kind: 'new_episode',
    createdAt: '2026-10-21T08:15:00Z',
    item: episodes.e2,
    parent: show,
  },
  {
    id: 1,
    kind: 'new_episode',
    createdAt: '2026-10-20T08:15:00Z',
    readAt: '2026-10-20T09:00:00Z',
    item: episodes.e1,
    parent: show,
  },
];

const inbox = (list: Notification[]): NotificationPage => ({
  notifications: list,
  total: list.length,
  unread: list.filter((entry) => !entry.readAt).length,
  offset: 0,
  limit: 50,
});

describe('notifications', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the unread count in the header and only asks for one entry there', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /home': () => json(homePage),
      'GET /notifications': () => json(inbox(entries)),
    });
    renderApp('/');
    const link = await screen.findByRole('link', { name: 'Benachrichtigungen, 1 ungelesen' });
    expect(link).toHaveAttribute('href', '/notifications');
    expect(within(link).getByText('1')).toBeVisible();
    // The badge needs only `unread`; the full list waits for the page.
    const badge = calls.find((call) => call.path === '/notifications');
    expect(badge).toBeDefined();
  });

  it('lists the entries; opening one marks it read', async () => {
    let list = entries;
    const { calls } = mockApi({
      ...signedIn,
      'GET /notifications': () => json(inbox(list)),
      'PUT /notifications/2/read': () => {
        list = list.map((entry) => ({ ...entry, readAt: '2026-10-21T09:00:00Z' }));
        return new Response(null, { status: 204 });
      },
      'GET /items/*': () =>
        json({
          ...episodes.e2,
          genres: [],
          mediaFiles: [],
          inWatchlist: false,
          favorite: false,
          collections: [],
        }),
    });
    const { router } = renderApp('/notifications');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Benachrichtigungen' }),
    ).toBeVisible();
    const list1 = await screen.findByRole('list', { name: 'Benachrichtigungen' });
    const unread = within(list1).getByRole('link', { name: /Ungelesen: S01E02 · Die lange Nacht/ });
    expect(within(unread).getByText('Neue Folge von Slouchcat Test Show')).toBeVisible();
    expect(within(list1).getByRole('link', { name: /S01E01 · Pilot/ })).not.toHaveAccessibleName(
      /Ungelesen/,
    );
    expect(screen.getByText('1 ungelesen')).toBeVisible();

    await userEvent.setup().click(unread);
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${seriesIds.e2}`));
    expect(
      calls.some((call) => call.method === 'PUT' && call.path === '/notifications/2/read'),
    ).toBe(true);
  });

  it('marks all read and the badge goes away', async () => {
    let list = entries;
    const { calls } = mockApi({
      ...signedIn,
      'GET /notifications': () => json(inbox(list)),
      'POST /notifications/read-all': () => {
        list = list.map((entry) => ({ ...entry, readAt: '2026-10-21T09:00:00Z' }));
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/notifications');
    await screen.findByRole('link', { name: 'Benachrichtigungen, 1 ungelesen' });
    await userEvent
      .setup()
      .click(await screen.findByRole('button', { name: 'Alle als gelesen markieren' }));
    await waitFor(() =>
      expect(screen.getByRole('link', { name: 'Benachrichtigungen' })).toBeVisible(),
    );
    expect(
      screen.queryByRole('button', { name: 'Alle als gelesen markieren' }),
    ).not.toBeInTheDocument();
    expect(screen.getByText('0 ungelesen')).toBeVisible();
    expect(calls.some((call) => call.path === '/notifications/read-all')).toBe(true);
  });

  it('"Alle als gelesen markieren" puts the focus on "Zurück" before the answer', async () => {
    let list = entries;
    let answer: () => void = () => undefined;
    const answered = new Promise<void>((resolve) => {
      answer = resolve;
    });
    mockApi({
      ...signedIn,
      'GET /notifications': () => json(inbox(list)),
      'POST /notifications/read-all': async () => {
        await answered;
        list = list.map((entry) => ({ ...entry, readAt: '2026-10-21T09:00:00Z' }));
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/notifications');
    const readAll = await screen.findByRole('button', { name: 'Alle als gelesen markieren' });
    await userEvent.setup().click(readAll);
    const back = screen.getByRole('button', { name: 'Zurück' });
    // Still waiting for the server, the button is there, and the focus is already on "Zurück".
    await waitFor(() => expect(back).toHaveFocus());
    expect(readAll).toBeInTheDocument();

    answer();
    await waitFor(() => expect(readAll).not.toBeInTheDocument());
    expect(back).toHaveFocus();
  });

  it('the open inbox follows the badge when a new notification arrives', async () => {
    let list = entries;
    mockApi({ ...signedIn, 'GET /notifications': () => json(inbox(list)) });
    const { queryClient } = renderApp('/notifications');
    const page = await screen.findByRole('list', { name: 'Benachrichtigungen' });
    expect(within(page).getAllByRole('link')).toHaveLength(2);

    list = [
      {
        id: 3,
        kind: 'new_episode',
        createdAt: '2026-10-21T10:00:00Z',
        item: episodes.e34,
        parent: show,
      },
      ...entries,
    ];
    // The badge's next poll (every 30 s) counts one more.
    await queryClient.refetchQueries({ queryKey: ['notifications', 'badge'] });
    await screen.findByRole('link', { name: 'Benachrichtigungen, 2 ungelesen' });
    await waitFor(() => expect(within(page).getAllByRole('link')).toHaveLength(3));
    expect(screen.getByText('2 ungelesen')).toBeVisible();
  });

  it('caps the badge at 99 with the text of the locale', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () => json(homePage),
      'GET /notifications': () => json({ ...inbox(entries), unread: 250 }),
    });
    renderApp('/');
    const link = await screen.findByRole('link', { name: 'Benachrichtigungen, 250 ungelesen' });
    expect(within(link).getByText('99+')).toBeVisible();
  });

  it('shows an empty inbox', async () => {
    mockApi({ ...signedIn, 'GET /notifications': () => json(inbox([])) });
    renderApp('/notifications');
    expect(await screen.findByText('Keine Benachrichtigungen')).toBeVisible();
  });
});
