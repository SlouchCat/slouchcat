import type { NotificationPage, ProfileAnnouncement } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';

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
  body: 'Von 8 bis 10 Uhr ist Slouchcat weg.\nDownloads spielen weiter.\n\nDanach gibt es Neues.',
  publishedAt: '2026-10-21T08:00:00Z',
};
const collections: ProfileAnnouncement = {
  id: 7,
  title: 'Neue Sammlungen',
  body: 'Schaut unter Sammlungen nach.',
  publishedAt: '2026-10-20T08:00:00Z',
};

/** The dialog comes with its own chunk (react-aria, P-10): the first one may take a moment. */
const findDialog = (name: string) => screen.findByRole('dialog', { name }, { timeout: 5000 });

/** The inbox page as a simple page with the app header (the dialog shows on every such page). */
function routes(list: ProfileAnnouncement[]): Routes {
  return {
    ...signedIn,
    'GET /notifications': () => json(emptyInbox),
    'GET /announcements': () => json(list),
    'POST /announcements/*/ack': () => new Response(null, { status: 204 }),
  };
}

describe('announcements at the start', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the newest one; "Verstanden" acknowledges it and the next follows', async () => {
    const { calls } = mockApi(routes([maintenance, collections]));
    renderApp('/notifications');
    const dialog = await findDialog('Wartung am Samstag');
    expect(within(dialog).getByText('Ankündigung')).toBeVisible();
    expect(within(dialog).getByText('Veröffentlicht am 21.10.2026')).toBeVisible();
    // Paragraphs at blank lines, the single line break stays inside the first one.
    const paragraphs = within(dialog).getAllByText(/Slouchcat weg|Danach gibt es Neues/);
    expect(paragraphs).toHaveLength(2);
    expect(paragraphs[0]).toHaveTextContent('Von 8 bis 10 Uhr ist Slouchcat weg. Downloads');
    expect(within(dialog).getByRole('status')).toHaveTextContent('1 von 2');
    const ok = within(dialog).getByRole('button', { name: 'Verstanden' });
    await waitFor(() => expect(ok).toHaveFocus());

    const user = userEvent.setup();
    await user.click(ok);
    const next = await findDialog('Neue Sammlungen');
    expect(within(next).getByRole('status')).toHaveTextContent('2 von 2');
    await user.click(within(next).getByRole('button', { name: 'Verstanden' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());

    const acks = calls.filter((call) => call.method === 'POST').map((call) => call.path);
    expect(acks).toEqual(['/announcements/8/ack', '/announcements/7/ack']);
    // Asked once for the profile, not again after the acknowledgements.
    expect(calls.filter((call) => call.path === '/announcements')).toHaveLength(1);
  });

  it('Escape closes it without acknowledging; it stays closed on the next page', async () => {
    const { calls } = mockApi({
      ...routes([collections]),
      'GET /me/channels': () => json({ channels: [] }),
    });
    const { router } = renderApp('/notifications');
    await findDialog('Neue Sammlungen');
    const user = userEvent.setup();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    // The page behind did not go back either.
    expect(router.state.location.pathname).toBe('/notifications');

    await router.navigate({ to: '/settings/notifications' });
    await screen.findByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });

  it("Backspace (a remote's back key) closes it as well", async () => {
    mockApi(routes([collections]));
    renderApp('/notifications');
    await findDialog('Neue Sammlungen');
    const user = userEvent.setup();
    await user.keyboard('{Backspace}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('"Später" closes it without acknowledging', async () => {
    const { calls } = mockApi(routes([collections]));
    renderApp('/notifications');
    const dialog = await findDialog('Neue Sammlungen');
    await userEvent.setup().click(within(dialog).getByRole('button', { name: 'Später' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });

  it('keeps the dialog with a message when the acknowledgement fails', async () => {
    mockApi({
      ...routes([collections]),
      'POST /announcements/*/ack': () => problem(500, 'internal'),
    });
    renderApp('/notifications');
    const dialog = await findDialog('Neue Sammlungen');
    await userEvent.setup().click(within(dialog).getByRole('button', { name: 'Verstanden' }));
    expect(
      await within(dialog).findByText('Etwas ist schiefgelaufen. Versuche es erneut.'),
    ).toBeVisible();
    expect(screen.getByRole('dialog', { name: 'Neue Sammlungen' })).toBeVisible();
  });

  it('drops one that was withdrawn meanwhile (404)', async () => {
    mockApi({
      ...routes([collections]),
      'POST /announcements/*/ack': () => problem(404, 'not_found'),
    });
    renderApp('/notifications');
    const dialog = await findDialog('Neue Sammlungen');
    await userEvent.setup().click(within(dialog).getByRole('button', { name: 'Verstanden' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('shows nothing without announcements or when the server cannot tell', async () => {
    mockApi(routes([]));
    renderApp('/notifications');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Benachrichtigungen' }),
    ).toBeVisible();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

    vi.unstubAllGlobals();
    mockApi({ ...routes([]), 'GET /announcements': () => problem(501, 'not_implemented') });
    renderApp('/notifications');
    expect(
      (await screen.findAllByRole('heading', { level: 1, name: 'Benachrichtigungen' })).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });
});
