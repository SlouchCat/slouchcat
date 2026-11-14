import type { Collection, ItemRef } from '@slouchcat/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

  it('shows the head with facts and progress, and every part in order, read whole', async () => {
    // 101 parts: page 0 holds 100 (concept 18 C-3), the last one comes with page 1.
    const fillers = Array.from({ length: 98 }, (_, index) => ({
      ...heat,
      id: `${ids.heat.slice(0, -2)}${String(index).padStart(2, '0')}`,
      title: `Heat ${index}`,
    }));
    const facts = {
      firstYear: 1979,
      lastYear: 2016,
      runtimeMinutes: 1178,
      seenCount: 1,
      started: true,
    };
    const { calls } = mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        calls.filter((call) => call.path === `/collections/${id}`).length > 1
          ? json({ ...collection, ...facts, items: [arrival], total: 101, offset: 100, limit: 100 })
          : json({
              ...collection,
              ...facts,
              items: [alien, heat, ...fillers],
              total: 101,
              limit: 100,
              next: alien,
              nextPart: 1,
            }),
    });
    renderApp(`/collections/${id}`);
    expect(await screen.findByRole('heading', { level: 1, name: 'Alien-Reihe' })).toBeVisible();
    expect(screen.getByText('Die Filme um die Nostromo und ihre Nachfolger.')).toBeVisible();
    // Every part a film, once the list is whole: «Filme».
    expect(await screen.findByText('101 Filme · 1979–2016 · zusammen 19 h 38 min')).toBeVisible();
    expect(screen.getByRole('progressbar', { name: '1 von 101 gesehen' })).toBeVisible();
    // Alien is begun: «Weiterschauen · Teil 1» resumes it.
    expect(screen.getByRole('link', { name: /Weiterschauen · Teil 1/ })).toHaveAttribute(
      'href',
      `/play/${ids.alien}`,
    );
    expect(screen.getByRole('button', { name: 'Zurück' })).toBeVisible();
    const parts = screen.getByRole('list', { name: 'In Reihenfolge' });
    const first = within(parts).getByRole('link', { name: 'Teil 1: Alien' });
    expect(first).toHaveAttribute('href', `/items/${ids.alien}`);
    // The next part carries its marker and its bar; the others none.
    expect(within(first).getByText('Weiterschauen')).toBeVisible();
    expect(within(first).getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    const second = within(parts).getByRole('link', { name: 'Teil 2: Heat' });
    expect(within(second).queryByText('Als Nächstes')).not.toBeInTheDocument();
    expect(within(parts).getAllByText('2 h 50 min').length).toBeGreaterThan(0);
    expect(await within(parts).findByRole('link', { name: 'Teil 101: Arrival' })).toBeVisible();
    expect(calls.filter((call) => call.path === `/collections/${id}`)).toHaveLength(2);
    expect(document.title).toBe('Alien-Reihe · Slouchcat');
  });

  it('plays the next part, «Als Nächstes» on it, and the first again when all are finished', async () => {
    const done = {
      ...alien,
      progress: {
        ...alien.progress!,
        completed: true,
        percent: 100,
        watchedAt: '2026-10-18T19:00:00Z',
      },
    };
    let all = false;
    mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        json(
          all
            ? {
                ...collection,
                items: [done, { ...done, id: ids.heat, title: 'Heat' }],
                total: 2,
                next: done,
                nextPart: 1,
                seenCount: 2,
                started: true,
              }
            : {
                ...collection,
                items: [done, heat],
                total: 2,
                next: heat,
                nextPart: 2,
                seenCount: 1,
                started: true,
              },
        ),
    });
    const { queryClient } = renderApp(`/collections/${id}`);
    const play = await screen.findByRole('link', { name: /Abspielen · Teil 2/ });
    expect(play).toHaveAttribute('href', `/play/${ids.heat}`);
    const parts = screen.getByRole('list', { name: 'In Reihenfolge' });
    expect(
      within(within(parts).getByRole('link', { name: 'Teil 2: Heat' })).getByText('Als Nächstes'),
    ).toBeVisible();
    all = true;
    await queryClient.refetchQueries({ queryKey: ['collection', id] });
    expect(await screen.findByRole('link', { name: /Nochmals ansehen · Teil 1/ })).toHaveAttribute(
      'href',
      `/play/${ids.alien}?at=0`,
    );
    expect(within(parts).queryByText('Als Nächstes')).not.toBeInTheDocument();
  });

  it('«Gesehen» marks every part at once and asks before it takes the marks back (O-19)', async () => {
    let seen = 0;
    const { calls } = mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        json({
          ...collection,
          items: [alien, heat],
          total: 2,
          seenCount: seen,
          started: seen > 0,
          next: alien,
          nextPart: 1,
        }),
      [`PUT /collections/${id}/watched`]: (call) => {
        seen = (call.body as { watched: boolean }).watched ? 2 : 0;
        return new Response(null, { status: 204 });
      },
    });
    renderApp(`/collections/${id}`);
    const user = userEvent.setup();
    const button = await screen.findByRole('button', { name: 'Gesehen' });
    expect(button).toHaveAttribute('aria-pressed', 'false');
    await user.click(button);
    const puts = () => calls.filter((call) => call.method === 'PUT');
    await waitFor(() => expect(puts()).toHaveLength(1));
    expect(puts()[0]?.body).toEqual({ watched: true });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Gesehen' })).toHaveAttribute(
        'aria-pressed',
        'true',
      ),
    );
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Gesehen' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent('«Alien-Reihe» als ungesehen markieren?');
    expect(dialog).toHaveTextContent('Bei 2 Titeln fällt der Haken.');
    await user.click(screen.getByRole('button', { name: 'Alle Haken entfernen' }));
    await waitFor(() => expect(puts()).toHaveLength(2));
    expect(puts()[1]?.body).toEqual({ watched: false });
  });

  it('«Alle merken» puts the parts not yet on the watchlist there, the fallback is said', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /collections/${id}`]: () =>
        json({
          ...collection,
          items: [{ ...alien, inWatchlist: true }, heat],
          total: 2,
          overview: 'The films of the Nostromo.',
          overviewLang: 'en',
          overviewFallback: true,
        }),
      'PUT /items/*/watchlist': () => new Response(null, { status: 204 }),
    });
    renderApp(`/collections/${id}`);
    expect(
      await screen.findByText('Beschreibung von TMDB, nur auf Englisch vorhanden'),
    ).toBeVisible();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Alle merken' }));
    await waitFor(() =>
      expect(calls.filter((call) => call.method === 'PUT').map((call) => call.path)).toEqual([
        `/items/${ids.heat}/watchlist`,
      ]),
    );
    expect(await screen.findByRole('button', { name: 'Alle gemerkt' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
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
    const members = await screen.findByRole('list', { name: 'In Reihenfolge' });
    // The check marks beside the cards (concept 20 P-5).
    const check = (title: string) =>
      within(members).getByRole('button', { name: `Gesehen: ${title}` });
    expect(check('Ganz gesehen')).toHaveAttribute('aria-pressed', 'true');
    expect(check('Halb gesehen')).toHaveAttribute('aria-pressed', 'false');
    expect(check('Ohne Folgen')).toHaveAttribute('aria-pressed', 'false');
    // Audio is heard (coordinator's decision of 12 November 2026).
    expect(
      within(members).getByRole('button', { name: 'Gehört: Slouchcat Test Podcast' }),
    ).toHaveAttribute('aria-pressed', 'true');
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
