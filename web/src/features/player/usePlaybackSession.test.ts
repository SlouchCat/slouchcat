import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { mockApi } from '@/test/api';
import { useStopPlaybackOnLeave } from './usePlaybackSession';

function stopsOf(calls: { method: string; path: string }[]) {
  return calls.filter((call) => call.method === 'DELETE').map((call) => call.path);
}

describe('useStopPlaybackOnLeave', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('keeps the session through the StrictMode double mount and stops it on unmount', async () => {
    const { calls } = mockApi({
      'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    });
    const { unmount } = renderHook(() => useStopPlaybackOnLeave('ps-a'), { reactStrictMode: true });
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(stopsOf(calls)).toEqual([]);
    unmount();
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(stopsOf(calls)).toEqual(['/playback/sessions/ps-a']);
  });

  it('stops the old session when the id changes', async () => {
    const { calls } = mockApi({
      'DELETE /playback/sessions/*': () => new Response(null, { status: 204 }),
    });
    const { rerender, unmount } = renderHook(({ id }) => useStopPlaybackOnLeave(id), {
      initialProps: { id: 'ps-old' },
    });
    rerender({ id: 'ps-new' });
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(stopsOf(calls)).toEqual(['/playback/sessions/ps-old']);
    unmount();
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(stopsOf(calls)).toEqual(['/playback/sessions/ps-old', '/playback/sessions/ps-new']);
  });
});
