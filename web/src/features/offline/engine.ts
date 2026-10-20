import type { SessionInfo } from '@slouchcat/api-client';
import type { QueryClient } from '@tanstack/react-query';
import { api, apiUrl, isApiError, request } from '@/api/client';
import { sessionQuery } from '@/app/session';
import { FetchError, fetchFile } from './downloader';
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
  waitsForWifi,
} from './sync';
import type { DownloadManifest, OfflineSync, StoredDownload } from './types';

// The offline engine of a page (design 4): it syncs the device (POST /offline/sync) at the start,
// when the network comes back, when the app gets the focus again and every 60 s while it is
// online with an active profile; applies the answer (revoked downloads, allowedProfileIds,
// profile snapshots, rules); fetches ready downloads block by block into OPFS, resuming where it
// stopped; and uploads the progress events of offline playback. One engine per page.

export const SYNC_INTERVAL_MS = 60_000;
/** Most events per upload (OfflineEventBatch.events). */
const EVENT_BATCH = 500;

/** Triggers within this time give one run. */
const KICK_DELAY_MS = 200;

interface Engine {
  queryClient: QueryClient;
  running: boolean;
  again: boolean;
  /** Timer of the next run (kickOfflineEngine). */
  scheduled: number | undefined;
  /** The download whose file is coming in now; one at a time. */
  fetching: { id: string; controller: AbortController } | null;
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
 * Starts the engine of this page (RootLayout). Returns the stop function; a second start while
 * one runs returns that one's stop.
 */
export function startOfflineEngine(queryClient: QueryClient): () => void {
  if (engine) return engine.stop;
  const kick = () => kickOfflineEngine();
  const onOffline = () => cancelFetch();
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
  window.addEventListener('online', kick);
  window.addEventListener('offline', onOffline);
  window.addEventListener('focus', kick);
  document.addEventListener('visibilitychange', onVisible);
  const stop = () => {
    window.clearInterval(timer);
    unsubscribe();
    window.removeEventListener('online', kick);
    window.removeEventListener('offline', onOffline);
    window.removeEventListener('focus', kick);
    document.removeEventListener('visibilitychange', onVisible);
    cancelFetch();
    if (engine?.scheduled !== undefined) window.clearTimeout(engine.scheduled);
    engine = null;
  };
  engine = {
    queryClient,
    running: false,
    again: false,
    scheduled: undefined,
    fetching: null,
    stop,
  };
  void removeOrphanFolders();
  kick();
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

/** Stops the running fetch (of `id`, or any). */
export function cancelFetch(id?: string): void {
  const current = engine?.fetching;
  if (current && (id === undefined || current.id === id)) current.controller.abort();
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
    cancelFetch();
    return;
  }
  let session = sessionOf(current.queryClient);
  // On the offline pages the guard does not ask for the session; ask now that there is network.
  if (session === undefined) {
    session = await current.queryClient.fetchQuery(sessionQuery).catch(() => undefined);
  }
  try {
    if (session?.profile) {
      await sendPendingDeletes(session.account.id);
      await sync(session.account.id, session.profile.id);
    } else if (session === null) {
      await checkRevocations();
    }
    if (session) await flushEvents(session.account.id);
  } catch {
    // Storage or network trouble: the next trigger tries again.
  }
  void fetchLoop(current);
}

// ---------- sync ----------

async function sync(accountId: number, profileId: number): Promise<void> {
  const held = heldDownloads(await listDownloads(), accountId);
  let answer: OfflineSync;
  try {
    answer = await request(api.POST('/offline/sync', { body: { held, network: networkKind() } }));
  } catch (error) {
    // The session ended (a revoked guest): the device asks by its cookie what to delete.
    if (isApiError(error, 401)) await checkRevocations();
    return;
  }
  const now = Date.now();
  const [downloads, profiles, queued] = await Promise.all([
    listDownloads(),
    listProfiles(),
    listAllEvents(),
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
    if (revoked.has(state.id)) continue;
    await updateDownload(state.id, (local) => mergeDownload(local, state, accountId, now));
  }
  for (const snapshot of answer.profiles) {
    await updateProfile(snapshot.id, (local) =>
      mergeProfile(local, snapshot, {
        accountId,
        now,
        offsetMs: result.serverOffsetMs,
        queued,
        rules: snapshot.id === profileId ? answer.rules : undefined,
      }),
    );
  }
  await deleteProfiles(result.removeProfiles);
  await setMeta('accountId', accountId);
  await setMeta('lastSyncAt', now);
  await setMeta('serverOffsetMs', result.serverOffsetMs);
}

/** Removes downloads with their files (revoked, deleted). */
async function removeDownloads(ids: readonly string[]): Promise<void> {
  for (const id of ids) {
    cancelFetch(id);
    await removeFiles(id).catch(() => undefined);
  }
  await deleteDownloads(ids);
}

/**
 * Without a valid session (a revoked guest, AN-4): the device names what it holds and learns by
 * its device cookie what to delete. Profiles of accounts without downloads go as well.
 */
async function checkRevocations(): Promise<void> {
  const downloads = await listDownloads();
  if (downloads.length === 0) return;
  let revoked: string[];
  try {
    revoked = (
      await request(
        api.POST('/offline/revocations', {
          body: { ids: downloads.slice(0, 2000).map((download) => download.id) },
        }),
      )
    ).revoked;
  } catch {
    return;
  }
  if (revoked.length === 0) return;
  await removeDownloads(revoked);
  const left = new Set((await listDownloads()).map((download) => download.accountId));
  const profiles = await listProfiles();
  await deleteProfiles(
    profiles.filter((profile) => !left.has(profile.accountId)).map((profile) => profile.id),
  );
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

// ---------- fetch ----------

async function fetchLoop(current: Engine): Promise<void> {
  if (current.fetching) return;
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
          !waitsForWifi(download, profiles, network) &&
          (download.manifest !== undefined || canFetchManifest(session, download)),
      );
    if (!next) return;
    tried.add(next.id);
    const controller = new AbortController();
    current.fetching = { id: next.id, controller };
    try {
      await fetchDownload(next, session, controller.signal);
    } catch (error) {
      if (!controller.signal.aborted) await recordFetchError(next.id, error);
    } finally {
      if (current.fetching?.id === next.id) current.fetching = null;
    }
    if (controller.signal.aborted) return;
  }
}

/** Errors that wait for the next sync; network errors retry at the next trigger. */
async function recordFetchError(id: string, error: unknown): Promise<void> {
  let code: string;
  if (error instanceof FetchError) code = error.code;
  else if (error instanceof DOMException && error.name === 'QuotaExceededError') code = 'quota';
  else if (error instanceof DOMException || isApiError(error)) code = 'storage';
  else return;
  if (code === 'network') return;
  if (code === 'size') await truncateMedia(id, 0).catch(() => undefined);
  await updateDownload(
    id,
    (download) =>
      download && {
        ...download,
        fetchError: code,
        ...(code === 'size' ? { bytesWritten: 0, manifest: undefined } : {}),
        updatedAt: Date.now(),
      },
  );
}

const patch = (id: string, change: Partial<StoredDownload>) =>
  updateDownload(id, (download) => download && { ...download, ...change, updatedAt: Date.now() });

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
      if (manifest && !sameFile(manifest, fresh)) {
        // Prepared again (after an eviction) with other bytes: start over.
        await truncateMedia(id, 0);
        bytes = 0;
        subtitles = [];
      }
      manifest = fresh;
      await patch(id, { manifest: fresh, bytesWritten: bytes, subtitlesStored: subtitles });
    }
  }
  if (!manifest) return;

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
      onDurable: async (written) => {
        await patch(id, { bytesWritten: written });
      },
    });
    if (durable < total) throw new FetchError('network', `${durable} of ${total} bytes`);
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
  await patch(id, { posterStored: true, complete: true, fetchError: undefined });
  // The next sync reports the file as complete (the server removes its copy 24 h later).
  kickOfflineEngine();
}

/** OPFS folders without a database entry (a download deleted while the page died) go away. */
async function removeOrphanFolders(): Promise<void> {
  try {
    const known = new Set((await listDownloads()).map((download) => download.id));
    for (const id of await listDownloadFolders()) {
      if (!known.has(id)) await removeFiles(id);
    }
  } catch {
    // Best effort.
  }
}
