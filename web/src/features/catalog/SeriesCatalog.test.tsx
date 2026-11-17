import type { ItemDetail } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { json, mockApi, renderApp, signedIn } from '@/test/api';
import { season1Detail, seriesIds, showDetail } from '@/test/fixtures';

// Concept 25 stage 4 (M-06): the missing seasons of a series of the library, only where the
// server's answer carries the catalog (a profile that may request series, F-9).

const catalog: NonNullable<ItemDetail['catalog']> = {
  status: 'Returning Series',
  inProduction: true,
  present: 1,
  total: 3,
  seasons: [
    { number: 1, episodeCount: 3, episodes: 3, state: 'present', airDate: '2024-09-02' },
    { number: 2, episodeCount: 10, episodes: 0, state: 'missing', airDate: '2025-09-06' },
    { number: 3, episodeCount: 10, episodes: 0, state: 'requested', airDate: '2026-09-05' },
    { number: 4, episodeCount: 0, episodes: 0, state: 'announced' },
  ],
  nextEpisode: { season: 4, episode: 1, airDate: '2026-12-04' },
};

const withCatalog: ItemDetail = { ...showDetail, tmdbId: 9100, catalog, canRequestSeasons: true };

const wall = () => screen.getByRole('list', { name: 'Staffeln' });

describe('missing seasons on the series page', () => {
  it('shows nothing of them without a catalog in the answer', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () => json(showDetail),
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
    });
    renderApp(`/items/${seriesIds.show}`);
    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    expect(within(wall()).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.queryByText(/Staffeln vorhanden/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Staffeln anfordern' })).not.toBeInTheDocument();
  });

  it('lists them with their states and the line, and requests the pressed season', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () => json(withCatalog),
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
      'GET /titles/show/9100/seasons': () =>
        json({
          source: 'seerr',
          seasons: [
            { number: 1, episodeCount: 3, state: 'available' },
            { number: 2, episodeCount: 10, state: 'missing' },
            { number: 3, episodeCount: 10, state: 'requested' },
          ],
        }),
      'GET /wishes/quota': () => json({ unlimited: true, at: '2026-11-17T08:00:00Z' }),
      'POST /wishes': () =>
        json(
          {
            id: 7,
            media: 'show',
            tmdbId: 9100,
            title: 'Slouchcat Test Show',
            state: 'pending',
            origin: 'slouchcat',
            role: 'owner',
            seasons: [{ number: 2, state: 'pending' }],
            canWithdraw: true,
            canRequestAgain: false,
            createdAt: '2026-11-17T08:00:00Z',
            updatedAt: '2026-11-17T08:00:00Z',
            events: [],
          },
          201,
        ),
    });
    renderApp(`/items/${seriesIds.show}`);
    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    expect(
      screen.getByText(/^1 von 3 Staffeln vorhanden · Läuft noch, nächste Folge am/),
    ).toBeVisible();
    // Season 1, the three of the catalog, then the specials.
    const items = within(wall()).getAllByRole('listitem');
    expect(items.map((item) => item.querySelector('button')?.getAttribute('aria-label'))).toEqual([
      null,
      'Staffel 2 anfordern',
      'Staffel 3: bereits angefordert, mitwünschen',
      'Staffel 4: Angekündigt',
      null,
    ]);
    expect(items[0]).toHaveTextContent(/^Season 1/);
    expect(items[4]).toHaveTextContent(/^Specials/);
    expect(within(wall()).getByText('Fehlt')).toBeInTheDocument();
    expect(within(wall()).getByText('Angefordert')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Staffeln anfordern' })).toBeVisible();

    const user = userEvent.setup();
    await user.click(within(wall()).getByRole('button', { name: 'Staffel 2 anfordern' }));
    const dialog = await screen.findByRole('dialog', {
      name: 'Serie anfordern: Slouchcat Test Show',
    });
    expect(await within(dialog).findByRole('checkbox', { name: 'Staffel 2' })).toBeChecked();
    expect(within(dialog).getByRole('checkbox', { name: 'Staffel 3' })).not.toBeChecked();
    await user.click(within(dialog).getByRole('button', { name: '1 Staffel anfordern' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        media: 'show',
        tmdbId: 9100,
        source: 'detail',
        seasons: [2],
      }),
    );
    expect(await screen.findByText(/^Angefordert. Den Stand siehst du/)).toBeVisible();
    expect(document.body.textContent).not.toContain('{{');
  });

  it('shows the seasons without the request where the server does not allow it', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () => json({ ...withCatalog, canRequestSeasons: false }),
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
    });
    renderApp(`/items/${seriesIds.show}`);
    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    expect(within(wall()).getByRole('button', { name: 'Staffel 2: Fehlt' })).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Staffeln anfordern' })).not.toBeInTheDocument();
  });
});
