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

/**
 * Bytes the downloads take on this device: whole files, plus what incomplete ones hold so far.
 * The stored preview sheets (concept 18, 6.8) are not counted: the manifest names no size for a
 * sheet and the record keeps none, so this is the media files without a few megabytes per title;
 * the device's own estimate in the storage panel includes them.
 */
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
