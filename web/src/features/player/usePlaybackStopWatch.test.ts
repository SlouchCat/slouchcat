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

describe('usePlaybackStopWatch', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    setVisibility('visible');
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('asks every 4 s while active and reports a stop by the administration', async () => {
    let state: object = { state: 'active' };
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json(state) });
    const onStopped = vi.fn();
    renderHook(() => usePlaybackStopWatch({ playbackSessionId: 'ps-a', active: true, onStopped }));

    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS - 1));
    expect(asks(calls)).toBe(0);
    await act(() => vi.advanceTimersByTimeAsync(1));
    expect(asks(calls)).toBe(1);
    expect(onStopped).not.toHaveBeenCalled();

    state = { state: 'stopped', stoppedBy: 'admin' };
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(2);
    expect(onStopped).toHaveBeenCalledExactlyOnceWith('admin');
  });

  it.each([
    [{ state: 'stopped', stoppedBy: 'limit' }, 'limit'],
    [{ state: 'stopped', stoppedBy: 'revoked' }, 'admin'],
    [{ state: 'stopped', stoppedBy: 'logout' }, 'admin'],
    [{ state: 'stopped' }, undefined],
  ])('maps %o to %s', async (answer, reason) => {
    mockApi({ [`GET ${statePath}`]: () => json(answer) });
    const onStopped = vi.fn();
    renderHook(() => usePlaybackStopWatch({ playbackSessionId: 'ps-a', active: true, onStopped }));
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(onStopped).toHaveBeenCalledExactlyOnceWith(reason);
  });

  it('treats 404 and a failed request as running', async () => {
    let reply = () => problem(404, 'not_found');
    const { calls } = mockApi({ [`GET ${statePath}`]: () => reply() });
    const onStopped = vi.fn();
    renderHook(() => usePlaybackStopWatch({ playbackSessionId: 'ps-a', active: true, onStopped }));
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    reply = () => {
      throw new TypeError('offline');
    };
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(2);
    expect(onStopped).not.toHaveBeenCalled();
  });

  it('does not ask while paused, and stops asking on pause and unmount', async () => {
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json({ state: 'active' }) });
    const { rerender, unmount } = renderHook(
      ({ active }) =>
        usePlaybackStopWatch({ playbackSessionId: 'ps-a', active, onStopped: vi.fn() }),
      { initialProps: { active: false } },
    );
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(0);

    rerender({ active: true });
    await act(() => vi.advanceTimersByTimeAsync(STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(1);

    rerender({ active: false });
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(1);

    rerender({ active: true });
    unmount();
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(1);
  });

  it('skips the ticks while the page is hidden and asks at once when it is visible again', async () => {
    const { calls } = mockApi({ [`GET ${statePath}`]: () => json({ state: 'active' }) });
    renderHook(() =>
      usePlaybackStopWatch({ playbackSessionId: 'ps-a', active: true, onStopped: vi.fn() }),
    );
    setVisibility('hidden');
    await act(() => vi.advanceTimersByTimeAsync(3 * STOP_WATCH_INTERVAL_MS));
    expect(asks(calls)).toBe(0);
    await act(async () => {
      setVisibility('visible');
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(asks(calls)).toBe(1);
  });
});
