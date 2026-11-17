import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, session } from '@/test/api';

const rights = { enabled: true, canRequest: true, movies: true, shows: true, discover: true };
const entry = (tmdbId: number, title: string) => ({
  title: {
    media: 'movie',
    tmdbId,
    title,
    year: 2026,
    images: {},
    hasTrailer: false,
    canRequest: true,
  },
  seerrRequested: false,
});

describe('«Filme» (concept 25, 5.4, M-32)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('reads the view from the URL, says the filter on its button and changes the URL', async () => {
    const { fetchMock } = mockApi({
      'GET /auth/session': () => json({ ...session, wishes: rights }),
      'GET /wishes': () =>
        json({ items: [], total: 0, counts: { open: 0, available: 0, closed: 0 } }),
      'GET /wishes/quota': () => json({ unlimited: true }),
      'GET /wishes/genres/movie': () => json({ items: [{ id: 18, name: 'Drama' }] }),
      'GET /wishes/titles/movie': () =>
        json({
          entries: [entry(9001, 'Glasvogel')],
          page: 1,
          totalPages: 1,
          stale: false,
          fetchedAt: '2026-11-17T10:00:00Z',
        }),
    });
    const { router } = renderApp('/discover/movies?genre=18&sort=newest');
    expect(await screen.findByRole('heading', { level: 1, name: 'Filme' })).toBeVisible();
    expect(await screen.findByRole('link', { name: /Glasvogel/ })).toHaveAttribute(
      'href',
      '/titles/movie/9001',
    );
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .find((url) => url.pathname === '/api/v1/wishes/titles/movie');
    expect(asked?.searchParams.get('genre')).toBe('18');
    expect(asked?.searchParams.get('sort')).toBe('newest');
    expect(asked?.searchParams.get('page')).toBe('1');

    // The band is closed and says its choice.
    const filter = await screen.findByRole('button', {
      name: 'Filter: Drama · Vorhandene sichtbar',
    });
    expect(filter).toHaveAttribute('aria-expanded', 'false');
    const user = userEvent.setup();
    await user.click(filter);
    await user.click(screen.getByRole('button', { name: 'Vorhandene ausblenden' }));
    await waitFor(() =>
      expect(router.state.location.search).toEqual({ genre: 18, sort: 'newest', hide: true }),
    );
    await user.click(screen.getByRole('button', { name: 'Filter zurücksetzen' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ sort: 'newest' }));
    expect(document.body.textContent).not.toContain('{{');
  });
});
