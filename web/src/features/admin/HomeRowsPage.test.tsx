import type { HomeRowSetting, TraktStatus } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

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

describe('admin start page rows (M7, E-9)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('moves and hides rows and saves all eight at once', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'PUT /admin/home-rows': (call) => json(call.body),
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

  it('leaves the Trakt status to the spotlight page and links there in one line', async () => {
    // The status endpoint still answers, so a status table would have data to show.
    const { calls } = mockApi({
      ...signedIn,
      'GET /admin/home-rows': () => json(defaultRows),
      'GET /admin/trakt': () => json(configured),
    });
    renderApp('/admin/home-rows');

    await screen.findByRole('table', { name: 'Reihen der Startseite' });
    const link = screen.getByRole('link', { name: 'Zur Seite «Spotlight»' });
    expect(link).toHaveAttribute('href', '/admin/spotlight');
    expect(link.parentElement).toHaveTextContent(/Welche Trakt-Liste Spotlight zeigt/);
    expect(screen.queryByRole('region', { name: 'Trakt' })).toBeNull();
    expect(screen.queryAllByRole('term')).toHaveLength(0);
    expect(screen.queryByText('Letzter erfolgreicher Abruf')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Jetzt aktualisieren' })).toBeNull();
    expect(calls.some((call) => call.path === '/admin/trakt')).toBe(false);
  });
});
