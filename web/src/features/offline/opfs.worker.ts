// Dedicated worker of the offline downloads: writes into the origin private file system through
// FileSystemSyncAccessHandle, which only exists in dedicated workers (spike worker.js, decision in
// docs/perf/M1-ios-offline.md). Every write is flushed, so each block is durable when the answer
// comes. Older WebKit returns promises from the handle methods, so every call is awaited. The
// worker imports nothing: it stays one small file in the precache.

interface SyncAccessHandle {
  write(buffer: ArrayBufferView | ArrayBuffer, options?: { at?: number }): number | Promise<number>;
  getSize(): number | Promise<number>;
  truncate(size: number): void | Promise<void>;
  flush(): void | Promise<void>;
  close(): void | Promise<void>;
}

interface SyncFileHandle {
  createSyncAccessHandle(): Promise<SyncAccessHandle>;
}

type Command =
  | { id: number; cmd: 'open'; path: string[] }
  | { id: number; cmd: 'write'; offset: number; buffer: ArrayBuffer }
  | { id: number; cmd: 'truncate'; size: number }
  | { id: number; cmd: 'close' }
  | { id: number; cmd: 'put'; path: string[]; buffer: ArrayBuffer }
  | { id: number; cmd: 'probe' };

interface WorkerScope {
  onmessage: ((event: MessageEvent<Command>) => void) | null;
  postMessage(message: unknown): void;
}

const scope = self as unknown as WorkerScope;
let handle: SyncAccessHandle | null = null;

/** The file at `path` below the OPFS root; folders and the file are created when missing. */
async function fileHandle(path: string[]): Promise<SyncFileHandle> {
  let dir = await navigator.storage.getDirectory();
  const name = path.at(-1);
  if (!name) throw new Error('empty path');
  for (const part of path.slice(0, -1)) dir = await dir.getDirectoryHandle(part, { create: true });
  return (await dir.getFileHandle(name, { create: true })) as unknown as SyncFileHandle;
}

function current(): SyncAccessHandle {
  if (!handle) throw new Error('no open file');
  return handle;
}

async function open(path: string[]): Promise<{ size: number }> {
  if (handle) await handle.close();
  handle = await (await fileHandle(path)).createSyncAccessHandle();
  return { size: await handle.getSize() };
}

async function write(offset: number, buffer: ArrayBuffer): Promise<{ written: number }> {
  const target = current();
  const written = await target.write(new Uint8Array(buffer), { at: offset });
  if (written !== buffer.byteLength) {
    throw new Error(`short write: ${written} of ${buffer.byteLength} bytes`);
  }
  await target.flush();
  return { written };
}

async function truncate(size: number): Promise<object> {
  const target = current();
  await target.truncate(size);
  await target.flush();
  return {};
}

async function close(): Promise<{ size: number | null }> {
  if (!handle) return { size: null };
  const target = handle;
  handle = null;
  await target.flush();
  const size = await target.getSize();
  await target.close();
  return { size };
}

/** Writes a whole small file (subtitle, poster) in one go. */
async function put(path: string[], buffer: ArrayBuffer): Promise<{ size: number }> {
  const target = await (await fileHandle(path)).createSyncAccessHandle();
  try {
    await target.truncate(0);
    await target.write(new Uint8Array(buffer), { at: 0 });
    await target.flush();
    return { size: await target.getSize() };
  } finally {
    await target.close();
  }
}

/** Writes and removes a probe file: the sync path really works in this browser. */
async function probe(): Promise<{ ok: boolean }> {
  if (typeof FileSystemFileHandle === 'undefined') return { ok: false };
  if (!('createSyncAccessHandle' in FileSystemFileHandle.prototype)) return { ok: false };
  const root = await navigator.storage.getDirectory();
  // One name per probe: two tabs that start together must not lock each other's probe file.
  const unique =
    typeof crypto.randomUUID === 'function' ? crypto.randomUUID() : String(Math.random()).slice(2);
  const name = `probe-${unique}.tmp`;
  const target = await (
    (await root.getFileHandle(name, { create: true })) as unknown as SyncFileHandle
  ).createSyncAccessHandle();
  await target.write(new Uint8Array([1, 2, 3, 4]), { at: 0 });
  await target.flush();
  const size = await target.getSize();
  await target.close();
  await root.removeEntry(name);
  return { ok: size === 4 };
}

function run(command: Command): Promise<object> {
  switch (command.cmd) {
    case 'open':
      return open(command.path);
    case 'write':
      return write(command.offset, command.buffer);
    case 'truncate':
      return truncate(command.size);
    case 'close':
      return close();
    case 'put':
      return put(command.path, command.buffer);
    case 'probe':
      return probe();
  }
}

scope.onmessage = (event) => {
  const command = event.data;
  run(command).then(
    (result) => scope.postMessage({ id: command.id, result }),
    // Name and message separately: the page rebuilds a DOMException (QuotaExceededError,
    // NoModificationAllowedError …) so the engine can tell storage trouble from the network.
    (error: unknown) =>
      scope.postMessage({
        id: command.id,
        error:
          error instanceof Error
            ? { name: error.name, message: error.message }
            : { name: 'Error', message: String(error) },
      }),
  );
};
