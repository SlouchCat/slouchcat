import type { Guest, Invitation } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

const invitation = (overrides: Partial<Invitation> = {}): Invitation => ({
  id: 1,
  note: 'Oma',
  state: 'open',
  maxRating: 12,
  allLibraries: true,
  libraryIds: [],
  createdAt: '2026-10-19T10:00:00Z',
  expiresAt: '2026-10-26T10:00:00Z',
  ...overrides,
});

const guest: Guest = {
  id: 5,
  email: 'oma@example.test',
  maxRating: 12,
  allLibraries: false,
  libraryIds: [1],
  profileCount: 2,
  createdAt: '2026-10-19T10:00:00Z',
  lastSeenAt: '2026-10-19T11:00:00Z',
  limits: { maxHeight: 720 },
};

const access = (invitations: Invitation[] = [], guests: Guest[] = []): Routes => ({
  ...signedIn,
  'GET /libraries': () => json(accountLibraries),
  'GET /admin/invitations': () => json(invitations),
  'GET /admin/guests': () => json(guests),
});

describe('access page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, account: { ...session.account, role: 'guest' } }),
    });
    renderApp('/admin/access');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });

  it('creates an invitation and shows the full link once, with a copy button', async () => {
    const created = {
      ...invitation({
        id: 2,
        note: 'Tante Rosa',
        maxRating: 16,
        allLibraries: false,
        libraryIds: [2],
      }),
      url: '/invite/geheimes-token',
      token: 'geheimes-token',
    };
    let listed: Invitation[] = [];
    const { calls } = mockApi({
      ...access(),
      'GET /admin/invitations': () => json(listed),
      'POST /admin/invitations': () => {
        listed = [created];
        return json(created, 201);
      },
    });
    renderApp('/admin/access');
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');

    await user.type(await screen.findByLabelText(/Für wen ist der Link/), 'Tante Rosa');
    await user.click(screen.getByRole('radio', { name: 'FSK 16' }));
    await user.click(screen.getByRole('radio', { name: 'Auswahl' }));
    await user.click(screen.getByRole('checkbox', { name: 'Serien' }));
    await user.click(screen.getByRole('button', { name: 'Einladung erstellen' }));

    const link = await screen.findByLabelText('Einladungslink');
    expect(link).toHaveValue(`${window.location.origin}/invite/geheimes-token`);
    expect(link).toHaveAttribute('readonly');
    expect(screen.getByText(/nur dieses eine Mal angezeigt/)).toBeVisible();
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      note: 'Tante Rosa',
      maxRating: 16,
      allLibraries: false,
      libraryIds: [2],
    });

    await user.click(screen.getByRole('button', { name: 'Link kopieren' }));
    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/invite/geheimes-token`);
    expect(await screen.findByText('Link kopiert.')).toBeVisible();

    // The new row is in the list; the link itself is not part of it.
    const list = screen.getByText('Bestehende Einladungen').closest('div')!;
    expect(await within(list).findByText('Tante Rosa')).toBeVisible();
    expect(within(list).queryByText(/geheimes-token/)).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Link ausblenden' }));
    expect(screen.queryByLabelText('Einladungslink')).not.toBeInTheDocument();
  });

  it('selects the link for Ctrl+C when the clipboard is not available', async () => {
    mockApi({
      ...access(),
      'POST /admin/invitations': () =>
        json({ ...invitation(), url: '/invite/abc', token: 'abc' }, 201),
    });
    renderApp('/admin/access');
    const user = userEvent.setup();
    vi.spyOn(navigator.clipboard, 'writeText').mockRejectedValue(new Error('not allowed'));

    await user.click(await screen.findByRole('button', { name: 'Einladung erstellen' }));
    await user.click(await screen.findByRole('button', { name: 'Link kopieren' }));
    expect(await screen.findByText(/Kopieren nicht möglich/)).toBeVisible();
    expect(screen.getByLabelText('Einladungslink')).toHaveFocus();
  });

  it('lists invitations with their state and revokes an open one after confirmation', async () => {
    const { calls } = mockApi({
      ...access([
        invitation(),
        invitation({ id: 2, note: 'Max', state: 'used', usedByEmail: 'max@example.test' }),
        invitation({ id: 3, note: undefined, state: 'expired' }),
        invitation({ id: 4, note: 'Alt', state: 'revoked' }),
      ]),
      'DELETE /admin/invitations/1': () => new Response(null, { status: 204 }),
    });
    renderApp('/admin/access');
    const user = userEvent.setup();

    expect(await screen.findByText('Offen')).toBeVisible();
    expect(screen.getByText('Benutzt')).toBeVisible();
    expect(screen.getByText('Abgelaufen')).toBeVisible();
    expect(screen.getByText('Widerrufen', { selector: 'span' })).toBeVisible();
    expect(screen.getByText('Angenommen von max@example.test')).toBeVisible();
    expect(screen.getByText('Ohne Notiz')).toBeVisible();
    // Only the open invitation can be revoked.
    expect(screen.getAllByRole('button', { name: 'Widerrufen' })).toHaveLength(1);

    await user.click(screen.getByRole('button', { name: 'Widerrufen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Einladung widerrufen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Widerrufen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
  });

  it('edits a guest and warns that removing ends the sessions at once', async () => {
    const { calls } = mockApi({
      ...access([], [guest]),
      'PATCH /admin/guests/5': () => json({ ...guest, maxRating: 6 }),
      'DELETE /admin/guests/5': () => new Response(null, { status: 204 }),
    });
    renderApp('/admin/access');
    const user = userEvent.setup();

    const row = (await screen.findByText('oma@example.test')).closest('li')!;
    expect(within(row).getByText('FSK 12')).toBeVisible();
    expect(within(row).getByText('2 Profile')).toBeVisible();

    await user.click(screen.getByRole('button', { name: 'Bearbeiten' }));
    const edit = await screen.findByRole('dialog', { name: 'Gast oma@example.test bearbeiten' });
    expect(within(edit).getByRole('radio', { name: 'FSK 12' })).toBeChecked();
    await user.click(within(edit).getByRole('radio', { name: 'FSK 6' }));
    await user.click(within(edit).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(edit).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toEqual({
      maxRating: 6,
      allLibraries: false,
      libraryIds: [1],
      // The stored limits go along unchanged (the object replaces all three).
      limits: { maxHeight: 720 },
    });

    await user.click(screen.getByRole('button', { name: 'Entfernen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Gast oma@example.test entfernen?',
    });
    expect(dialog).toHaveTextContent('enden sofort');
    await user.click(within(dialog).getByRole('button', { name: 'Gast entfernen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
  });

  it('sets the three playback limits of a guest account (W-8), without a watch time', async () => {
    const { calls } = mockApi({
      ...access([], [guest]),
      'PATCH /admin/guests/5': () => json(guest),
    });
    renderApp('/admin/access');
    const user = userEvent.setup();

    const row = (await screen.findByText('oma@example.test')).closest('li')!;
    expect(within(row).getByText('bis 720p')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Bearbeiten' }));
    const edit = await screen.findByRole('dialog', { name: 'Gast oma@example.test bearbeiten' });
    expect(within(edit).getByText('Gilt für alle Profile des Gastkontos.')).toBeVisible();
    expect(within(edit).queryByLabelText('Schauzeit pro Tag (Minuten)')).not.toBeInTheDocument();
    await user.click(within(edit).getByRole('radio', { name: '480p' }));
    await user.type(within(edit).getByLabelText('Bandbreite (kbit/s)'), '4000');
    await user.type(within(edit).getByLabelText('Gleichzeitige Streams'), '1');
    await user.click(within(edit).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(edit).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
      limits: { maxHeight: 480, maxBitrateKbps: 4000, maxStreams: 1 },
    });
  });

  it('offers no invitation form while the library list is missing, so no right is widened', async () => {
    const { calls } = mockApi({
      ...access(),
      'GET /libraries': () => problem(500, 'internal'),
      'POST /admin/invitations': () => json({ ...invitation(), url: '/invite/x', token: 'x' }, 201),
    });
    renderApp('/admin/access');

    expect(await screen.findByText('Die Bibliotheken konnten nicht geladen werden.')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Einladung erstellen' })).not.toBeInTheDocument();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });
});
