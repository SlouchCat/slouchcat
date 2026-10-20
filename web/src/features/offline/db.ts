import { openDB, type DBSchema, type IDBPDatabase } from 'idb';
import type { QueuedEvent, StoredDownload, StoredProfile } from './types';

/** Values of the `meta` store, by key. */
export interface MetaValues {
  /**
   * The account last signed in on this device: the offline library shows only its profiles
   * (decision of 2026-10-20 on shared devices).
   */
  accountId: number;
  /**
   * That account signed out here: its downloads stay (O-6) but the offline library keeps them
   * locked until the same account signs in again.
   */
  signedOut: boolean;
  /**
   * Downloads deleted on this device lately (id → device time): a sync answer sent before the
   * delete must not bring them back (K5).
   */
  recentDeletes: Record<string, number>;
  /** Device time of the last successful sync. */
  lastSyncAt: number;
  /** Server time minus device time at the last sync (OfflineSync.serverTime). */
  serverOffsetMs: number;
  /** `navigator.storage.persist()` was asked once (first download). */
  persistRequested: boolean;
}

export type MetaKey = keyof MetaValues;

/**
 * IndexedDB of the offline feature (plan 4, `idb`): downloads (server state, title data, manifest,
 * progress in bytes), profiles (sync snapshot, local PIN verifier, watch time), events (progress of
 * offline playback waiting for the upload) and meta (account, last sync, clock offset).
 */
export interface OfflineSchema extends DBSchema {
  downloads: { key: string; value: StoredDownload; indexes: { byAccount: number } };
  profiles: { key: number; value: StoredProfile; indexes: { byAccount: number } };
  events: { key: number; value: QueuedEvent; indexes: { byAccount: number } };
  meta: { key: MetaKey; value: MetaValues[MetaKey] };
}

export const DB_NAME = 'slouchcat-offline';
export const DB_VERSION = 1;

let opening: Promise<IDBPDatabase<OfflineSchema>> | null = null;

/** The database, opened once per page. Another tab upgrading it closes this connection. */
export function offlineDb(): Promise<IDBPDatabase<OfflineSchema>> {
  opening ??= openDB<OfflineSchema>(DB_NAME, DB_VERSION, {
    // One step per version, so a later version only adds its own changes (K7).
    upgrade(db, oldVersion) {
      if (oldVersion < 1) {
        const downloads = db.createObjectStore('downloads', { keyPath: 'id' });
        downloads.createIndex('byAccount', 'accountId');
        const profiles = db.createObjectStore('profiles', { keyPath: 'id' });
        profiles.createIndex('byAccount', 'accountId');
        const events = db.createObjectStore('events', { keyPath: 'key', autoIncrement: true });
        events.createIndex('byAccount', 'accountId');
        db.createObjectStore('meta');
      }
    },
    blocking() {
      void opening?.then((db) => db.close());
      opening = null;
    },
    terminated() {
      opening = null;
    },
  }).catch((error: unknown) => {
    opening = null;
    throw error;
  });
  return opening;
}
