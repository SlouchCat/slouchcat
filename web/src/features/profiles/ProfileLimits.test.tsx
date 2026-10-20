import type { Profile, WatchReport } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { accountLibraries, profiles } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn, type Routes } from '@/test/api';

// W-8 and K-5 on the profile settings page: limits per profile, the guest account's cap and the
// watch-time report. The server enforces and counts; the page edits and shows.

const routes = (list: Profile[] = profiles, extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /profiles': () => json(list),
  'GET /libraries': () => json(accountLibraries),
  ...extra,
});

const report: WatchReport = {
  profileId: 2,
  limitMinutes: 60,
  days: Array.from({ length: 30 }, (_, index) => ({
    date: `2026-09-${String(index + 1).padStart(2, '0')}`,
    minutes: index === 29 ? 12 : index % 7 === 0 ? 75 : 20,
  })),
};

describe('profile limits and watch-time report', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the limits that apply on every profile card', async () => {
    mockApi(routes());
    renderApp('/settings/profiles');
    const lena = await screen.findByRole('article', { name: 'Lena' });
    expect(within(lena).getByText('bis 720p')).toBeVisible();
    expect(within(lena).getByText('60 min pro Tag')).toBeVisible();
    const marco = screen.getByRole('article', { name: 'Marco' });
    expect(within(marco).queryByText(/^bis /)).not.toBeInTheDocument();
  });

  it('sends all limits as one object; an emptied field clears its limit', async () => {
    const { calls } = mockApi(
      routes(profiles, { 'PATCH /profiles/2': () => json({ ...profiles[1]!, limits: {} }) }),
    );
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    const lena = await screen.findByRole('article', { name: 'Lena' });
    await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });

    expect(within(dialog).getByRole('radio', { name: '720p' })).toBeChecked();
    expect(within(dialog).getByLabelText('Schauzeit pro Tag (Minuten)')).toHaveValue('60');
    await user.type(within(dialog).getByLabelText('Gleichzeitige Streams'), '2');
    await user.clear(within(dialog).getByLabelText('Schauzeit pro Tag (Minuten)'));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'PATCH')?.body).toMatchObject({
      limits: { maxHeight: 720, maxStreams: 2 },
    });
  });

  it('checks the ranges before sending', async () => {
    const { calls } = mockApi(routes());
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    const lena = await screen.findByRole('article', { name: 'Lena' });
    await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    await user.type(within(dialog).getByLabelText('Bandbreite (kbit/s)'), '50');
    await user.click(within(dialog).getByRole('radio', { name: 'Ohne Limit' }));
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    expect(await within(dialog).findByText('Erlaubt sind 200 bis 200000.')).toBeVisible();
    expect(calls.some((call) => call.method === 'PATCH')).toBe(false);
  });

  it('names the cap of a guest account in the form', async () => {
    const capped: Profile = {
      ...profiles[1]!,
      limits: { maxHeight: 1080 },
      effectiveLimits: { maxHeight: 480, maxStreams: 1 },
    };
    mockApi(routes([profiles[0]!, capped]));
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    const lena = await screen.findByRole('article', { name: 'Lena' });
    // The card shows what applies.
    expect(within(lena).getByText('bis 480p')).toBeVisible();
    await user.click(within(lena).getByRole('button', { name: 'Bearbeiten' }));
    const dialog = await screen.findByRole('dialog', { name: 'Profil bearbeiten' });
    expect(
      within(dialog).getByText('Das Gastkonto begrenzt dieses Profil: bis 480p · 1 Stream.'),
    ).toBeVisible();
  });

  it('opens the watch-time report of a profile: bars and a table of the 30 days', async () => {
    const { fetchMock } = mockApi(
      routes(profiles, { 'GET /profiles/2/watch-time': () => json(report) }),
    );
    renderApp('/settings/profiles');
    const user = userEvent.setup();
    const lena = await screen.findByRole('article', { name: 'Lena' });
    const toggle = within(lena).getByRole('button', { name: 'Schauzeit anzeigen' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle);

    const section = await within(lena).findByRole('region', { name: 'Schauzeit von Lena' });
    expect(await within(section).findByText('Heute: 12 min')).toBeVisible();
    expect(within(section).getByText('Limit: 60 min pro Tag')).toBeVisible();
    const table = within(section).getByRole('table', { name: 'Schauzeit pro Tag' });
    expect(within(table).getAllByRole('row')).toHaveLength(31);
    expect(within(table).getAllByRole('cell').at(-1)).toHaveTextContent('12 min');
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .find((url) => url.pathname.endsWith('/profiles/2/watch-time'));
    expect(asked?.searchParams.get('days')).toBe('30');

    await user.click(within(lena).getByRole('button', { name: 'Schauzeit ausblenden' }));
    expect(within(lena).queryByRole('region')).not.toBeInTheDocument();
  });
});
