import { describe, expect, it } from 'vitest';
import type { StoredWatchTime } from './types';
import { addPlayed, DAY_MS, periodAt, remainingMs, watchFromSnapshot } from './watchTime';

const MINUTE = 60_000;
const SYNCED = Date.parse('2026-10-20T18:00:00Z');
const RESET = Date.parse('2026-10-20T22:00:00Z'); // midnight in Zurich (CEST)

const snapshot = (pendingMs = 0): StoredWatchTime =>
  watchFromSnapshot(
    { limitMinutes: 60, remainingMinutes: 15, resetsAt: '2026-10-20T22:00:00Z' },
    SYNCED,
    pendingMs,
  )!;

describe('offline watch time (AN-18)', () => {
  it('starts from the server’s remaining minutes minus play it has not booked yet', () => {
    expect(remainingMs(snapshot(), SYNCED)).toBe(15 * MINUTE);
    expect(remainingMs(snapshot(5 * MINUTE), SYNCED)).toBe(10 * MINUTE);
    expect(watchFromSnapshot(undefined, SYNCED)).toBeUndefined();
  });

  it('subtracts what the device plays and stops at 0', () => {
    let watch = addPlayed(snapshot(), 10 * MINUTE, SYNCED + 20 * MINUTE);
    expect(remainingMs(watch, SYNCED + 20 * MINUTE)).toBe(5 * MINUTE);
    watch = addPlayed(watch, 6 * MINUTE, SYNCED + 30 * MINUTE);
    expect(remainingMs(watch, SYNCED + 30 * MINUTE)).toBe(0);
  });

  it('gives the full limit again from resetsAt on, and every 24 h after', () => {
    let watch = addPlayed(snapshot(), 15 * MINUTE, RESET - MINUTE);
    expect(remainingMs(watch, RESET - 1)).toBe(0);
    expect(periodAt(watch, RESET)).toBe(0);
    expect(remainingMs(watch, RESET)).toBe(60 * MINUTE);

    watch = addPlayed(watch, 50 * MINUTE, RESET + 2 * 60 * MINUTE);
    expect(watch.period).toBe(0);
    expect(remainingMs(watch, RESET + 3 * 60 * MINUTE)).toBe(10 * MINUTE);

    // The next day (no sync in between): full limit, and its own count.
    expect(periodAt(watch, RESET + DAY_MS + 1)).toBe(1);
    expect(remainingMs(watch, RESET + DAY_MS + 1)).toBe(60 * MINUTE);
    watch = addPlayed(watch, 20 * MINUTE, RESET + DAY_MS + 1);
    expect(watch).toMatchObject({ period: 1, playedMs: 20 * MINUTE });
    expect(remainingMs(watch, RESET + DAY_MS + 2)).toBe(40 * MINUTE);
  });

  it('ignores negative play', () => {
    expect(addPlayed(snapshot(), -5000, SYNCED).playedMs).toBe(0);
  });
});
