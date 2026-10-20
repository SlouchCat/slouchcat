// In-memory stand-in for features/offline/opfs.ts (jsdom has no OPFS). Tests replace the module
// with `vi.mock('@/features/offline/opfs', () => import('@/test/offlineFiles'))`.
import type { BlockWriter } from '@/features/offline/downloader';

export const MEDIA_FILE = 'media.mp4';
export const POSTER_FILE = 'poster';
export const subtitleFile = (track: number) => `${track}.vtt`;

const folders = new Map<string, Map<string, Blob>>();

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
  return Promise.resolve({
    async write(offset: number, data: ArrayBuffer) {
      const current = new Uint8Array(await (files.get(MEDIA_FILE) ?? new Blob()).arrayBuffer());
      const next = new Uint8Array(Math.max(current.length, offset + data.byteLength));
      next.set(current);
      next.set(new Uint8Array(data), offset);
      files.set(MEDIA_FILE, new Blob([next]));
      durable = offset + data.byteLength;
      return durable;
    },
    close: () => Promise.resolve(durable),
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
}

export const storedFiles = (id: string) => [...(folders.get(id)?.keys() ?? [])];
