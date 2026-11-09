import type { Trickplay } from '@slouchcat/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react';
import type { ReactNode } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem } from '@/test/api';
import { seekBarTrickplay, TRICKPLAY_POLL_MS, useTrickplay } from './useTrickplay';

const set = (state: Trickplay['state'], sheets: number[]): Trickplay => ({
  version: '1000-17',
  state,
  intervalMs: 10_000,
  tileWidth: 320,
  tileHeight: 180,
  columns: 10,
  rows: 10,
  pointsMs: [0, 10_000, 20_000],
  sheets: sheets.map((index) => ({
    index,
    url: `/items/i1/trickplay/${index}?mediaFileId=f1&v=1000-17`,
  })),
});

function renderTrickplay(initial: Trickplay | undefined) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
  return renderHook(
    () => useTrickplay({ itemId: 'i1', mediaFileId: 'f1', playbackSessionId: 'ps1', initial }),
    { wrapper },
  );
}

const asks = (calls: { method: string; path: string }[]) =>
  calls.filter((call) => call.path === '/items/i1/trickplay').length;

describe('seekBarTrickplay', () => {
  it('resolves the sheet URLs and leaves out a sheet without one', () => {
    const bar = seekBarTrickplay(set('running', [0, 2]), (url, index) =>
      index === 2 ? undefined : `blob:${url}`,
    );
    expect(bar?.sheets).toEqual([
      { index: 0, url: 'blob:/items/i1/trickplay/0?mediaFileId=f1&v=1000-17' },
    ]);
    expect(bar).toMatchObject({ tileWidth: 320, tileHeight: 180, columns: 10, rows: 10 });
    expect(seekBarTrickplay(set('complete', [1]))?.sheets[0]?.url).toBe(
      '/api/v1/items/i1/trickplay/1?mediaFileId=f1&v=1000-17',
    );
  });

  it('gives nothing for a set that failed without a sheet, but keeps one with sheets', () => {
    expect(seekBarTrickplay(set('failed', []))).toBeUndefined();
    expect(seekBarTrickplay(set('failed', [0]))?.sheets).toHaveLength(1);
    expect(seekBarTrickplay(set('running', []))?.sheets).toEqual([]);
  });
});

describe('useTrickplay (concept 18, 9)', () => {
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('asks again every 10 s while the set is made and stops at complete', async () => {
    const answers = [set('running', [0]), set('complete', [0, 1])];
    const { calls } = mockApi({
      'GET /items/i1/trickplay': () => json(answers.shift() ?? set('complete', [0, 1])),
    });
    const { result } = renderTrickplay(set('queued', []));
    expect(result.current?.sheets).toEqual([]);
    await act(() => vi.advanceTimersByTimeAsync(10));
    expect(asks(calls)).toBe(0);
    await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS));
    expect(asks(calls)).toBe(1);
    expect(result.current?.sheets.map((sheet) => sheet.index)).toEqual([0]);
    await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS));
    expect(asks(calls)).toBe(2);
    expect(result.current?.sheets.map((sheet) => sheet.index)).toEqual([0, 1]);
    await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS * 3));
    expect(asks(calls)).toBe(2);
  });

  it('does not ask for a complete or failed set, nor without one', async () => {
    for (const initial of [set('complete', [0]), set('failed', [0]), undefined]) {
      const { calls } = mockApi({ 'GET /items/i1/trickplay': () => json(set('complete', [0])) });
      const { result, unmount } = renderTrickplay(initial);
      await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS * 2));
      expect(asks(calls)).toBe(0);
      expect(result.current === undefined).toBe(initial === undefined);
      unmount();
    }
  });

  it('stops at a 404 and keeps the sheets it knew', async () => {
    const { calls } = mockApi({ 'GET /items/i1/trickplay': () => problem(404, 'not_found') });
    const { result } = renderTrickplay(set('running', [0]));
    await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS));
    expect(asks(calls)).toBe(1);
    await act(() => vi.advanceTimersByTimeAsync(TRICKPLAY_POLL_MS * 3));
    expect(asks(calls)).toBe(1);
    expect(result.current?.sheets.map((sheet) => sheet.index)).toEqual([0]);
  });
});
