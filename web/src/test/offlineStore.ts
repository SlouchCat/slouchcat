// In-memory stand-in for features/offline/store.ts (jsdom has no IndexedDB). Tests replace the
// module with `vi.mock('@/features/offline/store', () => import('@/test/offlineStore'))` and use
// the helpers below to seed and read it.
import type { MetaKey, MetaValues } from '@/features/offline/db';
import type { QueuedEvent, StoredDownload, StoredProfile } from '@/features/offline/types';

const downloads = new Map<string, StoredDownload>();
const profiles = new Map<number, StoredProfile>();
const events = new Map<number, QueuedEvent>();
const meta = new Map<MetaKey, unknown>();
const listeners = new Set<() => void>();
let nextEvent = 1;

const copy = <T>(value: T): T => structuredClone(value);

function notify(): void {
  for (const listener of listeners) listener();
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export async function listDownloads(): Promise<StoredDownload[]> {
  return Promise.resolve([...downloads.values()].map(copy));
}

export async function getDownload(id: string): Promise<StoredDownload | undefined> {
  const found = downloads.get(id);
  return Promise.resolve(found && copy(found));
}

export async function putDownloads(list: readonly StoredDownload[]): Promise<void> {
  for (const download of list) downloads.set(download.id, copy(download));
  if (list.length > 0) notify();
  return Promise.resolve();
}

export async function updateDownload(
  id: string,
  change: (download: StoredDownload | undefined) => StoredDownload | undefined,
): Promise<StoredDownload | undefined> {
  const current = downloads.get(id);
  const next = change(current && copy(current));
  if (next) {
    downloads.set(id, copy(next));
    notify();
  }
  return Promise.resolve(next ?? current);
}

export async function deleteDownloads(ids: readonly string[]): Promise<void> {
  for (const id of ids) downloads.delete(id);
  if (ids.length > 0) notify();
  return Promise.resolve();
}

export async function listProfiles(): Promise<StoredProfile[]> {
  return Promise.resolve([...profiles.values()].map(copy));
}

export async function getProfile(id: number): Promise<StoredProfile | undefined> {
  const found = profiles.get(id);
  return Promise.resolve(found && copy(found));
}

export async function putProfiles(list: readonly StoredProfile[]): Promise<void> {
  for (const profile of list) profiles.set(profile.id, copy(profile));
  if (list.length > 0) notify();
  return Promise.resolve();
}

export async function updateProfile(
  id: number,
  change: (profile: StoredProfile | undefined) => StoredProfile | undefined,
): Promise<StoredProfile | undefined> {
  const current = profiles.get(id);
  const next = change(current && copy(current));
  if (next) {
    profiles.set(id, copy(next));
    notify();
  }
  return Promise.resolve(next ?? current);
}

export async function deleteProfiles(ids: readonly number[]): Promise<void> {
  for (const id of ids) profiles.delete(id);
  if (ids.length > 0) notify();
  return Promise.resolve();
}

export async function queueEvents(list: readonly QueuedEvent[]): Promise<void> {
  for (const entry of list) {
    const key = nextEvent++;
    events.set(key, { ...copy(entry), key });
  }
  return Promise.resolve();
}

export async function listEvents(accountId: number, limit: number): Promise<QueuedEvent[]> {
  return Promise.resolve(
    [...events.values()]
      .filter((entry) => entry.accountId === accountId)
      .slice(0, limit)
      .map(copy),
  );
}

export async function listAllEvents(): Promise<QueuedEvent[]> {
  return Promise.resolve([...events.values()].map(copy));
}

export async function deleteEvents(keys: readonly number[]): Promise<void> {
  for (const key of keys) events.delete(key);
  return Promise.resolve();
}

export async function getMeta<K extends MetaKey>(key: K): Promise<MetaValues[K] | undefined> {
  return Promise.resolve(meta.get(key) as MetaValues[K] | undefined);
}

export async function setMeta<K extends MetaKey>(key: K, value: MetaValues[K]): Promise<void> {
  meta.set(key, value);
  return Promise.resolve();
}

/** Empties the store (call in beforeEach). */
export function resetOfflineStore(): void {
  downloads.clear();
  profiles.clear();
  events.clear();
  meta.clear();
  nextEvent = 1;
}

/** Synchronous views for assertions. */
export const storedDownloads = () => [...downloads.values()];
export const storedProfiles = () => [...profiles.values()];
export const queuedEvents = () => [...events.values()];
