import type { ItemRef, WatchedEntry, WatchedPage } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { audiobook, heat, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

const at = '2026-11-08T20:00:00Z';
const watchedHeat: ItemRef = {
  ...heat,
  progress: {
    positionMs: 10_200_000,
    durationMs: 10_200_000,
    percent: 100,
    completed: true,
    updatedAt: at,
    watchedAt: at,
  },
};
const heardBook: ItemRef = { ...audiobook, progress: watchedHeat.progress };
const show: ItemRef = {
  id: '77777777-7777-4777-8777-777777777777',
  kind: 'series',
  title: 'Nordlicht',
  inWatchlist: false,
  images: {},
  watched: false,
  watchedCount: 19,
};

const pageOf = (items: WatchedEntry[]): WatchedPage => ({
  items,
  total: items.length,
  offset: 0,
  limit: 100,
});

describe('page «Gesehen»', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the titles with their dates and the series with its episodes, by kind and sort', async () => {
    const urls: URL[] = [];
    const { fetchMock } = mockApi({
      ...signedIn,
      'GET /watched': () => {
        // The query is not in the recorded path: the request's URL is the last call's.
        const url = new URL(fetchMock.mock.calls.at(-1)![0].url);
        urls.push(url);
        if (url.searchParams.get('content') === 'movie') {
          return json(pageOf([{ item: watchedHeat, lastWatchedAt: at }]));
        }
        return json(
          pageOf([
            { item: show, lastWatchedAt: '2026-11-09T20:00:00Z', episodeCount: 96 },
            { item: watchedHeat, lastWatchedAt: at },
            { item: heardBook, lastWatchedAt: at },
          ]),
        );
      },
    });
    renderApp('/watched');
    expect(await screen.findByRole('heading', { level: 1, name: 'Gesehen' })).toBeVisible();
    const list = await screen.findByRole('list', { name: 'Gesehen' });
    expect(screen.getByText('3 Titel')).toBeVisible();
    const nav = screen.getByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(nav).getByRole('link', { name: 'Gesehen' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(list).getByRole('link', { name: /Nordlicht/ })).toHaveTextContent(
      '19 von 96 Folgen · zuletzt 9. Nov.',
    );
    expect(
      within(list).getByRole('progressbar', { name: '19 von 96 Folgen gesehen' }),
    ).toBeVisible();
    expect(within(list).getByRole('link', { name: /Heat/ })).toHaveTextContent(
      'gesehen am 8. Nov.',
    );
    expect(within(list).getByRole('link', { name: new RegExp(audiobook.title) })).toHaveTextContent(
      'gehört am 8. Nov.',
    );
    expect(within(list).getByRole('button', { name: 'Gesehen: Nordlicht' })).toHaveAttribute(
      'aria-pressed',
      'mixed',
    );
    expect(within(list).getByRole('button', { name: 'Gesehen: Heat' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    expect(urls[0]?.searchParams.get('sort')).toBe('watched');
    expect(urls[0]?.searchParams.has('content')).toBe(false);

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Filme' }));
    await waitFor(() => expect(within(list).getAllByRole('link')).toHaveLength(1));
    expect(urls.at(-1)?.searchParams.get('content')).toBe('movie');
    await user.click(screen.getByRole('button', { name: 'Titel A–Z' }));
    await waitFor(() => expect(urls.at(-1)?.searchParams.get('sort')).toBe('title'));
    expect(urls.at(-1)?.searchParams.get('content')).toBe('movie');
  });

  it('removes a film by its check: the card leaves and the focus goes to the next', async () => {
    let entries: WatchedEntry[] = [
      { item: watchedHeat, lastWatchedAt: at },
      { item: heardBook, lastWatchedAt: at },
    ];
    const { calls } = mockApi({
      ...signedIn,
      'GET /watched': () => json(pageOf(entries)),
      'PUT /items/*/watched': () => {
        entries = entries.filter((entry) => entry.item.id !== ids.heat);
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/watched');
    const list = await screen.findByRole('list', { name: 'Gesehen' });
    await userEvent.setup().click(within(list).getByRole('button', { name: 'Gesehen: Heat' }));
    await waitFor(() =>
      expect(within(list).queryByRole('link', { name: /Heat/ })).not.toBeInTheDocument(),
    );
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
    expect(
      calls.find((call) => call.method === 'PUT' && call.path === `/items/${ids.heat}/watched`)
        ?.body,
    ).toEqual({ watched: false });
    await waitFor(() =>
      expect(within(list).getByRole('link', { name: new RegExp(audiobook.title) })).toHaveFocus(),
    );
  });

  it('says so when nothing is watched yet', async () => {
    mockApi({ ...signedIn, 'GET /watched': () => json(pageOf([])) });
    renderApp('/watched');
    expect(await screen.findByText('Noch nichts gesehen.')).toBeVisible();
    expect(screen.getByText('Was du zu Ende schaust oder hörst, erscheint hier.')).toBeVisible();
    expect(screen.queryByRole('list', { name: 'Gesehen' })).not.toBeInTheDocument();
  });
});
