// In-memory stand-in for features/offline/opfs.ts (jsdom has no OPFS). Tests replace the module
// with `vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'))`.
import type { BlockWriter } from '@/features/offline/downloader';

export const MEDIA_FILE = 'media.mp4';
export const POSTER_FILE = 'poster';
export const subtitleFile = (track: number) => `${track}.vtt`;

const folders = new Map<string, Map<string, Blob>>();
/** Writers open per download: like a sync access handle, they lock the folder against removal. */
const openWriters = new Map<string, number>();
let writeFailure: Error | null = null;
let removalFailures = 0;

/** The next writes throw `error` (null: they work again), e.g. a QuotaExceededError. */
export function failWrites(error: Error | null): void {
  writeFailure = error;
}

/** The next `count` removals fail as if another tab held the file. */
export function failRemovals(count: number): void {
  removalFailures = count;
}

function folder(id: string): Map<string, Blob> {
  let files = folders.get(id);
  if (!files) {
    files = new Map();
    folders.set(id, files);
  }
  return files;
}

export async function readFile(id: string, name: string): Promise<File | null> {
  const blob = folders.get(id)?.get(name);
  return Promise.resolve(blob ? new File([blob], name) : null);
}

export async function mediaSize(id: string): Promise<number> {
  return Promise.resolve(folders.get(id)?.get(MEDIA_FILE)?.size ?? 0);
}

export async function removeFiles(id: string): Promise<void> {
  if ((openWriters.get(id) ?? 0) > 0 || removalFailures > 0) {
    removalFailures = Math.max(0, removalFailures - 1);
    return Promise.reject(new DOMException('file in use', 'NoModificationAllowedError'));
  }
  folders.delete(id);
  return Promise.resolve();
}

export async function listDownloadFolders(): Promise<string[]> {
  return Promise.resolve([...folders.keys()]);
}

export async function syncPathAvailable(): Promise<boolean> {
  return Promise.resolve(true);
}

export async function openMediaWriter(id: string): Promise<BlockWriter> {
  const files = folder(id);
  let durable: number | null = null;
  let open = true;
  openWriters.set(id, (openWriters.get(id) ?? 0) + 1);
  return Promise.resolve({
    async write(offset: number, data: ArrayBuffer) {
      if (writeFailure) throw writeFailure;
      const current = new Uint8Array(await (files.get(MEDIA_FILE) ?? new Blob()).arrayBuffer());
      const next = new Uint8Array(Math.max(current.length, offset + data.byteLength));
      next.set(current);
      next.set(new Uint8Array(data), offset);
      files.set(MEDIA_FILE, new Blob([next]));
      durable = offset + data.byteLength;
      return durable;
    },
    close: () => {
      if (open) {
        open = false;
        openWriters.set(id, (openWriters.get(id) ?? 1) - 1);
      }
      return Promise.resolve(durable);
    },
  });
}

export async function truncateMedia(id: string, size: number): Promise<void> {
  const files = folder(id);
  const current = files.get(MEDIA_FILE);
  if (current) files.set(MEDIA_FILE, current.slice(0, size));
  return Promise.resolve();
}

export async function writeFile(id: string, name: string, data: Blob): Promise<void> {
  folder(id).set(name, data);
  return Promise.resolve();
}

/** Puts a file in place (seeding a complete download). */
export function seedFile(id: string, name: string, data: BlobPart): void {
  folder(id).set(name, new Blob([data]));
}

export function resetOfflineFiles(): void {
  folders.clear();
  openWriters.clear();
  writeFailure = null;
  removalFailures = 0;
}

export const storedFiles = (id: string) => [...(folders.get(id)?.keys() ?? [])];
