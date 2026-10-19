// Dedicated worker of the offline spike: the "sync" write path writes download blocks into OPFS
// through FileSystemSyncAccessHandle, which only exists in dedicated workers. Classic script for
// the widest browser support. Older WebKit returns promises from the handle methods, so every call
// is awaited. Temporary: removed together with the spike after the M1b decision.

let handle = null;

async function open(name) {
  const root = await navigator.storage.getDirectory();
  const fh = await root.getFileHandle(name, { create: true });
  handle = await fh.createSyncAccessHandle();
  return { size: await handle.getSize() };
}

async function write(offset, buffer) {
  const t0 = performance.now();
  const n = await handle.write(new Uint8Array(buffer), { at: offset });
  if (n !== buffer.byteLength) throw new Error(`short write: ${n} of ${buffer.byteLength} bytes`);
  await handle.flush();
  return { written: n, ms: performance.now() - t0 };
}

async function close() {
  if (!handle) return {};
  await handle.flush();
  const size = await handle.getSize();
  await handle.close();
  handle = null;
  return { size };
}

/** Writes and removes a small probe file to show the sync path really works here. */
async function probe() {
  if (typeof FileSystemFileHandle === 'undefined') return { ok: false, reason: 'no OPFS' };
  if (!('createSyncAccessHandle' in FileSystemFileHandle.prototype)) {
    return { ok: false, reason: 'createSyncAccessHandle missing' };
  }
  const root = await navigator.storage.getDirectory();
  const name = 'spike-probe-sync.tmp';
  const fh = await root.getFileHandle(name, { create: true });
  const h = await fh.createSyncAccessHandle();
  await h.write(new Uint8Array([1, 2, 3, 4]), { at: 0 });
  await h.flush();
  const size = await h.getSize();
  await h.close();
  await root.removeEntry(name);
  return { ok: size === 4, reason: size === 4 ? 'probe write ok' : `probe size ${size}` };
}

self.onmessage = async (event) => {
  const { id, cmd } = event.data;
  try {
    let result;
    if (cmd === 'open') result = await open(event.data.name);
    else if (cmd === 'write') result = await write(event.data.offset, event.data.buffer);
    else if (cmd === 'close') result = await close();
    else if (cmd === 'probe') result = await probe();
    else throw new Error(`unknown command ${cmd}`);
    self.postMessage({ id, result });
  } catch (err) {
    self.postMessage({ id, error: `${err?.name ?? 'Error'}: ${err?.message ?? err}` });
  }
};
