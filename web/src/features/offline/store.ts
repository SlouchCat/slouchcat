import { offlineDb, type MetaKey, type MetaValues } from './db';
import type { QueuedEvent, StoredDownload, StoredProfile } from './types';

// Reads and writes of the offline database. Every write tells the listeners (pages that show
// downloads) in this tab and, through a BroadcastChannel, in the other tabs of the app.

const listeners = new Set<() => void>();
const CHANNEL = 'slouchcat-offline';
let channel: BroadcastChannel | null = null;

function broadcast(): BroadcastChannel | null {
  if (channel || typeof BroadcastChannel === 'undefined') return channel;
  channel = new BroadcastChannel(CHANNEL);
  channel.onmessage = () => {
    for (const listener of listeners) listener();
  };
  return channel;
}

function notify(): void {
  for (const listener of listeners) listener();
  broadcast()?.postMessage('changed');
}

/** Fetch progress tells the pages at most this often (each listener reads every download, K8). */
const PROGRESS_NOTIFY_MS = 1000;
let notifyTimer: ReturnType<typeof setTimeout> | undefined;

function notifySoon(): void {
  if (notifyTimer !== undefined) return;
  notifyTimer = setTimeout(() => {
    notifyTimer = undefined;
    notify();
  }, PROGRESS_NOTIFY_MS);
}

export interface UpdateOptions {
  /** A progress write (a block of the fetch): the listeners hear of it at most once a second. */
  progress?: boolean;
}

/** Calls `listener` after every change of the offline data; returns the unsubscribe. */
export function subscribe(listener: () => void): () => void {
  broadcast();
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export async function listDownloads(): Promise<StoredDownload[]> {
  return (await offlineDb()).getAll('downloads');
}

export async function getDownload(id: string): Promise<StoredDownload | undefined> {
  return (await offlineDb()).get('downloads', id);
}

export async function putDownloads(downloads: readonly StoredDownload[]): Promise<void> {
  if (downloads.length === 0) return;
  const tx = (await offlineDb()).transaction('downloads', 'readwrite');
  await Promise.all([...downloads.map((download) => tx.store.put(download)), tx.done]);
  notify();
}

/**
 * Read-modify-write of one download in one transaction, so the fetch loop and a sync never
 * overwrite each other's fields. `change` gets the current record (undefined when there is none)
 * and returns the new one, or undefined to leave it as it is.
 */
export async function updateDownload(
  id: string,
  change: (download: StoredDownload | undefined) => StoredDownload | undefined,
  options: UpdateOptions = {},
): Promise<StoredDownload | undefined> {
  const tx = (await offlineDb()).transaction('downloads', 'readwrite');
  const current = await tx.store.get(id);
  const next = change(current);
  if (next) await tx.store.put(next);
  await tx.done;
  if (next) {
    if (options.progress) notifySoon();
    else notify();
  }
  return next ?? current;
}

export async function deleteDownloads(ids: readonly string[]): Promise<void> {
  if (ids.length === 0) return;
  const tx = (await offlineDb()).transaction('downloads', 'readwrite');
  await Promise.all([...ids.map((id) => tx.store.delete(id)), tx.done]);
  notify();
}

export async function listProfiles(): Promise<StoredProfile[]> {
  return (await offlineDb()).getAll('profiles');
}

export async function getProfile(id: number): Promise<StoredProfile | undefined> {
  return (await offlineDb()).get('profiles', id);
}

export async function putProfiles(profiles: readonly StoredProfile[]): Promise<void> {
  if (profiles.length === 0) return;
  const tx = (await offlineDb()).transaction('profiles', 'readwrite');
  await Promise.all([...profiles.map((profile) => tx.store.put(profile)), tx.done]);
  notify();
}

/**
 * Read-modify-write of one profile (snapshot, verifier, lock, watch time) in one transaction;
 * `change` gets undefined when there is no record yet.
 */
export async function updateProfile(
  id: number,
  change: (profile: StoredProfile | undefined) => StoredProfile | undefined,
): Promise<StoredProfile | undefined> {
  const tx = (await offlineDb()).transaction('profiles', 'readwrite');
  const current = await tx.store.get(id);
  const next = change(current);
  if (next) await tx.store.put(next);
  await tx.done;
  if (next) notify();
  return next ?? current;
}

export async function deleteProfiles(ids: readonly number[]): Promise<void> {
  if (ids.length === 0) return;
  const tx = (await offlineDb()).transaction('profiles', 'readwrite');
  await Promise.all([...ids.map((id) => tx.store.delete(id)), tx.done]);
  notify();
}

export async function queueEvents(events: readonly QueuedEvent[]): Promise<void> {
  if (events.length === 0) return;
  const tx = (await offlineDb()).transaction('events', 'readwrite');
  await Promise.all([...events.map((event) => tx.store.add(event)), tx.done]);
}

/** Queued events of an account in the order they were created, at most `limit`. */
export async function listEvents(accountId: number, limit: number): Promise<QueuedEvent[]> {
  const events = await (await offlineDb()).getAllFromIndex('events', 'byAccount', accountId);
  return events.sort((a, b) => (a.key ?? 0) - (b.key ?? 0)).slice(0, limit);
}

/** Every queued event (watch time not yet booked on the server). */
export async function listAllEvents(): Promise<QueuedEvent[]> {
  return (await offlineDb()).getAll('events');
}

export async function deleteEvents(keys: readonly number[]): Promise<void> {
  if (keys.length === 0) return;
  const tx = (await offlineDb()).transaction('events', 'readwrite');
  await Promise.all([...keys.map((key) => tx.store.delete(key)), tx.done]);
}

export async function getMeta<K extends MetaKey>(key: K): Promise<MetaValues[K] | undefined> {
  return (await (await offlineDb()).get('meta', key)) as MetaValues[K] | undefined;
}

export async function setMeta<K extends MetaKey>(key: K, value: MetaValues[K]): Promise<void> {
  await (await offlineDb()).put('meta', value, key);
  // The offline library follows the device's account and its sign-out lock.
  if (key === 'accountId' || key === 'signedOut') notify();
}

/**
 * The account signed out on this device (useLogout): its downloads stay (O-6), but the offline
 * library keeps them locked until the same account signs in again (decision of 2026-10-20).
 */
export async function lockSignedOutAccount(): Promise<void> {
  await setMeta('signedOut', true);
}
