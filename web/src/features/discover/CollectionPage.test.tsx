import type { Collection, ItemRef } from '@slouchcat/api-client';
import { fireEvent, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, heat, ids, podcast, seriesIds } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const id = '01JB8Z6Q4M7N9P2R3S5T7V9C01';
const collection: Collection = {
  id,
  name: 'Alien-Reihe',
  overview: 'Die Filme um die Nostromo und ihre Nachfolger.',
  items: [alien, heat],
  total: 3,
  offset: 0,
  limit: 50,
};

describe('collection page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows name, description and the visible members, and reads the rest without «Mehr laden»', async () => {
    // 101 members: page 0 holds 100 (concept 18 C-3), the last one comes with page 1.
    const fillers = Array.from({ length: 98 }, (_, index) => ({
      ...heat,
      id: `${ids.heat.slice(0, -2)}${String(index).padStart(2, '0')}`,
      title: `Heat ${index}`,
    }));
    const { calls } = mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        calls.filter((call) => call.path === `/collections/${id}`).length > 1
          ? json({ ...collection, items: [arrival], total: 101, offset: 100, limit: 100 })
          : json({ ...collection, items: [alien, heat, ...fillers], total: 101, limit: 100 }),
    });
    renderApp(`/collections/${id}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien-Reihe' })).toBeVisible();
    expect(screen.getByText('Die Filme um die Nostromo und ihre Nachfolger.')).toBeVisible();
    expect(screen.getByText('101 Titel')).toBeVisible();
    // The head of a detail page (concept 17 E-16): the back arrow, the first member's cover.
    expect(screen.getByRole('button', { name: 'Zurück' })).toBeVisible();
    expect(
      document.querySelector(`img[src="/api/v1/items/${ids.alien}/images/poster/w500?v=abc"]`),
    ).not.toBeNull();
    const members = screen.getByRole('region', { name: 'Titel' });
    const alienCard = within(members).getAllByRole('link', { name: /Alien/ })[0]!;
    expect(alienCard).toHaveAttribute('href', `/items/${ids.alien}`);
    // The watch state on the cards.
    expect(within(alienCard).getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    expect(document.title).toBe('Alien-Reihe · Slouchcat');

    expect(await within(members).findByRole('link', { name: /Arrival/ })).toBeVisible();
    expect(calls.filter((call) => call.path === `/collections/${id}`)).toHaveLength(2);
    expect(within(members).queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument();
  });

  it("puts the collection poster on the cover; a failing file gives the first member's poster back", async () => {
    mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        json({
          ...collection,
          image: { url: `/collections/${id}/image?v=a&p=1`, width: 400, height: 600 },
        }),
    });
    renderApp(`/collections/${id}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien-Reihe' });
    // The cover of the head (the members below carry the same posters on their cards).
    const cover = () => document.querySelector('img[class*="posterImage"]');
    expect(cover()?.getAttribute('src')).toContain(`/collections/${id}/image?v=a&p=1`);
    const own = cover();
    if (own) fireEvent.error(own);
    expect(cover()?.getAttribute('src')).toBe(
      `/api/v1/items/${ids.alien}/images/poster/w500?v=abc`,
    );
  });

  it('checks a watched series and a heard podcast, not an open series (contract 1.22.0)', async () => {
    const series = (seriesId: string, title: string, watched?: boolean): ItemRef => ({
      id: seriesId,
      kind: 'series',
      title,
      inWatchlist: false,
      images: {},
      ...(watched === undefined ? {} : { watched }),
    });
    mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        json({
          ...collection,
          items: [
            series(seriesIds.show, 'Ganz gesehen', true),
            series(seriesIds.season1, 'Halb gesehen', false),
            series(seriesIds.specials, 'Ohne Folgen'),
            { ...podcast, watched: true },
          ],
          total: 4,
        }),
    });
    renderApp(`/collections/${id}`);
    const members = await screen.findByRole('region', { name: 'Titel' });
    const card = (name: RegExp) => within(members).getByRole('link', { name });
    expect(within(card(/Ganz gesehen/)).getByRole('img', { name: 'Gesehen' })).toBeVisible();
    expect(within(card(/Halb gesehen/)).queryByRole('img', { name: 'Gesehen' })).toBeNull();
    expect(within(card(/Ohne Folgen/)).queryByRole('img', { name: 'Gesehen' })).toBeNull();
    expect(within(card(/Test Podcast/)).getByRole('img', { name: 'Gehört' })).toBeVisible();
  });

  it('says "nicht gefunden" for an unknown or invisible collection', async () => {
    mockApi({ ...signedIn, 'GET /collections/*': () => problem(404, 'not_found') });
    renderApp(`/collections/${id}`);
    expect(await screen.findByText('Sammlung nicht gefunden')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Startseite' })).toHaveAttribute('href', '/');
  });

  it('drops a cached collection once a refetch finds it gone', async () => {
    let deleted = false;
    mockApi({
      ...signedIn,
      'GET /collections/*': () => (deleted ? problem(404, 'not_found') : json(collection)),
    });
    const { queryClient } = renderApp(`/collections/${id}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien-Reihe' })).toBeVisible();

    deleted = true;
    await queryClient.refetchQueries({ queryKey: ['collection', id] });
    expect(await screen.findByText('Sammlung nicht gefunden')).toBeVisible();
    expect(
      screen.queryByRole('heading', { level: 1, name: 'Alien-Reihe' }),
    ).not.toBeInTheDocument();
    expect(document.title).toBe('Sammlung nicht gefunden · Slouchcat');
  });
});
