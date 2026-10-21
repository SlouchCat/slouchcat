import type { ItemPage, ItemRef } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, heat, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

const page = (items: ItemRef[]): ItemPage => ({ items, total: items.length, offset: 0, limit: 50 });

describe('watchlist page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the titles and removes one with its button', async () => {
    let items = [heat, alien];
    const { calls } = mockApi({
      ...signedIn,
      'GET /watchlist': () => json(page(items)),
      'DELETE /items/*/watchlist': () => {
        items = items.filter((item) => item.id !== ids.heat);
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/watchlist');
    expect(await screen.findByRole('heading', { level: 1, name: 'Merkliste' })).toBeVisible();
    const list = await screen.findByRole('list', { name: 'Merkliste' });
    expect(within(list).getByRole('link', { name: /Heat/ })).toHaveAttribute(
      'href',
      `/items/${ids.heat}`,
    );
    const nav = screen.getByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(nav).getByRole('link', { name: 'Merkliste' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(calls.find((call) => call.path === '/watchlist')).toBeDefined();

    await userEvent
      .setup()
      .click(within(list).getByRole('button', { name: 'Heat von der Merkliste entfernen' }));
    await waitFor(() =>
      expect(within(list).queryByRole('link', { name: /Heat/ })).not.toBeInTheDocument(),
    );
    expect(
      calls.some((c) => c.method === 'DELETE' && c.path === `/items/${ids.heat}/watchlist`),
    ).toBe(true);
    expect(within(list).getByRole('link', { name: /Alien/ })).toBeVisible();
  });

  it('shows an empty state with the way to the search', async () => {
    mockApi({ ...signedIn, 'GET /watchlist': () => json(page([])) });
    renderApp('/watchlist');
    expect(await screen.findByText('Deine Merkliste ist leer')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Suche' })).toHaveAttribute('href', '/search');
  });
});
