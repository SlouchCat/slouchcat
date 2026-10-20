import type { SessionInfo } from '@slouchcat/api-client';
import type { QueryClient } from '@tanstack/react-query';
import { api, apiUrl, isApiError, request } from '@/api/client';
import { sessionQuery } from '@/app/session';
import { FetchError, fetchFile } from './downloader';
import { dropUnusedPinMaterial } from './offlineProfiles';
import {
  listDownloadFolders,
  MEDIA_FILE,
  mediaSize,
  openMediaWriter,
  POSTER_FILE,
  readFile,
  removeFiles,
  subtitleFile,
  truncateMedia,
  writeFile,
} from './opfs';
import { resumeOffset } from './range';
import {
  deleteDownloads,
  deleteEvents,
  deleteProfiles,
  getDownload,
  getMeta,
  listAllEvents,
  listDownloads,
  listEvents,
  listProfiles,
  setMeta,
  updateDownload,
  updateProfile,
} from './store';
import { networkKind } from './support';
import {
  applySync,
  fetchOrder,
  heldDownloads,
  mergeDownload,
  mergeProfile,
  otherAccountDownloads,
  profilesWithDownloads,
  waitsForWifi,
} from './sync';
import type { DownloadManifest, OfflineSync, StoredDownload } from './types';

// The offline engine of a page (design 4): it syncs the device (POST /offline/sync) at the start,
// when the network comes back, when the app gets the focus again and every 60 s while it is
// online with an active profile; applies the answer (revoked downloads, allowedProfileIds,
// profile snapshots, rules); asks the revocation list about the downloads of other accounts on
// the device; fetches ready downloads block by block into OPFS, resuming where it stopped; and
// uploads the progress events of offline playback. One engine per page; between the tabs of the
// app, Web Locks let one tab sync and one tab fetch at a time.

export const SYNC_INTERVAL_MS = 60_000;
/** Most events per upload (OfflineEventBatch.events). */
const EVENT_BATCH = 500;

/** Triggers within this time give one run. */
const KICK_DELAY_MS = 200;

/** How long a delete on this device keeps a sync answer from bringing the download back (K5). */
export const RECENT_DELETE_MS = 10 * 60_000;

/** Web Locks of the engine (one sync and one fetch at a time across the app's tabs, W9). */
const SYNC_LOCK = 'slouchcat-offline-sync';
const FETCH_LOCK = 'slouchcat-offline-fetch';

interface Fetching {
  id: string;
  controller: AbortController;
  /** Settles once the fetch stopped and its writer is closed (the files can be removed). */
  done: Promise<void>;
}

interface Engine {
  queryClient: QueryClient;
  running: boolean;
  again: boolean;
  /** Timer of the next run (kickOfflineEngine). */
  scheduled: number | undefined;
  /** The fetch loop runs (set before its first await, so a second run starts none, K6). */
  looping: boolean;
  /** The download whose file is coming in now; one at a time. */
  fetching: Fetching | null;
  stop: () => void;
}

let engine: Engine | null = null;

const sessionOf = (queryClient: QueryClient) =>
  queryClient.getQueryData<SessionInfo | null>(sessionQuery.queryKey);

/** Account and active profile of a session: what decides what to sync. */
const sessionKey = (session: SessionInfo | null | undefined) =>
  session === undefined
    ? 'unknown'
    : session === null
      ? 'none'
      : `${session.account.id}:${session.profile?.id ?? ''}`;

const canFetchManifest = (session: SessionInfo | null | undefined, download: StoredDownload) =>
  Boolean(
    session?.profile &&
    session.account.id === download.accountId &&
    download.allowedProfileIds.includes(session.profile.id),
  );

/** Same prepared file: the server did not prepare it again since the device stored the manifest. */
const sameFile = (a: DownloadManifest, b: DownloadManifest) =>
  a.sizeBytes === b.sizeBytes &&
  a.initLength === b.initLength &&
  a.segments.length === b.segments.length &&
  a.segments.at(-1)?.offset === b.segments.at(-1)?.offset;

/**
 * Runs `task` under the Web Lock `name` when no other tab holds it; skips it otherwise (the other
 * tab does the work). Without Web Locks (old browsers, tests) it just runs.
 */
async function withLock(name: string, task: () => Promise<void>): Promise<void> {
  const locks = typeof navigator === 'undefined' ? undefined : navigator.locks;
  if (!locks || typeof locks.request !== 'function') {
    await task();
    return;
  }
  await locks.request(name, { ifAvailable: true }, async (lock) => {
    if (lock) await task();
  });
}

/**
 * Starts the engine of this page (RootLayout). Returns the stop function; a second start while
 * one runs returns that one's stop.
 */
export function startOfflineEngine(queryClient: QueryClient): () => void {
  if (engine) return engine.stop;
  const kick = () => kickOfflineEngine();
  // A reconnect is a new chance for fetches that failed at the network.
  const onOnline = () => void clearNetworkErrors().finally(kick);
  const onOffline = () => void cancelFetch();
  const onVisible = () => {
    if (document.visibilityState === 'visible') kick();
  };
  const timer = window.setInterval(() => {
    if (navigator.onLine && sessionOf(queryClient)?.profile) kick();
  }, SYNC_INTERVAL_MS);
  // Another session (login, profile switch, logout) is a reason to sync or check revocations;
  // a refetch of the same session is not.
  let seen = sessionKey(sessionOf(queryClient));
  const unsubscribe = queryClient.getQueryCache().subscribe((event) => {
    const key = event.query.queryKey as readonly unknown[];
    if (event.type !== 'updated' || key[0] !== sessionQuery.queryKey[0]) return;
    const next = sessionKey(sessionOf(queryClient));
    if (next === seen) return;
    seen = next;
    kick();
  });
  window.addEventListener('online', onOnline);
  window.addEventListener('offline', onOffline);
  window.addEventListener('focus', kick);
  document.addEventListener('visibilitychange', onVisible);
  const stop = () => {
    window.clearInterval(timer);
    unsubscribe();
    window.removeEventListener('online', onOnline);
    window.removeEventListener('offline', onOffline);
    window.removeEventListener('focus', kick);
    document.removeEventListener('visibilitychange', onVisible);
    void cancelFetch();
    if (engine?.scheduled !== undefined) window.clearTimeout(engine.scheduled);
    engine = null;
  };
  engine = {
    queryClient,
    running: false,
    again: false,
    scheduled: undefined,
    looping: false,
    fetching: null,
    stop,
  };
  const current = engine;
  void checkStoredFiles()
    .then(() => sweepFolders(current))
    .catch(() => undefined)
    .finally(kick);
  return stop;
}

/** Runs the engine soon (after a download was created, an event queued, a profile chosen). */
export function kickOfflineEngine(): void {
  const current = engine;
  // Focus and visibilitychange come together: one run for triggers close to each other.
  if (!current || current.scheduled !== undefined) return;
  current.scheduled = window.setTimeout(() => {
    current.scheduled = undefined;
    void run();
  }, KICK_DELAY_MS);
}

/**
 * Stops the running fetch (of `id`, or any). Resolves once it stopped and closed its file, so the
 * caller can remove the files (a sync access handle keeps them locked until then, W8).
 */
export function cancelFetch(id?: string): Promise<void> {
  const current = engine?.fetching;
  if (!current || (id !== undefined && current.id !== id)) return Promise.resolve();
  current.controller.abort();
  return current.done;
}

async function run(): Promise<void> {
  const current = engine;
  if (!current) return;
  if (current.running) {
    current.again = true;
    return;
  }
  current.running = true;
  try {
    do {
      current.again = false;
      await runOnce(current);
    } while (current.again && engine === current);
  } finally {
    current.running = false;
  }
}

async function runOnce(current: Engine): Promise<void> {
  if (!navigator.onLine) {
    await cancelFetch();
    return;
  }
  let session = sessionOf(current.queryClient);
  // On the offline pages the guard does not ask for the session; ask now that there is network.
  if (session === undefined) {
    session = await current.queryClient.fetchQuery(sessionQuery).catch(() => undefined);
  }
  try {
    if (session) await rememberAccount(session.account.id);
    if (session?.profile) {
      const { account, profile } = session;
      await sendPendingDeletes(account.id);
      await withLock(SYNC_LOCK, () => sync(account.id, profile.id));
    } else if (session === null) {
      await checkRevocations((await listDownloads()).map((download) => download.id));
    }
    if (session) await flushEvents(session.account.id);
  } catch {
    // Storage or network trouble: the next trigger tries again.
  }
  // Files whose removal failed (a fetch still held them, W8) go now.
  await sweepFolders(current).catch(() => undefined);
  void fetchLoop(current);
}

// ---------- accounts on the device ----------

/**
 * The signed-in account is the device's account from now on: the offline library shows its
 * profiles, and a sign-out lock (useLogout) ends with its sign-in (decision of 2026-10-20).
 */
async function rememberAccount(accountId: number): Promise<void> {
  if ((await getMeta('accountId')) !== accountId) await setMeta('accountId', accountId);
  if ((await getMeta('signedOut')) === true) await setMeta('signedOut', false);
}

/** Notes downloads deleted on this device, so a sync answer in flight does not bring them back. */
export async function rememberDeleted(ids: readonly string[], now = Date.now()): Promise<void> {
  if (ids.length === 0) return;
  const recent = await recentDeletes(now);
  for (const id of ids) recent[id] = now;
  await setMeta('recentDeletes', recent);
}

/** The recent deletes that still count (older ones are dropped). */
async function recentDeletes(now: number): Promise<Record<string, number>> {
  const stored = (await getMeta('recentDeletes')) ?? {};
  return Object.fromEntries(Object.entries(stored).filter(([, at]) => now - at < RECENT_DELETE_MS));
}

// ---------- sync ----------

async function sync(accountId: number, profileId: number): Promise<void> {
  const held = heldDownloads(await listDownloads(), accountId);
  let answer: OfflineSync;
  try {
    answer = await request(api.POST('/offline/sync', { body: { held, network: networkKind() } }));
  } catch (error) {
    // The session ended (a revoked guest): the device asks by its cookie what to delete.
    if (isApiError(error, 401)) {
      await checkRevocations((await listDownloads()).map((download) => download.id));
    }
    return;
  }
  const now = Date.now();
  const [downloads, profiles, queued, deleted] = await Promise.all([
    listDownloads(),
    listProfiles(),
    listAllEvents(),
    recentDeletes(now),
  ]);
  const result = applySync({
    accountId,
    activeProfileId: profileId,
    now,
    downloads,
    profiles,
    queued,
    answer,
  });
  await removeDownloads(result.remove);
  const revoked = new Set(answer.revoked);
  // Per record, so a fetch writing its progress meanwhile keeps it.
  for (const state of answer.downloads) {
    // Revoked, or deleted here after the server wrote this answer (K5).
    if (revoked.has(state.id) || deleted[state.id] !== undefined) continue;
    await updateDownload(state.id, (local) =>
      mergeDownload(local, state, accountId, now, { profileId, offsetMs: result.serverOffsetMs }),
    );
  }
  // PIN material stays only for profiles a download on the device names (AN-19).
  const stored = await listDownloads();
  const withDownloads = profilesWithDownloads(stored);
  for (const snapshot of answer.profiles) {
    await updateProfile(snapshot.id, (local) =>
      mergeProfile(local, snapshot, {
        accountId,
        now,
        offsetMs: result.serverOffsetMs,
        queued,
        rules: snapshot.id === profileId ? answer.rules : undefined,
        holdsDownloads: withDownloads.has(`${accountId}:${snapshot.id}`),
      }),
    );
  }
  await deleteProfiles(result.removeProfiles);
  await setMeta('accountId', accountId);
  await setMeta('lastSyncAt', now);
  await setMeta('serverOffsetMs', result.serverOffsetMs);
  await setMeta('recentDeletes', deleted);
  // A guest who used the device before is revoked even while another account is signed in
  // (AN-4, K-7): the public list answers by the device cookie.
  await checkRevocations(otherAccountDownloads(stored, accountId), accountId);
  await dropUnusedPinMaterial();
}

/** Removes downloads with their files (revoked, deleted). */
async function removeDownloads(ids: readonly string[]): Promise<void> {
  for (const id of ids) {
    // The fetch closes its file first; a removal that still fails is retried (sweepFolders).
    await cancelFetch(id);
    await removeFiles(id).catch(() => undefined);
  }
  await deleteDownloads(ids);
}

/**
 * The device names downloads and learns by its device cookie which ones are revoked (AN-4):
 * without a valid session (a revoked guest), and for the downloads of other accounts after a
 * sync of `signedIn`. Profiles and queued events of accounts left without downloads go as well,
 * except those of `signedIn`.
 */
async function checkRevocations(ids: readonly string[], signedIn?: number): Promise<void> {
  if (ids.length === 0) return;
  let revoked: string[];
  try {
    revoked = (
      await request(api.POST('/offline/revocations', { body: { ids: ids.slice(0, 2000) } }))
    ).revoked;
  } catch {
    return;
  }
  if (revoked.length === 0) return;
  await removeDownloads(revoked);
  const left = new Set((await listDownloads()).map((download) => download.accountId));
  if (signedIn !== undefined) left.add(signedIn);
  const profiles = await listProfiles();
  await deleteProfiles(
    profiles.filter((profile) => !left.has(profile.accountId)).map((profile) => profile.id),
  );
  // Their events can never be uploaded (the account is gone): drop them (K7).
  const events = await listAllEvents();
  await deleteEvents(
    events.flatMap((entry) =>
      left.has(entry.accountId) || entry.key === undefined ? [] : [entry.key],
    ),
  );
  await dropUnusedPinMaterial();
}

/** Deletes made while offline reach the server now (the files are gone already). */
async function sendPendingDeletes(accountId: number): Promise<void> {
  const pending = (await listDownloads()).filter(
    (download) => download.pendingDelete && download.accountId === accountId,
  );
  for (const download of pending) {
    try {
      await request(api.DELETE('/downloads/{id}', { params: { path: { id: download.id } } }));
    } catch (error) {
      if (!isApiError(error, 404)) continue;
    }
    await deleteDownloads([download.id]);
  }
}

/** Uploads the queued events of the account (O-5); the server merges and books them. */
async function flushEvents(accountId: number): Promise<void> {
  for (;;) {
    const batch = await listEvents(accountId, EVENT_BATCH);
    if (batch.length === 0) return;
    try {
      await request(
        api.POST('/offline/events', {
          body: { sentAt: new Date().toISOString(), events: batch.map((entry) => entry.event) },
        }),
      );
    } catch (error) {
      // A batch the server cannot read would block the queue for good; anything else waits.
      if (!isApiError(error, 422)) return;
    }
    await deleteEvents(batch.flatMap((entry) => (entry.key === undefined ? [] : [entry.key])));
  }
}

/** Uploads queued events now when the device is online with a session (the player calls it). */
export function flushEventsSoon(): void {
  if (!engine || !navigator.onLine) return;
  const session = sessionOf(engine.queryClient);
  if (session) void flushEvents(session.account.id).catch(() => undefined);
}

// ---------- stored files ----------

/**
 * At the start: a complete download whose file is not (fully) in OPFS any more (storage cleared,
 * a delete whose request failed) is fetched again instead of offering a dead play button (K3).
 */
async function checkStoredFiles(): Promise<void> {
  for (const download of await listDownloads()) {
    if (!download.complete || !download.manifest) continue;
    const size = await mediaSize(download.id);
    if (size === download.manifest.sizeBytes) continue;
    await patch(download.id, {
      complete: false,
      ...(size === 0 ? { bytesWritten: 0, subtitlesStored: [], posterStored: false } : {}),
    });
  }
}

/**
 * OPFS folders without a live database entry (deleted, revoked, a removal that failed while a
 * fetch held the file) go away; the folder of the running fetch stays.
 */
async function sweepFolders(current: Engine): Promise<void> {
  const live = new Set(
    (await listDownloads())
      .filter((download) => !download.pendingDelete)
      .map((download) => download.id),
  );
  for (const id of await listDownloadFolders()) {
    if (live.has(id) || current.fetching?.id === id) continue;
    await removeFiles(id).catch(() => undefined);
  }
}

/** Fetch errors of the network wait no longer once the device is online again. */
async function clearNetworkErrors(): Promise<void> {
  for (const download of await listDownloads()) {
    if (download.fetchError !== 'network' && download.fetchError !== 'range') continue;
    await patch(download.id, { fetchError: undefined, fetchErrorAt: undefined });
  }
}

// ---------- fetch ----------

async function fetchLoop(current: Engine): Promise<void> {
  if (current.looping) return;
  current.looping = true;
  try {
    await withLock(FETCH_LOCK, () => fetchAll(current));
  } finally {
    current.looping = false;
  }
}

async function fetchAll(current: Engine): Promise<void> {
  const tried = new Set<string>();
  for (;;) {
    if (engine !== current || !navigator.onLine) return;
    const [downloads, profiles] = await Promise.all([listDownloads(), listProfiles()]);
    const session = sessionOf(current.queryClient);
    const network = networkKind();
    const byId = new Map(downloads.map((download) => [download.id, download]));
    const next = fetchOrder(downloads)
      .map((id) => byId.get(id))
      .find(
        (download): download is StoredDownload =>
          download !== undefined &&
          !tried.has(download.id) &&
          // Only the signed-in account's downloads come in (another account's stay as they are).
          download.accountId === session?.account.id &&
          !waitsForWifi(download, profiles, network) &&
          (download.manifest !== undefined || canFetchManifest(session, download)),
      );
    if (!next) return;
    tried.add(next.id);
    const controller = new AbortController();
    let finish = () => {};
    const done = new Promise<void>((resolve) => {
      finish = resolve;
    });
    current.fetching = { id: next.id, controller, done };
    try {
      await fetchDownload(next, session, controller.signal);
    } catch (error) {
      if (!controller.signal.aborted) await recordFetchError(next.id, error);
    } finally {
      if (current.fetching?.id === next.id) current.fetching = null;
      finish();
    }
    if (controller.signal.aborted) return;
  }
}

/** The stored code of a failed fetch (labels.ts shows it). */
function fetchErrorCode(error: unknown): string {
  if (error instanceof FetchError) return error.code;
  if (error instanceof DOMException && error.name === 'QuotaExceededError') return 'quota';
  if (isApiError(error)) return error.status >= 500 ? 'network' : 'unauthorized';
  // fetch rejects with a TypeError when the network or the server cannot be reached.
  if (error instanceof TypeError) return 'network';
  // The worker's or the writable's trouble (a locked file, a short write …).
  return 'storage';
}

/**
 * Keeps why the fetch stopped: errors of the server side wait for the next sync, those of the
 * device or the network for FETCH_RETRY_MS (sync.ts); a file prepared again starts over.
 */
async function recordFetchError(id: string, error: unknown): Promise<void> {
  const code = fetchErrorCode(error);
  const restart = code === 'size' || code === 'changed';
  if (restart) await truncateMedia(id, 0).catch(() => undefined);
  await updateDownload(
    id,
    (download) =>
      download && {
        ...download,
        fetchError: code,
        fetchErrorAt: Date.now(),
        ...(restart
          ? {
              bytesWritten: 0,
              subtitlesStored: [],
              manifest: undefined,
              fileEtag: undefined,
              fileReadyAt: undefined,
            }
          : {}),
        updatedAt: Date.now(),
      },
  );
}

const patch = (id: string, change: Partial<StoredDownload>, progress = false) =>
  updateDownload(id, (download) => download && { ...download, ...change, updatedAt: Date.now() }, {
    progress,
  });

/** Manifest, file, subtitles and poster of one download; then it counts as complete. */
async function fetchDownload(
  download: StoredDownload,
  session: SessionInfo | null | undefined,
  signal: AbortSignal,
): Promise<void> {
  const { id } = download;
  let manifest = download.manifest;
  let bytes = download.bytesWritten;
  let subtitles = download.subtitlesStored;
  let etag = download.fileEtag;
  // The server prepared the copy again since these bytes came (another readyAt, O1).
  let restart =
    download.fileReadyAt !== undefined &&
    download.readyAt !== undefined &&
    download.fileReadyAt !== download.readyAt;
  let reset = false;
  if (canFetchManifest(session, download)) {
    let fresh: DownloadManifest | undefined;
    try {
      fresh = await request(
        api.GET('/downloads/{id}/manifest', { params: { path: { id } }, signal }),
      );
    } catch (error) {
      if (isApiError(error, 409)) return;
      if (isApiError(error, 410)) throw new FetchError('evicted', 'manifest 410', 410);
      if (isApiError(error, 404)) throw new FetchError('gone', 'manifest 404', 404);
      if (!manifest) throw error;
    }
    if (fresh) {
      // Prepared again (after an eviction) with other bytes: start over.
      if (manifest && !sameFile(manifest, fresh)) restart = true;
      manifest = fresh;
      // Only a real new start resets the progress: a second tab's stale view must not (W9).
      await patch(id, {
        manifest: fresh,
        fileReadyAt: download.readyAt,
        ...(restart
          ? { bytesWritten: 0, subtitlesStored: [], fileEtag: undefined, complete: false }
          : {}),
      });
      reset = restart;
    }
  }
  if (!manifest) return;
  if (restart) {
    await truncateMedia(id, 0);
    if (!reset) {
      await patch(id, {
        bytesWritten: 0,
        subtitlesStored: [],
        fileEtag: undefined,
        fileReadyAt: download.readyAt,
      });
    }
    bytes = 0;
    subtitles = [];
    etag = undefined;
  }

  const total = manifest.sizeBytes;
  const start = resumeOffset(bytes, await mediaSize(id), total);
  if (start < total) {
    const writer = await openMediaWriter(id);
    const durable = await fetchFile({
      url: apiUrl(manifest.fileUrl),
      total,
      start,
      writer,
      signal,
      // Bytes before `start` came from the copy with this ETag (none: the fetch begins here).
      etag: start > 0 ? etag : undefined,
      onEtag: async (value) => {
        await patch(id, { fileEtag: value });
      },
      onDurable: async (written) => {
        await patch(id, { bytesWritten: written }, true);
      },
    });
    if (durable < total) throw new FetchError('storage', `${durable} of ${total} bytes durable`);
  } else if (bytes !== total) {
    await patch(id, { bytesWritten: total });
  }
  const file = await readFile(id, MEDIA_FILE);
  if (!file || file.size !== total) throw new FetchError('size', `file has ${file?.size ?? 0} B`);

  for (const subtitle of manifest.subtitles) {
    if (subtitles.includes(subtitle.track.id)) continue;
    const response = await fetch(apiUrl(subtitle.url), {
      signal,
      cache: 'no-store',
      credentials: 'same-origin',
    });
    if (response.status === 410) throw new FetchError('evicted', 'subtitle 410', 410);
    if (response.status === 401) throw new FetchError('unauthorized', 'subtitle 401', 401);
    if (response.ok) {
      await writeFile(id, subtitleFile(subtitle.track.id), await response.blob());
    } else if (response.status !== 404) {
      throw new FetchError('network', `subtitle HTTP ${response.status}`, response.status);
    }
    // A 404 subtitle is not on the server any more; the player offers the stored ones only.
    subtitles = [...subtitles, subtitle.track.id];
    await patch(id, { subtitlesStored: subtitles });
  }

  const latest = await getDownload(id);
  if (latest && !latest.posterStored) {
    const images = latest.item?.images;
    const poster =
      images?.poster ?? images?.posterSmall ?? images?.backdropSmall ?? images?.backdrop;
    if (poster) {
      const response = await fetch(apiUrl(poster.url), { signal, credentials: 'same-origin' });
      if (response.ok) await writeFile(id, POSTER_FILE, await response.blob());
      else if (response.status >= 500) {
        throw new FetchError('network', `poster HTTP ${response.status}`, response.status);
      }
    }
  }
  await patch(id, {
    posterStored: true,
    complete: true,
    fetchError: undefined,
    fetchErrorAt: undefined,
  });
  // The next sync reports the file as complete (the server removes its copy 24 h later).
  kickOfflineEngine();
}
