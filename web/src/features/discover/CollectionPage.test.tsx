import type { Collection } from '@slouchcat/api-client';
import { fireEvent, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, heat, ids } from '@/test/fixtures';
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

  it('shows name, description and the visible members, and pages the rest', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        calls.filter((call) => call.path === `/collections/${id}`).length > 1
          ? json({ ...collection, items: [arrival], offset: 2 })
          : json(collection),
    });
    renderApp(`/collections/${id}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien-Reihe' })).toBeVisible();
    expect(screen.getByText('Die Filme um die Nostromo und ihre Nachfolger.')).toBeVisible();
    expect(screen.getByText('3 Titel')).toBeVisible();
    // The head of a detail page (concept 17 E-16): the back arrow, the first member's cover.
    expect(screen.getByRole('button', { name: 'Zurück' })).toBeVisible();
    expect(
      document.querySelector(`img[src="/api/v1/items/${ids.alien}/images/poster/w500?v=abc"]`),
    ).not.toBeNull();
    const members = screen.getByRole('region', { name: 'Titel' });
    const alienCard = within(members).getByRole('link', { name: /Alien/ });
    expect(alienCard).toHaveAttribute('href', `/items/${ids.alien}`);
    // The watch state on the cards.
    expect(within(alienCard).getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    expect(document.title).toBe('Alien-Reihe · Slouchcat');

    await userEvent.setup().click(within(members).getByRole('button', { name: 'Mehr laden' }));
    expect(await within(members).findByRole('link', { name: /Arrival/ })).toBeVisible();
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
