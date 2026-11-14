import type { CollectionRef } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { alien } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

const collection = (
  id: string,
  name: string,
  count: number,
  extra: Partial<CollectionRef> = {},
): CollectionRef => ({
  id: `01JB8Z6Q4M7N9P2R3S5T7V9C${id}`,
  name,
  count,
  cover: alien,
  seenCount: 0,
  started: false,
  ...extra,
});

const list = [
  collection('01', 'Alien-Reihe', 4, {
    firstYear: 1979,
    lastYear: 1997,
    seenCount: 1,
    started: true,
  }),
  collection('02', 'Bond', 25, { firstYear: 1962, lastYear: 2021 }),
  collection('03', 'Einzelstück', 1, { firstYear: 2004, lastYear: 2004 }),
  collection('04', 'Zurück in die Zukunft', 3, { firstYear: 1985, lastYear: 1990 }),
];

const names = () =>
  within(screen.getByRole('list', { name: 'Sammlungen' }))
    .getAllByRole('link')
    .map((link) => link.querySelector('[class*="title"]')?.textContent);

describe('the page «Sammlungen» (concept 21 P-10)', () => {
  it('shows every collection by name with stack, years and the bar of a started one', async () => {
    mockApi({ ...signedIn, 'GET /collections': () => json(list) });
    renderApp('/collections');
    expect(await screen.findByText('4 Sammlungen')).toBeVisible();
    expect(names()).toEqual(['Alien-Reihe', 'Bond', 'Einzelstück', 'Zurück in die Zukunft']);
    const alienCard = screen.getByRole('link', { name: /Alien-Reihe/ });
    expect(alienCard).toHaveTextContent('4 Titel · 1979–1997');
    expect(alienCard).toHaveTextContent('4');
    expect(within(alienCard).getByRole('progressbar', { name: '1 von 4 gesehen' })).toBeVisible();
    expect(
      within(screen.getByRole('link', { name: /Bond/ })).queryByRole('progressbar'),
    ).toBeNull();
    expect(screen.getByRole('link', { name: /Einzelstück/ })).toHaveTextContent('1 Titel · 2004');
    // «Alle» is chosen; there is no random button (O-17).
    expect(screen.getByRole('button', { name: 'Alle' })).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByRole('button', { name: 'Name A–Z' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(screen.queryByRole('button', { name: /Zufall/ })).toBeNull();
  });

  it('filters by size, sorts, and keeps both in the address', async () => {
    mockApi({ ...signedIn, 'GET /collections': () => json(list) });
    const { router } = renderApp('/collections');
    await screen.findByText('4 Sammlungen');
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Ab 3' }));
    expect(names()).toEqual(['Alien-Reihe', 'Bond', 'Zurück in die Zukunft']);
    expect(screen.getByText('3 von 4 Sammlungen')).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Meiste Titel' }));
    expect(names()).toEqual(['Bond', 'Alien-Reihe', 'Zurück in die Zukunft']);
    await waitFor(() => expect(router.state.location.search).toEqual({ min: 3, sort: 'count' }));
    await user.click(screen.getByRole('button', { name: 'Angefangen zuerst' }));
    expect(names()).toEqual(['Alien-Reihe', 'Bond', 'Zurück in die Zukunft']);
    await user.click(screen.getByRole('button', { name: 'Alle' }));
    await user.click(screen.getByRole('button', { name: 'Name A–Z' }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('searches the names at once and writes the text into the address', async () => {
    mockApi({ ...signedIn, 'GET /collections': () => json(list) });
    const { router } = renderApp('/collections?sort=count');
    await screen.findByText('4 Sammlungen');
    const user = userEvent.setup();
    await user.type(screen.getByRole('searchbox', { name: 'Sammlung suchen' }), 'zuk');
    expect(names()).toEqual(['Zurück in die Zukunft']);
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'zuk', sort: 'count' }));
    await user.clear(screen.getByRole('searchbox', { name: 'Sammlung suchen' }));
    await user.type(screen.getByRole('searchbox', { name: 'Sammlung suchen' }), 'nichts');
    expect(await screen.findByText('Keine Sammlung passt zu Suche und Filter.')).toBeVisible();
  });

  it('opens with the view of the address', async () => {
    mockApi({ ...signedIn, 'GET /collections': () => json(list) });
    renderApp('/collections?min=5&q=o');
    expect(await screen.findByText('1 von 4 Sammlungen')).toBeVisible();
    expect(names()).toEqual(['Bond']);
    expect(screen.getByRole('searchbox', { name: 'Sammlung suchen' })).toHaveValue('o');
    expect(screen.getByRole('button', { name: 'Ab 5' })).toHaveAttribute('aria-pressed', 'true');
  });
});
