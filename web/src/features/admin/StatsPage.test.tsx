import type { Stats } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, renderApp, session, signedIn } from '@/test/api';

// A-8: the most watched titles, watch time per profile, the transcode share and the devices over
// 7, 30, 90 or 365 days. The server counts; the page shows its numbers.

const stats = (days: Stats['days']): Stats => ({
  days,
  topTitles: [
    { itemId: ids.heat, title: 'Heat', kind: 'movie', year: 1995, plays: 3, watchedMs: 18_000_000 },
    { itemId: ids.alien, title: 'Pilot', kind: 'episode', plays: 1, watchedMs: 2_700_000 },
  ],
  watchTime: [
    {
      profileId: 1,
      profileName: 'Marco',
      accountEmail: 'owner@example.test',
      watchedMs: 18_000_000,
      plays: 3,
    },
    {
      profileId: 9,
      profileName: 'Lena',
      accountEmail: 'gast@example.test',
      watchedMs: 2_700_000,
      plays: 1,
    },
  ],
  playback: { direct: 2, remux: 1, transcode: 1, transcodeShare: 0.25, hardwareShare: 1 },
  devices: [
    { type: 'desktop', count: 2, watchedMs: 18_000_000 },
    { type: 'unknown', count: 1, watchedMs: 2_700_000 },
  ],
});

const empty: Stats = {
  days: 30,
  topTitles: [],
  watchTime: [],
  playback: { direct: 0, remux: 0, transcode: 0, transcodeShare: 0, hardwareShare: 0 },
  devices: [],
};

/** The `days` of every statistics request, in order. */
function askedDays(fetchMock: ReturnType<typeof mockApi>['fetchMock']): (string | null)[] {
  return fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname === '/api/v1/admin/stats')
    .map((url) => url.searchParams.get('days'));
}

const routes = {
  ...signedIn,
  'GET /admin/stats': () => json(stats(30)),
};

describe('admin statistics (A-8)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the four blocks for the last 30 days', async () => {
    const { fetchMock } = mockApi(routes);
    renderApp('/admin/stats');
    expect(await screen.findByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
    expect(screen.getByRole('radio', { name: '30 Tage' })).toBeChecked();
    expect(askedDays(fetchMock)).toEqual(['30']);

    const top = await screen.findByRole('table', { name: 'Meistgesehen' });
    expect(
      within(top)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Titel', 'Art', 'Wiedergaben', 'Schauzeit']);
    const heat = within(top).getByRole('link', { name: 'Heat (1995)' });
    expect(heat).toHaveAttribute('href', `/admin/items/${ids.heat}`);
    expect(
      within(heat.closest('tr')!)
        .getAllByRole('cell')
        .map((cell) => cell.textContent),
    ).toEqual(['Film', '3', '5 h']);
    expect(within(top).getByRole('link', { name: 'Pilot' }).closest('tr')).toHaveTextContent(
      'Folge145 min',
    );

    const profiles = screen.getByRole('region', { name: 'Schauzeit je Profil' });
    expect(within(profiles).getByRole('figure')).toHaveTextContent(
      'Schauzeit je Profil als Diagramm',
    );
    const table = within(profiles).getByRole('table', { name: 'Schauzeit je Profil' });
    const lena = within(table).getByRole('rowheader', { name: /^Lena/ }).closest('tr')!;
    expect(lena).toHaveTextContent('gast@example.test');
    expect(lena).toHaveTextContent('145 min');

    const playback = screen.getByRole('region', { name: 'Wiedergabe' });
    const tiles = within(playback)
      .getAllByRole('term')
      .map((term) => [term.textContent, term.parentElement?.textContent]);
    expect(tiles.map(([label]) => label)).toEqual([
      'Direct Play',
      'Remux',
      'Transcode',
      'Transcode auf dem Grafikchip',
    ]);
    expect(tiles[0]?.[1]).toMatch(/2 Wiedergaben50\s%\saller Wiedergaben$/);
    expect(tiles[2]?.[1]).toMatch(/1 Wiedergabe25\s%\saller Wiedergaben$/);
    expect(tiles[3]?.[1]).toMatch(/100\s%Anteil der Transcodes$/);

    const devices = screen.getByRole('region', { name: 'Geräte' });
    const deviceTable = within(devices).getByRole('table', { name: 'Geräte' });
    expect(
      within(deviceTable).getByRole('rowheader', { name: 'Computer' }).closest('tr'),
    ).toHaveTextContent('Computer25 h');
    expect(within(deviceTable).getByRole('rowheader', { name: 'Unbekannt' })).toBeVisible();
  });

  it('switches the period with the arrow keys and keeps it in the address', async () => {
    const { fetchMock } = mockApi(routes);
    const { router } = renderApp('/admin/stats');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Meistgesehen' });
    const thirty = screen.getByRole('radio', { name: '30 Tage' });
    thirty.focus();
    await user.keyboard('{ArrowLeft}');
    await waitFor(() => expect(screen.getByRole('radio', { name: '7 Tage' })).toBeChecked());
    await waitFor(() => expect(router.state.location.search).toEqual({ days: 7 }));
    await waitFor(() => expect(askedDays(fetchMock)).toEqual(['30', '7']));
    await user.keyboard('{ArrowRight}{ArrowRight}{ArrowRight}');
    await waitFor(() => expect(router.state.location.search).toEqual({ days: 365 }));
    expect(screen.getByRole('radio', { name: '365 Tage' })).toHaveFocus();
  });

  it('takes the period from the address and ignores one it does not offer', async () => {
    const first = mockApi(routes);
    const view = renderApp('/admin/stats?days=90');
    expect(await screen.findByRole('radio', { name: '90 Tage' })).toBeChecked();
    await waitFor(() => expect(askedDays(first.fetchMock)).toEqual(['90']));
    view.unmount();
    vi.unstubAllGlobals();

    const second = mockApi(routes);
    renderApp('/admin/stats?days=12');
    expect(await screen.findByRole('radio', { name: '30 Tage' })).toBeChecked();
    await waitFor(() => expect(askedDays(second.fetchMock)).toEqual(['30']));
  });

  it('says when nothing was watched', async () => {
    mockApi({ ...signedIn, 'GET /admin/stats': () => json(empty) });
    renderApp('/admin/stats');
    expect(await screen.findByText('In diesem Zeitraum wurde nichts geschaut.')).toBeVisible();
    expect(screen.getByText('Kein Profil hat in diesem Zeitraum geschaut.')).toBeVisible();
    expect(screen.getByText('Kein Gerät hat in diesem Zeitraum geschaut.')).toBeVisible();
    const playback = screen.getByRole('region', { name: 'Wiedergabe' });
    expect(playback).toHaveTextContent(/^Wiedergabe.*0 Wiedergaben0\s%\saller Wiedergaben/);
    expect(screen.queryByRole('figure')).toBeNull();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, isPrimary: false } }),
    });
    renderApp('/admin/stats');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
