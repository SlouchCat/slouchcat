import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router';
import { act, render, waitFor } from '@testing-library/react';
import { createElement, useEffect, useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { setSpatialFocus, spatialFocusKey } from '@/ui';
import { useBackFocus, type BackFocusList } from './useBackFocus';

vi.mock('@/ui', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/ui')>()),
  setSpatialFocus: vi.fn(),
  spatialFocusKey: vi.fn(),
}));

// The way back to a card (concept 08 7.4) and, for the virtual library grid, to a card that is
// not mounted when the page comes back (concept 17 E-13).

const shown: { set?: (keys: string[]) => void } = {};

function makeRouter(initial: string[], list?: BackFocusList) {
  function Grid() {
    const [keys, setKeys] = useState(initial);
    useEffect(() => {
      shown.set = setKeys;
    }, []);
    useBackFocus('grid', keys, list);
    return null;
  }
  const root = createRootRoute({ component: () => createElement(Outlet) });
  const grid = createRoute({ getParentRoute: () => root, path: '/', component: Grid });
  const detail = createRoute({
    getParentRoute: () => root,
    path: '/detail',
    component: () => null,
  });
  return createRouter({
    routeTree: root.addChildren([grid, detail]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  });
}

/** Opens the grid, leaves it from `key` to the detail and comes back with the browser's back. */
async function leaveAndReturn(router: ReturnType<typeof makeRouter>, key: string) {
  render(createElement(RouterProvider, { router }));
  await waitFor(() => expect(router.state.location.pathname).toBe('/'));
  vi.mocked(spatialFocusKey).mockReturnValue(key);
  act(() => router.history.push('/detail'));
  await waitFor(() => expect(router.state.location.pathname).toBe('/detail'));
  vi.mocked(setSpatialFocus).mockClear();
  act(() => router.history.back());
  await waitFor(() => expect(router.state.location.pathname).toBe('/'));
}

describe('useBackFocus', () => {
  beforeEach(() => vi.mocked(setSpatialFocus).mockClear());
  afterEach(() => vi.mocked(spatialFocusKey).mockReset());

  it('focuses the card it was left from when it is shown again', async () => {
    const router = makeRouter(['grid-a', 'grid-b']);
    await leaveAndReturn(router, 'grid-b');
    await waitFor(() => expect(setSpatialFocus).toHaveBeenCalledWith('grid-b'));
  });

  it('brings a card that is not mounted back by its place, then focuses it', async () => {
    const list = { indexOf: vi.fn(() => 640), reveal: vi.fn() };
    const router = makeRouter(['grid-a', 'grid-b'], list);
    await leaveAndReturn(router, 'grid-z');
    expect(list.indexOf).toHaveBeenCalledWith('grid-z');
    // The cards drawn now do not hold it: its place comes into view, once.
    await waitFor(() => expect(list.reveal).toHaveBeenCalledWith(640));
    act(() => shown.set?.(['grid-y']));
    expect(list.reveal).toHaveBeenCalledTimes(1);
    expect(setSpatialFocus).not.toHaveBeenCalled();
    act(() => shown.set?.(['grid-y', 'grid-z']));
    await waitFor(() => expect(setSpatialFocus).toHaveBeenCalledWith('grid-z'));
  });

  it('waits for the first card before it brings a place back', async () => {
    const list = { indexOf: vi.fn(() => 12), reveal: vi.fn() };
    const router = makeRouter([], list);
    await leaveAndReturn(router, 'grid-q');
    expect(list.reveal).not.toHaveBeenCalled();
    act(() => shown.set?.(['grid-a']));
    await waitFor(() => expect(list.reveal).toHaveBeenCalledWith(12));
  });

  it('remembers nothing for a focus outside the grid', async () => {
    const list = { indexOf: vi.fn(() => 3), reveal: vi.fn() };
    const router = makeRouter(['grid-a'], list);
    await leaveAndReturn(router, 'library-sort');
    act(() => shown.set?.(['grid-a', 'grid-b']));
    expect(list.indexOf).not.toHaveBeenCalled();
    expect(list.reveal).not.toHaveBeenCalled();
    expect(setSpatialFocus).not.toHaveBeenCalled();
  });
});
