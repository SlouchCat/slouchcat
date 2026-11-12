import type { ItemRef } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockApi, problem } from '@/test/api';
import { CoverWatched, patchWatched, watchedState } from './watched';

const id = '55555555-5555-4555-8555-555555555555';
const other = '66666666-6666-4666-8666-666666666666';
const now = '2026-11-12T10:00:00.000Z';

const film: ItemRef = { id, kind: 'movie', title: 'Alien', inWatchlist: false, images: {} };
const watchedFilm: ItemRef = {
  ...film,
  progress: {
    positionMs: 7_000_000,
    durationMs: 7_000_000,
    percent: 100,
    completed: true,
    updatedAt: now,
    watchedAt: now,
  },
};
const series = (watched: boolean, watchedCount: number): ItemRef => ({
  id,
  kind: 'series',
  title: 'Nordlicht',
  inWatchlist: false,
  images: {},
  watched,
  watchedCount,
});

describe('watchedState', () => {
  it('reads watchedAt of a single title, not completed', () => {
    expect(watchedState(film)).toBe('none');
    expect(watchedState(watchedFilm)).toBe('full');
    // A second run: no longer completed, still watched.
    const again = { ...watchedFilm.progress!, completed: false, positionMs: 1000, percent: 1 };
    expect(watchedState({ ...film, progress: again })).toBe('full');
    // Completed by the 90 % rule before contract 1.31.0 without watchedAt: not watched.
    const old = { ...watchedFilm.progress! };
    delete old.watchedAt;
    expect(watchedState({ ...film, progress: old })).toBe('none');
  });

  it('gives a series, podcast and season three states (F-12)', () => {
    expect(watchedState(series(false, 0))).toBe('none');
    expect(watchedState(series(false, 19))).toBe('half');
    expect(watchedState(series(true, 96))).toBe('full');
    expect(watchedState({ ...series(false, 3), kind: 'podcast' })).toBe('half');
    const season = { ...film, kind: 'season' as const, childCount: 8 };
    expect(watchedState({ ...season, watchedCount: 0 })).toBe('none');
    expect(watchedState({ ...season, watchedCount: 3 })).toBe('half');
    expect(watchedState({ ...season, watchedCount: 8 })).toBe('full');
    expect(watchedState({ ...season, childCount: 0, watchedCount: 0 })).toBe('none');
  });
});

describe('patchWatched', () => {
  it('sets a single title as the server will: finished, watchedAt now', () => {
    const next = patchWatched({ items: [film, { ...film, id: other }] }, id, true, now) as {
      items: ItemRef[];
    };
    expect(next.items[0]?.progress).toEqual({
      positionMs: 0,
      durationMs: 0,
      percent: 100,
      completed: true,
      updatedAt: now,
      watchedAt: now,
    });
    expect(watchedState(next.items[0]!)).toBe('full');
  });

  it('removes a check: a finished title starts at 0, a second run keeps its position', () => {
    const finished = patchWatched(watchedFilm, id, false, now) as ItemRef;
    expect(finished.progress).toMatchObject({ completed: false, positionMs: 0, percent: 0 });
    expect(finished.progress?.watchedAt).toBeUndefined();
    const running = {
      ...watchedFilm,
      progress: { ...watchedFilm.progress!, completed: false, positionMs: 1000, percent: 1 },
    };
    const kept = patchWatched(running, id, false, now) as ItemRef;
    expect(kept.progress).toMatchObject({ completed: false, positionMs: 1000 });
    expect(kept.progress?.watchedAt).toBeUndefined();
  });

  it('fills or empties a series from any state, and leaves one in that state alone', () => {
    expect(watchedState(patchWatched(series(false, 19), id, true, now) as ItemRef)).toBe('full');
    expect(watchedState(patchWatched(series(false, 19), id, false, now) as ItemRef)).toBe('none');
    expect(watchedState(patchWatched(series(true, 96), id, false, now) as ItemRef)).toBe('none');
    expect(patchWatched(series(true, 96), id, true, now)).toBeUndefined();
    expect(patchWatched({ items: [film] }, other, true, now)).toBeUndefined();
  });
});

/** The check mark over a title held in the query cache, as a list page holds it. */
function Cover({ onRemoved }: { onRemoved?: () => void }) {
  const { data } = useQuery<ItemRef>({
    queryKey: ['item', id],
    queryFn: () => new Promise<ItemRef>(() => undefined),
    staleTime: Infinity,
  });
  return data ? <CoverWatched item={data} onRemoved={onRemoved} /> : null;
}

function renderCover(item: ItemRef, onRemoved?: () => void) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  queryClient.setQueryData(['item', id], item);
  render(
    <QueryClientProvider client={queryClient}>
      <Cover onRemoved={onRemoved} />
    </QueryClientProvider>,
  );
  return queryClient;
}

const putCalls = (calls: { method: string; path: string; body: unknown }[]) =>
  calls.filter((call) => call.method === 'PUT' && call.path === `/items/${id}/watched`);

describe('CoverWatched', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('sets a single title without a question and shows the full check at once', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watched': () => new Response(null, { status: 204 }),
    });
    renderCover(film);
    const button = screen.getByRole('button', { name: 'Gesehen: Alien' });
    expect(button).toHaveAttribute('aria-pressed', 'false');
    expect(button).toHaveAttribute('title', 'Als gesehen markieren');
    await userEvent.setup().click(button);
    expect(screen.getByRole('button', { name: 'Gesehen: Alien' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await waitFor(() => expect(putCalls(calls)).toHaveLength(1));
    expect(putCalls(calls)[0]?.body).toEqual({ watched: true });
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument();
  });

  it('removes the check of a single title without a question and calls onRemoved', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watched': () => new Response(null, { status: 204 }),
    });
    const onRemoved = vi.fn();
    renderCover(watchedFilm, onRemoved);
    await userEvent.setup().click(screen.getByRole('button', { name: 'Gesehen: Alien' }));
    await waitFor(() => expect(onRemoved).toHaveBeenCalledTimes(1));
    expect(putCalls(calls)[0]?.body).toEqual({ watched: false });
  });

  it('sets a whole series without a question', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watched': () => new Response(null, { status: 204 }),
    });
    renderCover(series(false, 0));
    await userEvent.setup().click(screen.getByRole('button', { name: 'Gesehen: Nordlicht' }));
    await waitFor(() => expect(putCalls(calls)).toHaveLength(1));
    expect(putCalls(calls)[0]?.body).toEqual({ watched: true });
  });

  it('asks before the check of a whole series falls, and Cancel does nothing', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watched': () => new Response(null, { status: 204 }),
    });
    renderCover(series(true, 19));
    const user = userEvent.setup();
    const button = screen.getByRole('button', { name: 'Gesehen: Nordlicht' });
    await user.click(button);
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent('«Nordlicht» als ungesehen markieren?');
    expect(dialog).toHaveTextContent('Bei 19 Folgen fällt der Haken.');
    await user.click(screen.getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    expect(putCalls(calls)).toHaveLength(0);

    await user.click(button);
    await user.click(await screen.findByRole('button', { name: 'Alle Haken entfernen' }));
    await waitFor(() => expect(putCalls(calls)).toHaveLength(1));
    expect(putCalls(calls)[0]?.body).toEqual({ watched: false });
  });

  it('asks on a half check: mark all or remove all', async () => {
    const { calls } = mockApi({
      'PUT /items/*/watched': () => new Response(null, { status: 204 }),
    });
    renderCover(series(false, 19));
    const user = userEvent.setup();
    const button = screen.getByRole('button', { name: 'Gesehen: Nordlicht' });
    expect(button).toHaveAttribute('aria-pressed', 'mixed');
    await user.click(button);
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent('«Nordlicht» ist teilweise gesehen');
    expect(dialog).toHaveTextContent('19 Folgen tragen den Haken.');
    await user.click(screen.getByRole('button', { name: 'Alle Folgen als gesehen markieren' }));
    await waitFor(() => expect(putCalls(calls)).toHaveLength(1));
    expect(putCalls(calls)[0]?.body).toEqual({ watched: true });
    expect(screen.getByRole('button', { name: 'Gesehen: Nordlicht' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
  });

  it('turns back and says why when the server refuses', async () => {
    mockApi({ 'PUT /items/*/watched': () => problem(404, 'not_found') });
    renderCover(film);
    await userEvent.setup().click(screen.getByRole('button', { name: 'Gesehen: Alien' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Gesehen: Alien' })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
  });

  it('is a display only offline', () => {
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false);
    renderCover(watchedFilm);
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.getByRole('img', { name: 'Gesehen' })).toBeInTheDocument();
  });

  it('has no check on an extra', () => {
    renderCover({ ...film, kind: 'extra' });
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
