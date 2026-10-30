import type { components, ItemRef } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, heat, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

type PicksPage = components['schemas']['PicksPage'];

const note = 'Unbedingt im Kino-Modus schauen.';

const page = (items: ItemRef[], total = items.length, offset = 0): PicksPage => ({
  items,
  total,
  offset,
  limit: 50,
});

describe("Cat's recommendations page", () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the picks in order with the note as caption', async () => {
    mockApi({ ...signedIn, 'GET /picks': () => json(page([{ ...heat, note }, alien])) });
    renderApp('/picks');
    expect(
      await screen.findByRole('heading', { level: 1, name: "Cat's Empfehlungen" }),
    ).toBeVisible();
    expect(await screen.findByText('2 Titel')).toBeVisible();
    expect(
      screen.getByText('Titel, die Cat für dich ausgesucht hat, in Cats Reihenfolge.'),
    ).toBeVisible();
    const grid = await screen.findByRole('list', { name: "Cat's Empfehlungen" });
    const cards = within(grid).getAllByRole('link');
    expect(cards.map((card) => card.getAttribute('href'))).toEqual([
      `/items/${ids.heat}`,
      `/items/${ids.alien}`,
    ]);
    expect(cards[0]).toHaveTextContent(note);
    // Without a note the year stands under the poster.
    expect(cards[1]).toHaveTextContent('1979');
    expect(screen.queryByRole('button', { name: /Mehr laden/ })).not.toBeInTheDocument();
  });

  it('loads more from 50 and names how many follow', async () => {
    const many = Array.from({ length: 50 }, (_, index) => ({
      ...heat,
      id: `01JB8Z6Q4M7N9P2R3S5T7V${String(index).padStart(4, '0')}`,
      title: `Heat ${index}`,
    }));
    const { calls } = mockApi({
      ...signedIn,
      'GET /picks': () =>
        json(
          calls.filter((call) => call.path === '/picks').length > 1
            ? page([alien], 51, 50)
            : page(many, 51),
        ),
    });
    renderApp('/picks');
    const more = await screen.findByRole('button', { name: 'Mehr laden (1 weitere)' });
    await userEvent.setup().click(more);
    const grid = screen.getByRole('list', { name: "Cat's Empfehlungen" });
    await waitFor(() => expect(within(grid).getAllByRole('link')).toHaveLength(51));
    expect(screen.queryByRole('button', { name: /Mehr laden/ })).not.toBeInTheDocument();
  });
});
