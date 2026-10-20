import { NO_LOCK } from './pin';
import type { NetworkKind } from './support';
import type {
  DownloadState,
  OfflineProfile,
  OfflineSync,
  QueuedEvent,
  StoredDownload,
  StoredProfile,
} from './types';
import { snapshotDayStart, watchFromSnapshot } from './watchTime';

// What a sync answer (POST /offline/sync) changes on the device. Pure functions: the engine reads
// the database, calls these and writes the result. The client decides no visibility itself: it
// stores `allowedProfileIds` as the server computed them and filters by them (plan principle 1).

/** Most ids one sync may report (OfflineSyncRequest.held). */
export const MAX_HELD = 2000;

/**
 * Fetch errors the server side resolves (it prepares the copy again, a new manifest brings a new
 * token): the next sync that lists the download as ready clears them.
 */
const SERVER_ERRORS = new Set(['evicted', 'gone', 'unauthorized', 'size', 'changed']);

/**
 * Fetch errors of the device or of the way to the server (full storage, a locked file, a proxy
 * without Range, a network that failed on every attempt): tried again after this long, not at
 * every sync, so a full disk does not pull a block per minute (W7, K4).
 */
export const FETCH_RETRY_MS = 5 * 60_000;

const retryDue = (download: StoredDownload, now: number) =>
  download.fetchError === undefined ||
  (!SERVER_ERRORS.has(download.fetchError) && now - (download.fetchErrorAt ?? 0) >= FETCH_RETRY_MS);

/** The downloads the device reports for an account, with whether each is complete. */
export function heldDownloads(
  downloads: readonly StoredDownload[],
  accountId: number,
): { id: string; complete: boolean }[] {
  return downloads
    .filter((download) => download.accountId === accountId && !download.pendingDelete)
    .slice(0, MAX_HELD)
    .map((download) => ({ id: download.id, complete: download.complete }));
}

/** A download the device has not seen before (created on another tab, by a rule, now). */
export function newDownload(state: DownloadState, accountId: number, now: number): StoredDownload {
  return {
    id: state.id,
    accountId,
    state: state.state,
    quality: state.quality,
    auto: state.auto,
    allowedProfileIds: [...state.allowedProfileIds],
    estimatedBytes: state.estimatedBytes,
    sizeBytes: state.sizeBytes,
    segmentsDone: state.segmentsDone,
    segmentsTotal: state.segmentsTotal,
    error: state.error,
    createdAt: state.createdAt,
    readyAt: state.readyAt,
    item: state.item,
    bytesWritten: 0,
    subtitlesStored: [],
    posterStored: false,
    complete: false,
    updatedAt: now,
  };
}

/**
 * The server's state of a download merged into what the device keeps: the server owns state,
 * sizes and `allowedProfileIds`; the device keeps its manifest, its progress in bytes and the
 * title data when the answer has none (another profile is active, DownloadState.item).
 */
export function mergeDownload(
  local: StoredDownload | undefined,
  state: DownloadState,
  accountId: number,
  now: number,
): StoredDownload {
  const fresh = newDownload(state, accountId, now);
  if (!local) return fresh;
  return {
    ...local,
    ...fresh,
    item: state.item ?? local.item,
    manifest: local.manifest,
    bytesWritten: local.bytesWritten,
    subtitlesStored: local.subtitlesStored,
    posterStored: local.posterStored,
    complete: local.complete,
    pendingDelete: local.pendingDelete,
    positions: local.positions,
    fileReadyAt: local.fileReadyAt,
    fileEtag: local.fileEtag,
    ...keptFetchError(local, state),
  };
}

/**
 * A sync that lists the download as ready is a new chance after a server-side error (evicted
 * copies come back as `ready`); a copy prepared again (new readyAt) clears every error. Errors of
 * the device wait for FETCH_RETRY_MS.
 */
function keptFetchError(
  local: StoredDownload,
  state: DownloadState,
): Pick<StoredDownload, 'fetchError' | 'fetchErrorAt'> {
  const error = local.fetchError;
  const preparedAgain = state.readyAt !== undefined && state.readyAt !== local.readyAt;
  const cleared =
    error === undefined || (state.state === 'ready' && (SERVER_ERRORS.has(error) || preparedAgain));
  return cleared
    ? { fetchError: undefined, fetchErrorAt: undefined }
    : { fetchError: error, fetchErrorAt: local.fetchErrorAt };
}

/** `accountId:profileId` of every profile a download on the device names (not being deleted). */
export function profilesWithDownloads(downloads: readonly StoredDownload[]): Set<string> {
  const keys = new Set<string>();
  for (const download of downloads) {
    if (download.pendingDelete) continue;
    for (const id of download.allowedProfileIds) keys.add(`${download.accountId}:${id}`);
  }
  return keys;
}

/**
 * A snapshot without the PIN material (salt and the separate offline PIN): a profile keeps it
 * only on a device with downloads it may play (AN-19, plan 3.3).
 */
export function withoutPinMaterial(snapshot: OfflineProfile): OfflineProfile {
  const rest = { ...snapshot };
  delete rest.pinSalt;
  delete rest.pinIterations;
  delete rest.offlinePin;
  return rest;
}

/** Played milliseconds of queued events of a profile since `since` (server time). */
function queuedPlay(
  queued: readonly QueuedEvent[],
  profileId: number,
  since: number,
  offsetMs: number,
): number {
  return queued
    .filter(
      ({ event }) =>
        event.profileId === profileId && Date.parse(event.clientTime) + offsetMs >= since,
    )
    .reduce((sum, { event }) => sum + event.playedMs, 0);
}

/** The stored profile of a sync answer; a new PIN salt voids the device's verifier. */
export function mergeProfile(
  local: StoredProfile | undefined,
  snapshot: OfflineProfile,
  context: {
    accountId: number;
    now: number;
    offsetMs: number;
    queued: readonly QueuedEvent[];
    rules?: StoredProfile['rules'];
    /** A download on the device names the profile: only then it keeps PIN material (AN-19). */
    holdsDownloads: boolean;
  },
): StoredProfile {
  const kept = context.holdsDownloads ? snapshot : withoutPinMaterial(snapshot);
  const verifier =
    kept.hasPin && kept.pinSalt && local?.pinVerifier?.salt === kept.pinSalt
      ? local.pinVerifier
      : undefined;
  const pending = snapshot.watchTime
    ? queuedPlay(
        context.queued,
        snapshot.id,
        snapshotDayStart(snapshot.watchTime),
        context.offsetMs,
      )
    : 0;
  return {
    id: snapshot.id,
    accountId: context.accountId,
    snapshot: kept,
    syncedAt: context.now,
    pinVerifier: verifier,
    watch: watchFromSnapshot(snapshot.watchTime, context.now, pending),
    lock: local?.lock ?? NO_LOCK,
    rules: context.rules ?? local?.rules,
  };
}

export interface SyncInput {
  accountId: number;
  activeProfileId: number;
  /** Device time when the answer arrived. */
  now: number;
  /** Everything the device holds (all accounts). */
  downloads: readonly StoredDownload[];
  profiles: readonly StoredProfile[];
  /** Events not uploaded yet: their watch time is not in the snapshot. */
  queued: readonly QueuedEvent[];
  answer: OfflineSync;
}

export interface SyncResult {
  /** Downloads to store (merged). */
  put: StoredDownload[];
  /** Revoked: delete the entry and its files (AN-4, K-7). */
  remove: string[];
  putProfiles: StoredProfile[];
  /** Profiles of the account the server no longer lists for this device. */
  removeProfiles: number[];
  /** Server time minus device time. */
  serverOffsetMs: number;
  /** Ready on the server, not complete here: to fetch, oldest first. */
  fetch: string[];
}

export function applySync(input: SyncInput): SyncResult {
  const { accountId, activeProfileId, now, answer } = input;
  const serverOffsetMs = Date.parse(answer.serverTime) - now;
  const revoked = new Set(answer.revoked);
  const local = new Map(input.downloads.map((download) => [download.id, download]));
  const put = answer.downloads
    .filter((state) => !revoked.has(state.id))
    .map((state) => mergeDownload(local.get(state.id), state, accountId, now));
  const listed = new Set(answer.profiles.map((profile) => profile.id));
  const localProfiles = new Map(input.profiles.map((profile) => [profile.id, profile]));
  // What the device holds after the answer: the merged ones and the others it keeps.
  const merged = new Set(put.map((download) => download.id));
  const withDownloads = profilesWithDownloads([
    ...put,
    ...input.downloads.filter((download) => !merged.has(download.id) && !revoked.has(download.id)),
  ]);
  const putProfiles = answer.profiles.map((snapshot) =>
    mergeProfile(localProfiles.get(snapshot.id), snapshot, {
      accountId,
      now,
      offsetMs: serverOffsetMs,
      queued: input.queued,
      rules: snapshot.id === activeProfileId ? answer.rules : undefined,
      holdsDownloads: withDownloads.has(`${accountId}:${snapshot.id}`),
    }),
  );
  return {
    put,
    remove: [...revoked].filter((id) => local.has(id)),
    putProfiles,
    removeProfiles: input.profiles
      .filter((profile) => profile.accountId === accountId && !listed.has(profile.id))
      .map((profile) => profile.id),
    serverOffsetMs,
    fetch: fetchOrder(put, now),
  };
}

/**
 * Downloads ready on the server whose file the device lacks, oldest first; a download whose
 * last fetch failed waits as `retryDue` says.
 */
export function fetchOrder(downloads: readonly StoredDownload[], now = Date.now()): string[] {
  return downloads
    .filter(
      (download) =>
        download.state === 'ready' &&
        !download.complete &&
        !download.pendingDelete &&
        retryDue(download, now),
    )
    .sort((a, b) => a.createdAt.localeCompare(b.createdAt))
    .map((download) => download.id);
}

/**
 * The downloads a profile may see on this device: the server listed the profile in
 * `allowedProfileIds` (O-7). The only filter of the offline library.
 */
export function downloadsForProfile(
  downloads: readonly StoredDownload[],
  accountId: number,
  profileId: number,
): StoredDownload[] {
  return downloads
    .filter(
      (download) =>
        download.accountId === accountId &&
        !download.pendingDelete &&
        download.allowedProfileIds.includes(profileId),
    )
    .sort((a, b) => b.createdAt.localeCompare(a.createdAt));
}

/**
 * Profiles that may open the offline library: of the device's account (the last one signed in,
 * decision of 2026-10-20 on shared devices), listed by the server and allowed for a download.
 * Without an account (none yet, or it signed out) the library shows no profile.
 */
export function offlineProfiles(
  profiles: readonly StoredProfile[],
  downloads: readonly StoredDownload[],
  accountId: number | undefined,
): StoredProfile[] {
  if (accountId === undefined) return [];
  return profiles
    .filter(
      (profile) =>
        profile.accountId === accountId &&
        downloads.some(
          (download) =>
            download.accountId === profile.accountId &&
            !download.pendingDelete &&
            download.allowedProfileIds.includes(profile.id),
        ),
    )
    .sort(
      (a, b) =>
        Number(b.snapshot.isPrimary) - Number(a.snapshot.isPrimary) ||
        a.snapshot.name.localeCompare(b.snapshot.name),
    );
}

/**
 * Downloads of other accounts on this device (a guest who used it before): the device asks the
 * public revocation list about them after every sync of the signed-in account (AN-4, K-7).
 */
export function otherAccountDownloads(
  downloads: readonly StoredDownload[],
  accountId: number,
): string[] {
  return downloads
    .filter((download) => download.accountId !== accountId)
    .slice(0, MAX_HELD)
    .map((download) => download.id);
}

/**
 * An auto-download waits on a cellular connection while an enabled rule of its account on this
 * device says "Wi-Fi only" (AN-13). Without network information (desktop, iOS) it fetches.
 */
export function waitsForWifi(
  download: StoredDownload,
  profiles: readonly StoredProfile[],
  network: NetworkKind,
): boolean {
  if (!download.auto || network !== 'cellular') return false;
  return profiles.some(
    (profile) =>
      profile.accountId === download.accountId &&
      profile.rules?.enabled === true &&
      profile.rules.wifiOnly,
  );
}
