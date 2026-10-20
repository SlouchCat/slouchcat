import type { BlockWriter } from './downloader';

// Files of the downloads in the origin private file system: `/downloads/<id>/media.mp4` (init
// segment and media segments as the server prepared them), `<track>.vtt` per text subtitle and
// `poster` (the poster image; offline images must not depend on the service worker's cache).
// Writes go through the dedicated worker with createSyncAccessHandle; createWritable is the
// fallback for browsers without the sync handle (decision in docs/perf/M1-ios-offline.md).

const ROOT = 'downloads';
export const MEDIA_FILE = 'media.mp4';
export const POSTER_FILE = 'poster';
export const subtitleFile = (track: number) => `${track}.vtt`;

/**
 * With createWritable, progress is durable only at a commit (every this many blocks; each reopen
 * may copy the file, so commits are rare, as in the spike).
 */
const WRITABLE_COMMIT_EVERY = 16;

async function rootDir(): Promise<FileSystemDirectoryHandle> {
  return (await navigator.storage.getDirectory()).getDirectoryHandle(ROOT, { create: true });
}

async function downloadDir(id: string, create: boolean): Promise<FileSystemDirectoryHandle | null> {
  try {
    return await (await rootDir()).getDirectoryHandle(id, { create });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotFoundError') return null;
    throw error;
  }
}

/** A stored file of a download, or null when it is not there. */
export async function readFile(id: string, name: string): Promise<File | null> {
  const dir = await downloadDir(id, false);
  if (!dir) return null;
  try {
    return await (await dir.getFileHandle(name)).getFile();
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotFoundError') return null;
    throw error;
  }
}

/** Size of media.mp4 (0 when missing). */
export async function mediaSize(id: string): Promise<number> {
  return (await readFile(id, MEDIA_FILE))?.size ?? 0;
}

/** Removes every file of a download (revoked, deleted). */
export async function removeFiles(id: string): Promise<void> {
  try {
    await (await rootDir()).removeEntry(id, { recursive: true });
  } catch (error) {
    if (error instanceof DOMException && error.name === 'NotFoundError') return;
    throw error;
  }
}

/** Ids of the download folders in OPFS (to remove folders without a database entry). */
export async function listDownloadFolders(): Promise<string[]> {
  const dir = (await rootDir()) as unknown as { keys(): AsyncIterable<string> };
  const ids: string[] = [];
  for await (const name of dir.keys()) ids.push(name);
  return ids;
}

// ---------- worker RPC (sync write path) ----------

interface WorkerReply {
  id: number;
  result?: Record<string, unknown>;
  error?: { name: string; message: string };
}

/**
 * The worker's error as the DOMException it was (QuotaExceededError, NoModificationAllowedError
 * …), so the engine can tell a full disk or a locked file from the network.
 */
export function workerError(error: { name: string; message: string }): Error {
  return new DOMException(error.message, error.name);
}

interface WorkerClient {
  call(cmd: string, payload?: Record<string, unknown>, transfer?: Transferable[]): Promise<unknown>;
  terminate(): void;
}

function startWorker(): WorkerClient {
  const worker = new Worker(new URL('./opfs.worker.ts', import.meta.url), { type: 'module' });
  const pending = new Map<
    number,
    { resolve: (value: unknown) => void; reject: (e: Error) => void }
  >();
  let nextId = 1;
  worker.onmessage = (event: MessageEvent<WorkerReply>) => {
    const { id, result, error } = event.data;
    const entry = pending.get(id);
    if (!entry) return;
    pending.delete(id);
    if (error) entry.reject(workerError(error));
    else entry.resolve(result);
  };
  worker.onerror = (event) => {
    for (const entry of pending.values()) entry.reject(new Error(`worker: ${event.message}`));
    pending.clear();
  };
  return {
    call: (cmd, payload = {}, transfer = []) =>
      new Promise((resolve, reject) => {
        const id = nextId++;
        pending.set(id, { resolve, reject });
        worker.postMessage({ id, cmd, ...payload }, transfer);
      }),
    terminate: () => {
      worker.terminate();
      for (const entry of pending.values()) entry.reject(new Error('worker terminated'));
      pending.clear();
    },
  };
}

let syncPath: Promise<boolean> | null = null;

/** Whether createSyncAccessHandle works in a worker here (probed once per page). */
export function syncPathAvailable(): Promise<boolean> {
  syncPath ??= (async () => {
    if (typeof Worker === 'undefined') return false;
    const worker = startWorker();
    try {
      const result = (await worker.call('probe')) as { ok?: boolean };
      return result.ok === true;
    } catch {
      return false;
    } finally {
      worker.terminate();
    }
  })();
  return syncPath;
}

const mediaPath = (id: string) => [ROOT, id, MEDIA_FILE];

/** Sync path: each block is written and flushed in the worker, so every block is durable. */
async function syncWriter(id: string): Promise<BlockWriter> {
  const worker = startWorker();
  try {
    await worker.call('open', { path: mediaPath(id) });
  } catch (error) {
    worker.terminate();
    throw error;
  }
  let durable: number | null = null;
  return {
    async write(offset, data) {
      // The buffer is transferred to the worker: it is detached (0 bytes) after postMessage.
      const length = data.byteLength;
      await worker.call('write', { offset, buffer: data }, [data]);
      durable = offset + length;
      return durable;
    },
    async close() {
      try {
        await worker.call('close');
      } finally {
        worker.terminate();
      }
      return durable;
    },
  };
}

/**
 * Writable path: createWritable writes into a swap file that replaces the OPFS file on close, so
 * progress is durable only at commits (every few blocks; each reopen may copy the file).
 */
async function writableWriter(id: string): Promise<BlockWriter> {
  const dir = await downloadDir(id, true);
  if (!dir) throw new Error('download folder missing');
  const file = await dir.getFileHandle(MEDIA_FILE, { create: true });
  let stream: FileSystemWritableFileStream | null = await file.createWritable({
    keepExistingData: true,
  });
  let pending = 0;
  let written = 0;
  let committed: number | null = null;
  const commit = async () => {
    if (!stream) return committed;
    await stream.close();
    stream = null;
    pending = 0;
    committed = written;
    return committed;
  };
  return {
    async write(offset, data) {
      stream ??= await file.createWritable({ keepExistingData: true });
      await stream.write({ type: 'write', position: offset, data });
      written = offset + data.byteLength;
      pending += 1;
      if (pending < WRITABLE_COMMIT_EVERY) return null;
      return commit();
    },
    close: commit,
  };
}

/** A writer for media.mp4 of a download: the sync path when it works, else createWritable. */
export async function openMediaWriter(id: string): Promise<BlockWriter> {
  await downloadDir(id, true);
  return (await syncPathAvailable()) ? syncWriter(id) : writableWriter(id);
}

/** Cuts media.mp4 to `size` bytes (a fetch that starts over after the server prepared it again). */
export async function truncateMedia(id: string, size: number): Promise<void> {
  const dir = await downloadDir(id, true);
  if (!dir) return;
  if (await syncPathAvailable()) {
    const worker = startWorker();
    try {
      await worker.call('open', { path: mediaPath(id) });
      await worker.call('truncate', { size });
      await worker.call('close');
    } finally {
      worker.terminate();
    }
    return;
  }
  const stream = await (
    await dir.getFileHandle(MEDIA_FILE, { create: true })
  ).createWritable({ keepExistingData: true });
  await stream.truncate(size);
  await stream.close();
}

/** Stores a whole small file (subtitle, poster) of a download. */
export async function writeFile(id: string, name: string, data: Blob): Promise<void> {
  const dir = await downloadDir(id, true);
  if (!dir) throw new Error('download folder missing');
  const buffer = await data.arrayBuffer();
  if (await syncPathAvailable()) {
    const worker = startWorker();
    try {
      await worker.call('put', { path: [ROOT, id, name], buffer }, [buffer]);
    } finally {
      worker.terminate();
    }
    return;
  }
  const stream = await (await dir.getFileHandle(name, { create: true })).createWritable();
  await stream.write(buffer);
  await stream.close();
}
