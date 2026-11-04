import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem } from '@/test/api';
import { STOP_WATCH_INTERVAL_MS, usePlaybackStopWatch } from './usePlaybackStopWatch';

const statePath = '/playback/sessions/ps-a/state';
const asks = (calls: { method: string; path: string }[]) =>
  calls.filter((call) => call.method === 'GET' && call.path === statePath).length;

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange'));
}

const watch = (active = true, onStopped = vi.fn()) =>
  renderHook(
    ({ on }: { on: boolean }) =>
      usePlaybackStopWatch({ playbackSessionId: 'ps-a', active: on, onStopped }),
    { initialProps: { on: active } },
  );

describe('usePlaybackStopWatch', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    setVisibility('visible');
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('asks at once, then every 4 s, and reports a stop by the administration', async () => {
    let state: object = { state: 'active' };
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json(state) });
    const onStopped = vi.fn();
    watch(true, onStopped);

    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(asks(calls)).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS - 1));
    expect(asks(calls)).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(asks(calls)).toBe(2);
    expect(onStopped).not.toHaveBeenCalled();

    state = { state: 'stopped', stoppedBy: 'admin' };
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(3);
    expect(onStopped).toHaveBeenCalledExactlyOnceWith('admin');
  });

  it.each([
    ['the watch-time limit', () => json({ state: 'stopped', stoppedBy: 'limit' }), 'limit'],
    ['a profile switch', () => json({ state: 'stopped', stoppedBy: 'logout' }), 'admin'],
    ['an expired playback', () => json({ state: 'stopped' }), undefined],
    // A logout (also in another tab) and a deleted account end the browser session first.
    ['an ended browser session (401)', () => problem(401, 'unauthenticated'), undefined],
    // A deleted profile is detached from the browser session first.
    ['a deleted profile (403 profile_required)', () => problem(403, 'profile_required'), undefined],
  ])('reports %s as a stop', async (_name, reply, reason) => {
    mockApi({ [`GET ${statePath}`]: reply });
    const onStopped = vi.fn();
    watch(true, onStopped);
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(onStopped).toHaveBeenCalledExactlyOnceWith(reason);
  });

  it('treats 404, another 403 and a failed request as running', async () => {
    let reply = () => problem(404, 'not_found');
    const { calls } = mockApi({ [`GET ${statePath}`]: () => reply() });
    const onStopped = vi.fn();
    watch(true, onStopped);
    await act(() => vi.advanceTimersByTimeAsync(0));
    reply = () => problem(403, 'csrf_header_missing');
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    reply = () => {
      throw new TypeError('offline');
    };
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(3);
    expect(onStopped).not.toHaveBeenCalled();
  });

  it('abandons a request that hangs and asks again 4 s later', async () => {
    let hang = true;
    const { calls } = mockApi({
      [`GET ${statePath}`]: () =>
        hang ? new Promise<Response>(() => undefined) : json({ state: 'stopped' }),
    });
    const onStopped = vi.fn();
    watch(true, onStopped);
    await act(() => vi.advanceTimersByTimeAsync(0));
    hang = false;
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(2);
    expect(onStopped).toHaveBeenCalledExactlyOnceWith(undefined);
  });

  it('does not ask while paused; a pause does not push the next request', async () => {
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json({ state: 'active' }) });
    const { rerender, unmount } = watch(false);
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(0);

    rerender({ on: true });
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(asks(calls)).toBe(1);

    // Paused 1 s after the request, resumed 1 s later: the next request stays 4 s after the last.
    await act(() => vi.advanceTimersByTimeAsync(1000));
    rerender({ on: false });
    await act(() => vi.advanceTimersByTimeAsync(1000));
    rerender({ on: true });
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS - 2001));
    expect(asks(calls)).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(asks(calls)).toBe(2);

    // Paused longer than 4 s: the resume asks at once.
    rerender({ on: false });
    await act(() => vi.advanceTimersByTimeAsync(2 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(2);
    rerender({ on: true });
    await act(() => vi.advanceTimersByTimeAsync(0));
    expect(asks(calls)).toBe(3);

    unmount();
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(3);
  });

  it('keeps asking while the page is hidden and asks at once when it is visible again', async () => {
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json({ state: 'active' }) });
    watch();
    await act(() => vi.advanceTimersByTimeAsync(0));
    setVisibility('hidden');
    await act(() => vi.advanceTimersByTimeAsync(2 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(3);
    await act(() => vi.advanceTimersByTimeAsync(1000));
    await act(async () => {
      setVisibility('visible');
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(asks(calls)).toBe(4);
  });
});
