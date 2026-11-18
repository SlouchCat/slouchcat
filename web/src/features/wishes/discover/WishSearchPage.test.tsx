import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, session } from '@/test/api';

const rights = { enabled: true, canRequest: true, movies: true, shows: true, discover: true };
const base = {
  'GET /auth/session': () => json({ ...session, wishes: rights }),
  'GET /wishes': () => json({ items: [], total: 0, counts: { open: 0, available: 0, closed: 0 } }),
  'GET /wishes/quota': () => json({ unlimited: true }),
};
const page = (entries: unknown[]) =>
  json({ entries, page: 1, totalPages: 1, stale: false, fetchedAt: '2026-11-17T10:00:00Z' });

describe('«Suche» of the wishes area (concept 25, 5.4, M-34)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('searches the text of the URL and the field, says too short and nothing found', async () => {
    const { fetchMock } = mockApi({ ...base, 'GET /wishes/search': () => page([]) });
    const { router } = renderApp('/discover/search?q=tmdb:603');
    expect(await screen.findByText('Nichts gefunden für «tmdb:603»')).toBeVisible();
    const field = screen.getByRole('searchbox', { name: 'Film oder Serie suchen' });
    expect(field).toHaveValue('tmdb:603');
    const user = userEvent.setup();
    await user.clear(field);
    await user.type(field, 'D');
    expect(await screen.findByText('Gib mindestens zwei Zeichen ein.')).toBeVisible();
    await user.type(field, 'u');
    await waitFor(() => expect(router.state.location.search).toEqual({ q: 'Du' }));
    // Only texts of two characters and more go to the server.
    const asked = () =>
      fetchMock.mock.calls
        .map(([request]) => new URL(request.url))
        .filter((url) => url.pathname === '/api/v1/wishes/search')
        .map((url) => url.searchParams.get('q'));
    await waitFor(() => expect(asked().at(-1)).toBe('Du'));
    expect(asked()).not.toContain('D');
    expect(document.body.textContent).not.toContain('{{');
  });

  it('shows the hits as tiles to the preview page', async () => {
    mockApi({
      ...base,
      'GET /wishes/search': () =>
        page([
          {
            title: {
              media: 'show',
              tmdbId: 9100,
              title: 'Nebelhafen',
              year: 2023,
              images: {},
              hasTrailer: false,
              canRequest: true,
            },
            seerrRequested: false,
          },
        ]),
    });
    renderApp('/discover/search?q=Nebel');
    expect(await screen.findByRole('link', { name: /Nebelhafen/ })).toHaveAttribute(
      'href',
      '/titles/show/9100',
    );
    expect(screen.getByRole('button', { name: 'Nebelhafen anfordern' })).toBeVisible();
  });
});
