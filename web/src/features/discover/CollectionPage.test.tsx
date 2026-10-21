import type { Collection } from '@slouchcat/api-client';
import { screen, within } from '@testing-library/react';
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
    const members = screen.getByRole('region', { name: 'Titel' });
    expect(within(members).getByRole('link', { name: /Alien/ })).toHaveAttribute(
      'href',
      `/items/${ids.alien}`,
    );
    expect(document.title).toBe('Alien-Reihe · Slouchcat');

    await userEvent.setup().click(within(members).getByRole('button', { name: 'Mehr laden' }));
    expect(await within(members).findByRole('link', { name: /Arrival/ })).toBeVisible();
    expect(within(members).queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument();
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
