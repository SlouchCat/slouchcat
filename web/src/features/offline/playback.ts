import { flushEventsSoon } from './engine';
import { getMeta, queueEvents, updateDownload, updateProfile } from './store';
import type { OfflineEvent, StoredDownload, StoredWatchTime } from './types';
import { addPlayed } from './watchTime';

// What the offline player writes: progress events into the queue (uploaded at the next contact,
// O-5), the position per profile on the download (resume without the server) and the played time
// into the profile's offline watch time (AN-18).

export interface OfflinePlayback {
  accountId: number;
  profileId: number;
  downloadId: string;
  itemId: string;
  /** ULID of this playback (duplicate check and played-time bound on the server). */
  playbackId: string;
}

export type PlaybackSample = Pick<
  OfflineEvent,
  'positionMs' | 'durationMs' | 'playedMs' | 'seq' | 'state' | 'playbackRate'
>;

/**
 * Where the player of `profileId` starts (source timeline, ms): the newer of the device's own
 * position (offline playback) and the server's progress from the last sync (online play, another
 * device; W-5, O-5). A title the server has as completed starts at 0, as online.
 */
export function resumePosition(download: StoredDownload, profileId: number): number {
  const own = download.positions?.[profileId];
  const server = download.serverPositions?.[profileId];
  if (server && (!own || server.at > own.at)) return server.completed ? 0 : server.positionMs;
  return own?.positionMs ?? 0;
}

/** Device clock corrected by the offset of the last sync. */
export async function serverNow(): Promise<number> {
  const offset = await getMeta('serverOffsetMs').catch(() => undefined);
  return Date.now() + (offset ?? 0);
}

/**
 * Queues one progress event, stores the position and books the played time; online the queue
 * goes up right away. Resolves with the profile's watch time afterwards.
 */
export async function recordPlayback(
  playback: OfflinePlayback,
  sample: PlaybackSample,
): Promise<StoredWatchTime | undefined> {
  const event: OfflineEvent = {
    profileId: playback.profileId,
    itemId: playback.itemId,
    playbackId: playback.playbackId,
    downloadId: playback.downloadId,
    ...sample,
    clientTime: new Date().toISOString(),
  };
  await queueEvents([{ accountId: playback.accountId, event }]);
  await updateDownload(
    playback.downloadId,
    (download) =>
      download && {
        ...download,
        positions: {
          ...download.positions,
          [playback.profileId]: { positionMs: sample.positionMs, at: Date.now() },
        },
      },
  );
  const now = await serverNow();
  const profile = await updateProfile(playback.profileId, (current) =>
    current?.watch && sample.playedMs > 0
      ? { ...current, watch: addPlayed(current.watch, sample.playedMs, now) }
      : undefined,
  );
  flushEventsSoon();
  return profile?.watch;
}
