import type { StoredDownload } from './types';

/** What the browser says about the storage of this site (O-7). */
export interface StorageInfo {
  /** Bytes the site uses (all of it: database, files, caches). */
  usage: number | null;
  /** Bytes the site may use. */
  quota: number | null;
  /** The browser promised not to evict the storage (`persist()` granted). */
  persisted: boolean | null;
}

export async function storageInfo(): Promise<StorageInfo> {
  const storage = typeof navigator === 'undefined' ? undefined : navigator.storage;
  const [estimate, persisted] = await Promise.all([
    storage?.estimate?.().catch(() => null) ?? null,
    storage?.persisted?.().catch(() => null) ?? null,
  ]);
  return {
    usage: estimate?.usage ?? null,
    quota: estimate?.quota ?? null,
    persisted,
  };
}

/** Asks for persistent storage (best effort; some browsers ask the viewer). */
export async function requestPersistence(): Promise<boolean> {
  try {
    const storage = navigator.storage;
    if (!storage?.persist) return false;
    if (await storage.persisted()) return true;
    return await storage.persist();
  } catch {
    return false;
  }
}

/** Bytes the downloads take on this device: whole files, plus what incomplete ones hold so far. */
export function downloadsBytes(downloads: readonly StoredDownload[]): number {
  return downloads.reduce(
    (sum, download) =>
      sum +
      (download.complete
        ? (download.manifest?.sizeBytes ?? download.sizeBytes ?? download.bytesWritten)
        : download.bytesWritten),
    0,
  );
}
