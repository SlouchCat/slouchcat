import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { formatCountdown, useCountdown } from './useCountdown';

describe('useCountdown', () => {
  afterEach(() => vi.useRealTimers());

  it('is locked from the very first render, then counts down to zero', async () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ seconds }) => useCountdown(seconds), {
      initialProps: { seconds: null as number | null },
    });
    expect(result.current).toBe(0);

    rerender({ seconds: 30 });
    expect(result.current).toBe(30);
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(result.current).toBe(20);
    await act(() => vi.advanceTimersByTimeAsync(20_500));
    expect(result.current).toBe(0);
  });

  it('starts again for the same number of seconds with a new run id', async () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ run }) => useCountdown(5, run), {
      initialProps: { run: 1 },
    });
    await act(() => vi.advanceTimersByTimeAsync(6_000));
    expect(result.current).toBe(0);
    rerender({ run: 2 });
    expect(result.current).toBe(5);
  });

  it('formats m:ss', () => {
    expect(formatCountdown(75)).toEqual({ minutes: '1', seconds: '15' });
    expect(formatCountdown(9)).toEqual({ minutes: '0', seconds: '09' });
  });
});
