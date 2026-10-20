import type { OfflineWatchTime, StoredWatchTime } from './types';

// Watch time offline (K-5, AN-18; declared exception 2 of plan principle 1): the server says at
// the sync how many minutes the profile has left today and when the day ends (midnight in the
// server's time zone). Offline the device subtracts what it plays by its own clock; from
// `resetsAt` on the full limit applies again, every further 24 h as well. All times here are
// server time: the device clock plus the offset measured at the sync.

export const DAY_MS = 86_400_000;
const MINUTE_MS = 60_000;

/**
 * The stored watch time of a sync answer. `pendingMs` is play of today that the server has not
 * booked yet (queued events), so it still counts against the snapshot.
 */
export function watchFromSnapshot(
  snapshot: OfflineWatchTime | undefined,
  syncedAt: number,
  pendingMs = 0,
): StoredWatchTime | undefined {
  if (!snapshot) return undefined;
  return {
    limitMinutes: snapshot.limitMinutes,
    remainingMinutes: snapshot.remainingMinutes,
    resetsAt: Date.parse(snapshot.resetsAt),
    syncedAt,
    period: -1,
    playedMs: Math.max(0, pendingMs),
  };
}

/** -1 before `resetsAt`, else the number of the 24-h period after it that `now` lies in. */
export function periodAt(watch: StoredWatchTime, now: number): number {
  return now < watch.resetsAt ? -1 : Math.floor((now - watch.resetsAt) / DAY_MS);
}

/** Milliseconds the profile may still play at `now`. */
export function remainingMs(watch: StoredWatchTime, now: number): number {
  const period = periodAt(watch, now);
  const played = watch.period === period ? watch.playedMs : 0;
  const allowance =
    period === -1 ? watch.remainingMinutes * MINUTE_MS : watch.limitMinutes * MINUTE_MS;
  return Math.max(0, allowance - played);
}

/** Adds played time at `now`; play in a new period starts that period's count. */
export function addPlayed(watch: StoredWatchTime, ms: number, now: number): StoredWatchTime {
  const period = periodAt(watch, now);
  const add = Math.max(0, ms);
  return period === watch.period
    ? { ...watch, playedMs: watch.playedMs + add }
    : { ...watch, period, playedMs: add };
}

/** Start of the day the snapshot describes (server time), for counting queued play. */
export const snapshotDayStart = (snapshot: OfflineWatchTime) =>
  Date.parse(snapshot.resetsAt) - DAY_MS;
