import type { HomeRowSetting, TraktStatus } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const keys = [
  'spotlight',
  'picks',
  'continue',
  'recent',
  'released',
  'watchlist',
  'collections',
  'genres',
] as const;
const defaultRows: HomeRowSetting[] = keys.map((key, position) => ({
  key,
  position,
  hidden: false,
}));
const configured: TraktStatus = {
  configured: true,
  lastSuccess: '2026-10-20T06:00:00Z',
  lastAttempt: '2026-10-20T06:00:00Z',
  listKey: 'trending',
  entries: 40,
};

const rowNames = () =>
  within(screen.getByRole('table', { name: 'Reihen der Startseite' }))
    .getAllByRole('rowheader')
    .map((cell) => cell.textContent);

describe('admin start page rows (M7, E-9) and Trakt (E-7)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('moves and hides rows and saves all eight at once', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'PUT /admin/home-rows': (call) => json(call.body),
      'GET /admin/trakt': () => json(configured),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();

    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    expect(rowNames()).toEqual([
      'Spotlight',
      "Cat's Empfehlungen",
      'Weiterschauen',
      'Neu hinzugefügt',
      'Kürzlich veröffentlicht',
      'Merkliste',
      'Sammlungen',
      'Genres',
    ]);
    expect(screen.getByRole('button', { name: 'Spotlight nach oben' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Genres nach unten' })).toBeDisabled();

    await user.click(screen.getByRole('button', { name: 'Neu hinzugefügt nach oben' }));
    expect(rowNames().slice(1, 4)).toEqual([
      "Cat's Empfehlungen",
      'Neu hinzugefügt',
      'Weiterschauen',
    ]);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Neu hinzugefügt nach oben' })).toHaveFocus(),
    );
    expect(screen.getByText(/Neu hinzugefügt steht jetzt an Stelle 3/)).toBeVisible();
    await user.click(screen.getByRole('checkbox', { name: 'Genres ausblenden' }));
    // Nothing is stored before "Speichern".
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Die Startseite ist gespeichert.')).toBeVisible();
    const body = calls.find((call) => call.method === 'PUT')?.body as HomeRowSetting[];
    expect(body.map((row) => row.key)).toEqual([
      'spotlight',
      'picks',
      'recent',
      'continue',
      'released',
      'watchlist',
      'collections',
      'genres',
    ]);
    expect(body.map((row) => row.position)).toEqual([0, 1, 2, 3, 4, 5, 6, 7]);
    expect(body.find((row) => row.key === 'genres')?.hidden).toBe(true);
    expect(body.filter((row) => row.hidden)).toHaveLength(1);
    expect(
      screen.getByRole('navigation', { name: 'Verwaltung' }).querySelector('[aria-current="page"]'),
    ).toHaveTextContent('Startseite');
  });

  it('shows the Trakt state and queues a refresh', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'GET /admin/trakt': () =>
        json({ ...configured, lastError: 'trakt_unavailable: HTTP 503' } satisfies TraktStatus),
      'POST /admin/spotlight/refresh': () => new Response(null, { status: 202 }),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();

    const section = await screen.findByRole('region', { name: 'Trakt' });
    expect(await within(section).findByText('Ja')).toBeVisible();
    expect(within(section).getByText('Trending')).toBeVisible();
    expect(within(section).getByText('40')).toBeVisible();
    expect(within(section).getByText('trakt_unavailable: HTTP 503')).toBeVisible();
    await user.click(within(section).getByRole('button', { name: 'Jetzt aktualisieren' }));
    expect(await within(section).findByText('Die Trakt-Listen werden abgerufen.')).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/spotlight/refresh')).toBe(true);
  });

  it('without a client id the refresh is refused with a notice', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'GET /admin/trakt': () =>
        json({ configured: false, listKey: 'trending', entries: 0 } satisfies TraktStatus),
      'POST /admin/spotlight/refresh': () => problem(422, 'provider_not_configured'),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();

    const section = await screen.findByRole('region', { name: 'Trakt' });
    expect(await within(section).findByText(/Trakt ist nicht eingerichtet/)).toBeVisible();
    expect(within(section).getByText('Nein')).toBeVisible();
    expect(within(section).getAllByText('Noch nie')).toHaveLength(2);
    await user.click(within(section).getByRole('button', { name: 'Jetzt aktualisieren' }));
    expect(
      await within(section).findByText(/Dieser Anbieter ist nicht eingerichtet/),
    ).toBeVisible();
  });

  it('a refresh within a minute of the last one shows the rate-limit notice', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'GET /admin/trakt': () => json(configured),
      'POST /admin/spotlight/refresh': () => problem(429, 'too_many_requests'),
    });
    renderApp('/admin/home-rows');
    const user = userEvent.setup();

    const section = await screen.findByRole('region', { name: 'Trakt' });
    await user.click(await within(section).findByRole('button', { name: 'Jetzt aktualisieren' }));
    expect(
      await within(section).findByText(/vor weniger als einer Minute abgerufen/),
    ).toBeVisible();
  });
});
