import type { ItemRef } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { mockApi, problem } from '@/test/api';
import { patchWatchlist, useWatchlistMutation, useWatchlistPending } from './queries';

const ref = (id: string, inWatchlist = false): ItemRef => ({
  id,
  kind: 'movie',
  title: `Title ${id}`,
  inWatchlist,
  images: {},
});

const target = '11111111-1111-4111-8111-111111111111';
const other = '22222222-2222-4222-8222-222222222222';
const third = '33333333-3333-4333-8333-333333333333';

describe('patchWatchlist', () => {
  it('patches the title on the page of an infinite query and keeps the other pages', () => {
    const data = {
      pages: [
        { items: [ref(other), ref(third)], total: 3, offset: 0, limit: 2 },
        { items: [ref(target)], total: 3, offset: 2, limit: 2 },
      ],
      pageParams: [0, 2],
    };
    const next = patchWatchlist(data, target, true) as typeof data;
    expect(next).not.toBe(data);
    expect(next.pages[1]?.items[0]).toEqual({ ...ref(target), inWatchlist: true });
    // Structural sharing: what did not change is the same object.
    expect(next.pages[0]).toBe(data.pages[0]);
    expect(next.pageParams).toBe(data.pageParams);
    expect(next.pages[1]).not.toBe(data.pages[1]);
    // The data in the cache is never changed in place.
    expect(data.pages[1]?.items[0]?.inWatchlist).toBe(false);
  });

  it('finds the title at any depth: the detail itself and its collection members', () => {
    const detail = {
      ...ref(target),
      collections: [{ id: 7, name: 'Saga', members: [ref(other), ref(target)] }],
      seriesRef: { id: target, title: 'No mark here', images: {} },
    };
    const next = patchWatchlist(detail, target, true) as typeof detail;
    expect(next.inWatchlist).toBe(true);
    expect(next.collections[0]?.members[1]?.inWatchlist).toBe(true);
    expect(next.collections[0]?.members[0]).toBe(detail.collections[0]?.members[0]);
    // An object with the id but without a mark (a series reference) stays as it is.
    expect(next.seriesRef).toBe(detail.seriesRef);
  });

  it('returns undefined when nothing changed', () => {
    expect(patchWatchlist({ items: [ref(other)] }, target, true)).toBeUndefined();
    // The title is there but already in that state.
    expect(patchWatchlist({ items: [ref(target, true)] }, target, true)).toBeUndefined();
    expect(patchWatchlist(undefined, target, true)).toBeUndefined();
    expect(patchWatchlist('text', target, true)).toBeUndefined();
  });

  it('walks plain objects and arrays only', () => {
    class Box {
      item = ref(target);
    }
    const map = new Map([['item', ref(target)]]);
    expect(patchWatchlist({ box: new Box(), map }, target, true)).toBeUndefined();
  });
});

describe('useWatchlistMutation', () => {
  afterEach(() => vi.unstubAllGlobals());

  const setup = () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const library = {
      pages: [{ items: [ref(other), ref(target)], total: 2, offset: 0, limit: 50 }],
      pageParams: [0],
    };
    queryClient.setQueryData(['library', 'films', {}], library);
    queryClient.setQueryData(['item', target], { ...ref(target), collections: [] });
    queryClient.setQueryData(['item', other], { ...ref(other), collections: [] });
    queryClient.setQueryData(['watchlist'], {
      pages: [{ items: [ref(third, true)], total: 1, offset: 0, limit: 50 }],
      pageParams: [0],
    });
    queryClient.setQueryData(['home'], { rows: [{ id: 'recent', items: [ref(target)] }] });
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    const { result } = renderHook(() => useWatchlistMutation(), { wrapper });
    const mark = (key: readonly unknown[]) => {
      const found = JSON.stringify(queryClient.getQueryData(key)).match(
        new RegExp(`"id":"${target}"[^}]*"inWatchlist":(true|false)`),
      );
      return found?.[1];
    };
    return { queryClient, result, mark, library };
  };

  it('marks the title at once in every query that shows it and cancels only those', async () => {
    let answer: (response: Response) => void = () => undefined;
    const { calls } = mockApi({
      'PUT /items/*/watchlist': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    const { queryClient, result, mark, library } = setup();
    const cancel = vi.spyOn(queryClient, 'cancelQueries');
    const untouched = queryClient.getQueryState(['item', other])?.dataUpdatedAt;

    act(() => result.current.mutate({ id: target, on: true }));
    await waitFor(() => expect(mark(['item', target])).toBe('true'));
    expect(mark(['library', 'films', {}])).toBe('true');
    expect(mark(['home'])).toBe('true');
    // The other title's card in the library is the same object as before.
    const patched = queryClient.getQueryData<typeof library>(['library', 'films', {}]);
    expect(patched?.pages[0]?.items[0]).toBe(library.pages[0]?.items[0]);
    // Queries without the title are neither cancelled nor touched.
    const cancelled = cancel.mock.calls.map(([filters]) => filters?.queryKey);
    expect(cancelled).toHaveLength(3);
    expect(cancelled).toEqual(
      expect.arrayContaining([['library', 'films', {}], ['item', target], ['home']]),
    );
    expect(queryClient.getQueryState(['item', other])?.dataUpdatedAt).toBe(untouched);

    act(() => answer(new Response(null, { status: 204 })));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(calls.some((call) => call.method === 'PUT' && call.path.includes(target))).toBe(true);
    // Only the watchlist and the start page are read again; the library and the detail keep
    // their patched data.
    expect(queryClient.getQueryState(['watchlist'])?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(['home'])?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(['library', 'films', {}])?.isInvalidated).toBe(false);
    expect(queryClient.getQueryState(['item', target])?.isInvalidated).toBe(false);
    expect(mark(['library', 'films', {}])).toBe('true');
  });

  it('reads a query again whose page load the change cancelled', async () => {
    let answer: (response: Response) => void = () => undefined;
    mockApi({
      'PUT /items/*/watchlist': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    const { queryClient, result, mark } = setup();
    // A page of the search is being loaded while the title is marked.
    queryClient.setQueryData(['search', {}], { pages: [{ items: [ref(target)] }] });
    void queryClient.prefetchQuery({
      queryKey: ['search', {}],
      queryFn: () => new Promise(() => undefined),
    });
    expect(queryClient.getQueryState(['search', {}])?.fetchStatus).toBe('fetching');

    act(() => result.current.mutate({ id: target, on: true }));
    await waitFor(() => expect(mark(['search', {}])).toBe('true'));
    expect(queryClient.getQueryState(['search', {}])?.fetchStatus).toBe('idle');
    act(() => answer(new Response(null, { status: 204 })));
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    // The aborted query is read again; the library, which was not loading, keeps its data.
    expect(queryClient.getQueryState(['search', {}])?.isInvalidated).toBe(true);
    expect(queryClient.getQueryState(['library', 'films', {}])?.isInvalidated).toBe(false);
  });

  it('tells every button of a title that a change of it is in flight', async () => {
    let answer: (response: Response) => void = () => undefined;
    mockApi({
      'PUT /items/*/watchlist': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    const queryClient = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    // Two separate mutations (the cover and the detail page's "Merken").
    const { result } = renderHook(
      () => ({
        cover: useWatchlistMutation(),
        detail: useWatchlistMutation(),
        pending: useWatchlistPending(target),
        otherPending: useWatchlistPending(other),
      }),
      { wrapper },
    );
    expect(result.current.pending).toBe(false);
    act(() => result.current.cover.mutate({ id: target, on: true }));
    await waitFor(() => expect(result.current.pending).toBe(true));
    expect(result.current.detail.isPending).toBe(false);
    expect(result.current.otherPending).toBe(false);
    act(() => answer(new Response(null, { status: 204 })));
    await waitFor(() => expect(result.current.pending).toBe(false));
  });

  it('turns the same queries back when the server refuses', async () => {
    let answer: (response: Response) => void = () => undefined;
    mockApi({
      'PUT /items/*/watchlist': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    const { queryClient, result, mark } = setup();
    act(() => result.current.mutate({ id: target, on: true }));
    await waitFor(() => expect(mark(['item', target])).toBe('true'));
    // A query that got the title only after the change began is not turned back.
    queryClient.setQueryData(['search', {}], { pages: [{ items: [ref(target, true)] }] });
    act(() => answer(problem(500, 'internal')));

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(mark(['item', target])).toBe('false');
    expect(mark(['library', 'films', {}])).toBe('false');
    expect(mark(['home'])).toBe('false');
    expect(mark(['search', {}])).toBe('true');
  });
});
