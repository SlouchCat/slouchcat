import type { ItemRef } from '@slouchcat/api-client';
import { screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, episodes, heat, seriesIds, showSeriesRef } from '@/test/fixtures';
import { json, mockApi, renderApp, session, signedIn } from '@/test/api';

const page = (items: ItemRef[]) => ({ items, total: items.length, offset: 0, limit: 100 });

/** S4 · F7, started: 20 of 45 minutes left. */
const started: ItemRef = {
  ...episodes.e1,
  title: 'Donuts im Stau',
  seasonNumber: 4,
  episodeNumber: 7,
  seriesRef: showSeriesRef,
  progress: { ...episodes.e1.progress!, positionMs: 1_500_000, percent: 56 },
};

// The lists behind the row titles of the start page (concept 20 P-7).
describe('list behind a row title', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('«Weiterschauen» draws an episode as its series, like the row; its pen edits the series', async () => {
    mockApi({ ...signedIn, 'GET /home/rows/continue': () => json(page([started, alien])) });
    renderApp('/lists/continue');
    const grid = await screen.findByRole('list', { name: 'Weiterschauen' });
    const card = within(grid).getByRole('link', {
      name: 'Slouchcat Test Show, S4 · F7, Donuts im Stau, Noch 20 min',
    });
    expect(card).toHaveAttribute('href', `/items/${seriesIds.e1}`);
    expect(within(card).getByText('S4 · F7 · Noch 20 min')).toBeVisible();
    expect(within(card).queryByText('Donuts im Stau')).not.toBeInTheDocument();
    expect(card.querySelector('img')).toHaveAttribute(
      'src',
      `/api/v1/items/${seriesIds.show}/images/poster/w500?v=abc`,
    );
    expect(within(card).getByRole('progressbar', { name: '56 % gesehen' })).toBeVisible();
    // The pen edits the series, whose cover the card shows; the film keeps its own.
    expect(
      within(grid).getByRole('button', { name: 'Slouchcat Test Show: Metadaten bearbeiten' }),
    ).toBeVisible();
    const film = within(grid).getByRole('link', { name: /Alien/ });
    expect(within(film).getByText('Noch 1 h 15 min')).toBeVisible();
    expect(within(grid).getByRole('button', { name: 'Alien: Metadaten bearbeiten' })).toBeVisible();
    expect(screen.getByText('2 Titel')).toBeVisible();
  });

  it('offers only the kinds a list can hold', async () => {
    mockApi({
      ...signedIn,
      'GET /home/rows/*': () => json(page([heat])),
    });
    const chips = async () => {
      const group = await screen.findByRole('group', { name: 'Inhalt' });
      return within(group)
        .getAllByRole('button')
        .map((button) => button.textContent);
    };
    const first = renderApp('/lists/recent');
    expect(await chips()).toEqual(['Alle', 'Filme', 'Serien', 'Hörbücher', 'Podcasts']);
    first.unmount();
    const second = renderApp('/lists/released');
    expect(await chips()).toEqual(['Alle', 'Filme', 'Serien', 'Hörbücher']);
    second.unmount();
    renderApp('/lists/spotlight');
    expect(await chips()).toEqual(['Alle', 'Filme', 'Serien']);
  });

  it('shows no pens for an account that does not manage the library, nor for another profile', async () => {
    for (const other of [
      { ...session, account: { ...session.account, role: 'user' as const, isOwner: false } },
      { ...session, profile: { ...session.profile, id: 2, isPrimary: false } },
    ]) {
      mockApi({
        'GET /auth/session': () => json(other),
        'GET /home/rows/recent': () => json(page([heat, alien])),
      });
      const { unmount } = renderApp('/lists/recent');
      const grid = await screen.findByRole('list', { name: 'Neu hinzugefügt' });
      expect(within(grid).getAllByRole('link')).toHaveLength(2);
      expect(
        within(grid).queryByRole('button', { name: /Metadaten bearbeiten/ }),
      ).not.toBeInTheDocument();
      unmount();
    }
  });

  it('an unknown list says so', async () => {
    mockApi({ ...signedIn });
    renderApp('/lists/nope');
    expect(await screen.findByText('Liste nicht gefunden')).toBeVisible();
  });
});
