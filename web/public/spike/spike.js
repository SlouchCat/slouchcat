// Offline spike page logic (plan 10.4): storage facts, Range download into OPFS over two write
// paths with resume, playback from OPFS via Blob URL and via service-worker Range responses with
// seek timing, a playback soak with heartbeats, a persistence listing and an append-only protocol.
// Plain ES module without build step. Temporary: removed after the M1b decision.

import {
  BLOCK_SIZE,
  SEEK_POSITIONS,
  ageDays,
  blockCount,
  blockRange,
  buildSummary,
  formatBytes,
  formatDuration,
  formatLogLine,
  formatRate,
  logEntry,
  parseContentRange,
  rangeHeader,
  resumeBlock,
  seekPassed,
} from './lib.js';

const KEY = {
  log: 'spike.log',
  results: 'spike.results',
  soak: 'spike.soak',
  heartbeat: 'spike.heartbeat',
  ui: 'spike.ui',
  dlPrefix: 'spike.dl.',
};
const HEARTBEAT_MS = 10000;
const BLOCK_TIMEOUT_MS = 120000;
const EVENT_TIMEOUT_MS = 20000;
const LOG_SHOWN = 300;

const $ = (id) => document.getElementById(id);
const video = $('video');

// ---------- localStorage helpers ----------

function readJSON(key, fallback) {
  try {
    const raw = localStorage.getItem(key);
    return raw ? JSON.parse(raw) : fallback;
  } catch {
    return fallback;
  }
}

let storageWarned = false;
function writeJSON(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch (err) {
    if (!storageWarned) {
      storageWarned = true;
      const p = $('insecure');
      p.hidden = false;
      p.textContent = `localStorage write failed (${errText(err)}); export the protocol now.`;
    }
  }
}

function errText(err) {
  if (err == null) return 'unknown error';
  return `${err.name ?? 'Error'}: ${err.message ?? err}`;
}

function sleep(ms, signal) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, ms);
    signal?.addEventListener('abort', () => {
      clearTimeout(timer);
      reject(signal.reason ?? new DOMException('aborted', 'AbortError'));
    });
  });
}

// ---------- protocol (append-only) ----------

const logCache = readJSON(KEY.log, []);

function log(type, msg, data) {
  const entry = logEntry(type, msg, data);
  logCache.push(entry);
  writeJSON(KEY.log, logCache);
  prependLogLine(entry);
  if (isInterruption(entry)) renderInterruptions();
  scheduleSummary();
}

function isInterruption(e) {
  return e.type === 'interrupt' || e.type === 'error' || e.type === 'lifecycle';
}

function prependLogLine(entry) {
  const li = document.createElement('li');
  li.textContent = formatLogLine(entry);
  if (entry.type === 'error') li.className = 'bad';
  if (entry.type === 'result') li.className = 'ok';
  const list = $('log');
  list.prepend(li);
  while (list.children.length > LOG_SHOWN) list.lastChild.remove();
}

function renderLog() {
  $('log').replaceChildren();
  for (const e of logCache.slice(-LOG_SHOWN)) prependLogLine(e);
}

function renderInterruptions() {
  const list = $('interruptions');
  list.replaceChildren();
  for (const e of logCache.filter(isInterruption).slice(-50).reverse()) {
    const li = document.createElement('li');
    li.textContent = formatLogLine(e);
    list.append(li);
  }
}

// ---------- results ----------

const results = readJSON(KEY.results, {});
results.seeks ??= [];
results.soaks ??= [];
results.playChecks ??= [];
function saveResults() {
  writeJSON(KEY.results, results);
  scheduleSummary();
}

// ---------- UI settings ----------

const ui = readJSON(KEY.ui, {});
for (const id of ['base', 'variant', 'path', 'commit-every', 'soak-minutes', 'soak-method']) {
  if (ui[id] != null) $(id).value = ui[id];
  $(id).addEventListener('change', () => {
    ui[id] = $(id).value;
    writeJSON(KEY.ui, ui);
    renderDlStats();
  });
}

// ---------- environment ----------

const env = {
  ua: navigator.userAgent,
  secure: window.isSecureContext,
  standalone:
    window.matchMedia('(display-mode: standalone)').matches || navigator.standalone === true,
  persistResult: null,
  persisted: null,
  quota: null,
  usage: null,
  opfs: typeof navigator.storage?.getDirectory === 'function',
  writableFeature:
    typeof FileSystemFileHandle !== 'undefined' &&
    'createWritable' in FileSystemFileHandle.prototype,
  syncProbe: null,
  writableProbe: null,
  swControlled: Boolean(navigator.serviceWorker?.controller),
  performanceMemory: Boolean(performance.memory),
  wakeLock: 'wakeLock' in navigator,
};

async function refreshStorage() {
  try {
    if (navigator.storage?.persisted) env.persisted = await navigator.storage.persisted();
    if (navigator.storage?.estimate) {
      const est = await navigator.storage.estimate();
      env.quota = est.quota ?? null;
      env.usage = est.usage ?? null;
    }
  } catch (err) {
    log('error', `storage query failed: ${errText(err)}`);
  }
  env.swControlled = Boolean(navigator.serviceWorker?.controller);
  renderEnv();
}

async function requestPersist(trigger) {
  if (!navigator.storage?.persist) {
    env.persistResult = 'unsupported';
    log('storage', `persist() unsupported (${trigger})`);
    return;
  }
  env.persistResult = 'pending (browser may be asking)';
  renderEnv();
  try {
    const granted = await navigator.storage.persist();
    env.persistResult = String(granted);
    log('storage', `persist() -> ${granted} (${trigger})`);
  } catch (err) {
    env.persistResult = `error ${errText(err)}`;
    log('error', `persist() failed: ${errText(err)}`);
  }
  await refreshStorage();
}

function renderEnv() {
  const rows = [
    ['User agent', env.ua],
    ['Secure context', String(env.secure)],
    ['Installed PWA (display-mode: standalone)', String(env.standalone)],
    ['persist() result', env.persistResult ?? 'not requested yet'],
    ['persisted()', String(env.persisted)],
    ['Quota', `${formatBytes(env.quota)} (${env.quota ?? 'n/a'} B)`],
    ['Usage', `${formatBytes(env.usage)} (${env.usage ?? 'n/a'} B)`],
    ['OPFS (storage.getDirectory)', String(env.opfs)],
    ['createSyncAccessHandle in worker', env.syncProbe ?? 'probing…'],
    ['createWritable (FileSystemWritableFileStream)', env.writableProbe ?? 'probing…'],
    ['Service worker controls this page', String(env.swControlled)],
    ['performance.memory', String(env.performanceMemory)],
    ['Screen wake lock API', String(env.wakeLock)],
  ];
  const dl = $('env');
  dl.replaceChildren();
  for (const [k, v] of rows) {
    const dt = document.createElement('dt');
    dt.textContent = k;
    const dd = document.createElement('dd');
    dd.textContent = v;
    dl.append(dt, dd);
  }
  scheduleSummary();
}

// ---------- worker RPC (sync write path) ----------

function startWorker() {
  const worker = new Worker(new URL('worker.js', import.meta.url));
  const pending = new Map();
  let nextId = 1;
  worker.onmessage = (event) => {
    const { id, result, error } = event.data;
    const p = pending.get(id);
    if (!p) return;
    pending.delete(id);
    if (error) p.reject(new Error(error));
    else p.resolve(result);
  };
  worker.onerror = (event) => {
    for (const p of pending.values()) p.reject(new Error(`worker error: ${event.message}`));
    pending.clear();
  };
  const call = (cmd, payload = {}, transfer = []) =>
    new Promise((resolve, reject) => {
      const id = nextId++;
      pending.set(id, { resolve, reject });
      worker.postMessage({ id, cmd, ...payload }, transfer);
    });
  return { call, terminate: () => worker.terminate() };
}

async function probeWritePaths() {
  try {
    const w = startWorker();
    const r = await w.call('probe');
    w.terminate();
    env.syncProbe = r.ok ? `yes (${r.reason})` : `no (${r.reason})`;
  } catch (err) {
    env.syncProbe = `no (${errText(err)})`;
  }
  if (!env.opfs || !env.writableFeature) {
    env.writableProbe = `no (${env.opfs ? 'createWritable missing' : 'no OPFS'})`;
  } else {
    const name = 'spike-probe-writable.tmp';
    try {
      const root = await navigator.storage.getDirectory();
      const fh = await root.getFileHandle(name, { create: true });
      const ws = await fh.createWritable();
      await ws.write(new Uint8Array([1, 2, 3, 4]));
      await ws.close();
      const size = (await fh.getFile()).size;
      await root.removeEntry(name);
      env.writableProbe = size === 4 ? 'yes (probe write ok)' : `no (probe size ${size})`;
    } catch (err) {
      env.writableProbe = `no (${errText(err)})`;
    }
  }
  log('storage', 'write path probes', { sync: env.syncProbe, writable: env.writableProbe });
  renderEnv();
}

// ---------- download ----------

const fileName = (variant, path) => `spike-${variant}-${path}.mp4`;
const dlKey = (variant, path) => `${KEY.dlPrefix}${variant}-${path}`;
const getDl = (variant, path) => readJSON(dlKey(variant, path), null);
const saveDl = (st) => writeJSON(dlKey(st.variant, st.path), st);

function allDownloads() {
  const list = [];
  for (let i = 0; i < localStorage.length; i++) {
    const k = localStorage.key(i);
    if (k?.startsWith(KEY.dlPrefix)) list.push(readJSON(k, null));
  }
  return list.filter(Boolean).sort((a, b) => a.name.localeCompare(b.name));
}

function apiUrl(variant) {
  const base = $('base').value.trim().replace(/\/+$/, '') || location.origin;
  return `${base}/api/v1/spike/${variant}`;
}

/**
 * Total size from the Content-Range of a 1-byte GET (the spike route answers GET only). A
 * cross-origin page cannot read Content-Range (not CORS-exposed), so HEAD Content-Length is the
 * fallback there.
 */
async function fetchTotal(url) {
  const res = await fetch(url, { headers: { Range: 'bytes=0-0' }, cache: 'no-store' });
  await res.body?.cancel(); // never read a full 1.5 GB body if the Range was ignored
  if (res.status !== 206 && res.status !== 200) throw new Error(`GET ${url} -> HTTP ${res.status}`);
  const cr = parseContentRange(res.headers.get('Content-Range'));
  if (cr?.total) return cr.total;
  const head = await fetch(url, { method: 'HEAD', cache: 'no-store' });
  const len = Number(head.headers.get('Content-Length'));
  if (head.ok && len > 0) return len;
  throw new Error('size unknown: no readable Content-Range and no HEAD Content-Length');
}

async function fetchBlock(url, range, signal) {
  signal.throwIfAborted(); // a listener added now would never see an earlier abort
  const ctrl = new AbortController();
  const onAbort = () => ctrl.abort(signal.reason);
  signal.addEventListener('abort', onAbort);
  const timer = setTimeout(
    () => ctrl.abort(new Error(`block timeout ${BLOCK_TIMEOUT_MS / 1000} s`)),
    BLOCK_TIMEOUT_MS,
  );
  try {
    const res = await fetch(url, {
      headers: { Range: rangeHeader(range.start, range.end) },
      cache: 'no-store',
      signal: ctrl.signal,
    });
    if (res.status !== 206) throw new Error(`HTTP ${res.status} instead of 206`);
    const cr = parseContentRange(res.headers.get('Content-Range'));
    if (cr && (cr.start !== range.start || cr.end !== range.end)) {
      throw new Error(
        `Content-Range ${cr.start}-${cr.end} != requested ${range.start}-${range.end}`,
      );
    }
    const buf = await res.arrayBuffer();
    if (buf.byteLength !== range.length) {
      throw new Error(`body ${buf.byteLength} B != ${range.length} B`);
    }
    return { buf, total: cr?.total ?? null };
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', onAbort);
  }
}

/** Sync path: every block is written and flushed in the worker, so each block is durable. */
async function openSyncWriter(name) {
  const w = startWorker();
  await w.call('open', { name });
  return {
    async write(index, offset, buf) {
      await w.call('write', { offset, buffer: buf }, [buf]);
      return index + 1;
    },
    async close() {
      try {
        await w.call('close');
      } finally {
        w.terminate();
      }
      return null;
    },
  };
}

/**
 * Writable path: createWritable writes into a swap file that only replaces the OPFS file on
 * close(), so progress is durable only at commits (every commitEvery blocks). Each reopen with
 * keepExistingData may copy the file; commit and reopen times are logged as spike data.
 */
async function openWritableWriter(fh, commitEvery) {
  let ws = await fh.createWritable({ keepExistingData: true });
  let pending = 0;
  let last = null;
  async function commit() {
    const t0 = performance.now();
    await ws.close();
    const closeMs = performance.now() - t0;
    const blocks = pending;
    pending = 0;
    return { closeMs, blocks };
  }
  return {
    async write(index, offset, buf) {
      await ws.write({ type: 'write', position: offset, data: buf });
      pending++;
      last = index + 1;
      if (pending < commitEvery) return null;
      const c = await commit();
      const t0 = performance.now();
      ws = await fh.createWritable({ keepExistingData: true });
      log('commit', `createWritable commit up to block ${last}`, {
        blocks: c.blocks,
        closeMs: Math.round(c.closeMs),
        reopenMs: Math.round(performance.now() - t0),
      });
      return last;
    },
    async close() {
      const c = await commit();
      if (c.blocks > 0) {
        log('commit', `createWritable final commit up to block ${last}`, {
          blocks: c.blocks,
          closeMs: Math.round(c.closeMs),
        });
      }
      return last;
    },
  };
}

let active = null; // { st, ctrl, sessionBytes, sessionStart, lastTick }
let unloading = false;

function tick() {
  if (!active) return;
  const now = performance.now();
  active.st.activeMs = (active.st.activeMs ?? 0) + (now - active.lastTick);
  active.lastTick = now;
}

async function runDownload(trigger) {
  if (active) {
    log('info', 'a download is already running');
    return;
  }
  if (!env.opfs) {
    log('error', 'OPFS is not available on this page (secure context needed)');
    return;
  }
  const variant = $('variant').value;
  const path = $('path').value;
  const name = fileName(variant, path);
  const st = getDl(variant, path) ?? {
    variant,
    path,
    name,
    blockSize: BLOCK_SIZE,
    total: null,
    done: 0,
    attempts: 0,
    resumes: 0,
    retries: 0,
    bytesFetched: 0,
    activeMs: 0,
    createdAt: new Date().toISOString(),
  };
  if (st.completedAt) {
    log('info', `${name} is already complete; delete the file or reset progress to download again`);
    return;
  }
  const ctrl = new AbortController();
  const now = performance.now();
  active = { st, ctrl, sessionBytes: 0, sessionStart: now, lastTick: now };
  st.attempts += 1;
  renderButtons();
  const url = apiUrl(variant);
  let writer = null;
  try {
    if (!st.total) st.total = await fetchTotal(url);
    const n = blockCount(st.total, BLOCK_SIZE);
    const root = await navigator.storage.getDirectory();
    const fh = await root.getFileHandle(name, { create: true });
    const size = (await fh.getFile()).size;
    const next = resumeBlock(st.done ?? 0, size, st.total, BLOCK_SIZE);
    if (next !== (st.done ?? 0)) {
      log(
        'warn',
        `progress said ${st.done} blocks but the file holds ${size} B; resuming at ${next}`,
      );
    }
    if (next > 0) {
      st.resumes += 1;
      log('download', `resume ${name} at block ${next}/${n} (${trigger})`, { url });
    } else {
      log('download', `start ${name}: ${n} blocks of 8 MiB, ${st.total} B (${trigger})`, { url });
    }
    st.done = next;
    st.running = true;
    st.startedAt ??= new Date().toISOString();
    saveDl(st);
    const commitEvery = Math.max(1, Number($('commit-every').value) || 16);
    writer =
      path === 'sync' ? await openSyncWriter(name) : await openWritableWriter(fh, commitEvery);
    for (let i = next; i < n; i++) {
      ctrl.signal.throwIfAborted();
      const range = blockRange(i, st.total, BLOCK_SIZE);
      let got;
      for (let attempt = 0; ; attempt++) {
        try {
          got = await fetchBlock(url, range, ctrl.signal);
          break;
        } catch (err) {
          if (ctrl.signal.aborted) throw err;
          // A fetch torn down by pagehide is no network failure; the reload counts as a resume.
          if (unloading) throw err;
          st.retries += 1;
          saveDl(st);
          const wait = Math.min(30000, 2000 * 2 ** attempt);
          log('interrupt', `block ${i} failed: ${errText(err)}; retry in ${wait / 1000} s`, {
            download: name,
            block: i,
          });
          await sleep(wait, ctrl.signal);
        }
      }
      if (got.total != null && got.total !== st.total) {
        throw new Error(`server size changed: ${got.total} != ${st.total}`);
      }
      const committed = await writer.write(i, range.start, got.buf);
      active.sessionBytes += range.length;
      st.bytesFetched += range.length;
      tick();
      if (committed != null) {
        st.done = committed;
        saveDl(st);
      }
      if ((i + 1) % 16 === 0) {
        log('progress', `${name} block ${i + 1}/${n}`, {
          session: formatRate(active.sessionBytes, performance.now() - active.sessionStart),
        });
      }
      renderDlStats();
    }
    const last = await writer.close();
    writer = null;
    if (last != null) st.done = last;
    st.done = n;
    const file = await fh.getFile();
    const head = new TextDecoder().decode(await file.slice(4, 8).arrayBuffer());
    st.verified = file.size === st.total && head === 'ftyp';
    st.finalSize = file.size;
    st.completedAt = new Date().toISOString();
    st.running = false;
    tick();
    saveDl(st);
    log(
      st.verified ? 'result' : 'error',
      `download ${st.verified ? 'complete' : 'BROKEN'} ${name}`,
      {
        size: file.size,
        expected: st.total,
        ftyp: head === 'ftyp',
        attempts: st.attempts,
        resumes: st.resumes,
        retries: st.retries,
        activeMs: Math.round(st.activeMs),
        wallMs: Date.parse(st.completedAt) - Date.parse(st.startedAt),
        avg: formatRate(st.bytesFetched, st.activeMs),
      },
    );
  } catch (err) {
    if (writer) {
      try {
        const last = await writer.close();
        if (last != null) st.done = last;
      } catch (closeErr) {
        log('error', `closing the writer failed: ${errText(closeErr)}`);
      }
    }
    st.running = false;
    tick();
    saveDl(st);
    if (ctrl.signal.aborted) log('download', `cancelled ${name} after block ${st.done}`);
    else log('error', `download ${name} failed: ${errText(err)}`);
  } finally {
    active = null;
    renderButtons();
    renderDlStats();
    await refreshFiles();
    await refreshStorage();
  }
}

function renderButtons() {
  $('dl-start').disabled = Boolean(active);
  $('dl-cancel').disabled = !active;
  $('dl-reset').disabled = Boolean(active);
}

function renderDlStats() {
  const variant = $('variant').value;
  const path = $('path').value;
  const st = active?.st ?? getDl(variant, path);
  const rows = [['File', fileName(variant, path)]];
  if (!st) {
    rows.push(['State', 'not started']);
    $('dl-progress').value = 0;
  } else {
    const n = st.total ? blockCount(st.total, BLOCK_SIZE) : null;
    const done = st.done ?? 0;
    let state = 'incomplete';
    if (active?.st === st) state = 'running';
    else if (st.completedAt)
      state = st.verified ? 'complete, size verified' : 'complete, SIZE MISMATCH';
    $('dl-progress').value = n ? done / n : 0;
    rows.push(
      ['State', state],
      ['Blocks', `${done} / ${n ?? '?'}`],
      [
        'Bytes',
        `${formatBytes(Math.min(done * BLOCK_SIZE, st.total ?? 0))} of ${formatBytes(st.total)}`,
      ],
      ['Active time', formatDuration(st.activeMs)],
      [
        'Wall time since first start',
        st.startedAt
          ? formatDuration(
              (st.completedAt ? Date.parse(st.completedAt) : Date.now()) - Date.parse(st.startedAt),
            )
          : 'n/a',
      ],
      ['Throughput (average)', formatRate(st.bytesFetched ?? 0, st.activeMs)],
      [
        'Throughput (this session)',
        active?.st === st
          ? formatRate(active.sessionBytes, performance.now() - active.sessionStart)
          : 'n/a',
      ],
      ['Attempts / resumes / retries', `${st.attempts} / ${st.resumes} / ${st.retries}`],
    );
    if (st.completedAt) rows.push(['Completed', st.completedAt]);
  }
  const dl = $('dl-stats');
  dl.replaceChildren();
  for (const [k, v] of rows) {
    const dt = document.createElement('dt');
    dt.textContent = k;
    const dd = document.createElement('dd');
    dd.textContent = v;
    dl.append(dt, dd);
  }
}

function cancelDownload() {
  if (!active) return;
  log('download', `cancel requested for ${active.st.name}`);
  active.ctrl.abort(new DOMException('cancelled by user', 'AbortError'));
}

function resetProgress() {
  const variant = $('variant').value;
  const path = $('path').value;
  if (!getDl(variant, path)) return;
  if (!window.confirm(`Reset progress of ${fileName(variant, path)}? The OPFS file stays.`)) return;
  localStorage.removeItem(dlKey(variant, path));
  log('download', `progress reset for ${fileName(variant, path)}`);
  renderDlStats();
}

// ---------- playback ----------

let objectUrl = null;
const swStats = { count: 0, bytes: 0, last: '' };

function waitEvent(el, name, timeoutMs = EVENT_TIMEOUT_MS) {
  return new Promise((resolve, reject) => {
    const cleanup = () => {
      clearTimeout(timer);
      el.removeEventListener(name, onEvent);
      el.removeEventListener('error', onError);
    };
    const onEvent = () => {
      cleanup();
      resolve();
    };
    const onError = () => {
      cleanup();
      reject(new Error(`media error ${el.error?.code ?? '?'}: ${el.error?.message ?? ''}`));
    };
    const timer = setTimeout(() => {
      cleanup();
      reject(new Error(`timeout waiting for ${name} (${timeoutMs} ms)`));
    }, timeoutMs);
    el.addEventListener(name, onEvent);
    el.addEventListener('error', onError);
  });
}

async function loadSource(method, name) {
  video.pause();
  if (objectUrl) {
    URL.revokeObjectURL(objectUrl);
    objectUrl = null;
  }
  if (method === 'blob') {
    const root = await navigator.storage.getDirectory();
    const file = await (await root.getFileHandle(name)).getFile();
    objectUrl = URL.createObjectURL(file);
    video.src = objectUrl;
  } else {
    if (!navigator.serviceWorker?.controller) {
      throw new Error('sw.js does not control this page yet; reload the page once');
    }
    swStats.count = 0;
    swStats.bytes = 0;
    video.src = new URL(`opfs/${encodeURIComponent(name)}`, location.href).href;
  }
}

/** Starts playback and resolves with the ms from src assignment to the first playing event. */
async function startPlayback(method, name) {
  const t0 = performance.now();
  const playing = waitEvent(video, 'playing');
  await loadSource(method, name);
  await video.play();
  await playing;
  return performance.now() - t0;
}

/** Seeks to pct of the duration; ms until seeked and until playback advances again. */
async function measureSeek(pct) {
  const target = video.duration * pct;
  if (!Number.isFinite(target)) {
    return { pct, target: null, ok: false, error: `duration ${video.duration} not finite` };
  }
  if (video.paused) await video.play();
  const t0 = performance.now();
  try {
    const seeked = waitEvent(video, 'seeked');
    video.currentTime = target;
    await seeked;
    const seekedMs = performance.now() - t0;
    const at = video.currentTime;
    // A seek clamped to a shorter, still growing duration lands far from the target.
    if (Math.abs(at - target) > 3) {
      return {
        pct,
        target,
        seekedMs,
        landed: at,
        ok: false,
        error: `landed at ${at.toFixed(1)} s`,
      };
    }
    await new Promise((resolve, reject) => {
      const cleanup = () => {
        clearTimeout(timer);
        video.removeEventListener('playing', done);
        video.removeEventListener('timeupdate', onTime);
        video.removeEventListener('error', onError);
      };
      const done = () => {
        cleanup();
        resolve();
      };
      const onTime = () => {
        if (!video.paused && video.currentTime > at + 0.05) done();
      };
      const onError = () => {
        cleanup();
        reject(new Error(`media error ${video.error?.code ?? '?'}`));
      };
      const timer = setTimeout(() => {
        cleanup();
        reject(new Error('timeout waiting for playback after seek'));
      }, EVENT_TIMEOUT_MS);
      video.addEventListener('playing', done);
      video.addEventListener('timeupdate', onTime);
      video.addEventListener('error', onError);
    });
    return { pct, target, seekedMs, landed: at, playingMs: performance.now() - t0, ok: true };
  } catch (err) {
    return { pct, target, ok: false, error: errText(err) };
  }
}

let playBusy = false;

async function seekTest(method) {
  const name = $('play-file').value;
  if (!name || playBusy) return;
  playBusy = true;
  const run = { t: new Date().toISOString(), file: name, method, startMs: null, seeks: [] };
  try {
    run.startMs = await startPlayback(method, name);
    // Firefox reports a growing duration for fMP4 behind the service worker (sidx not used), so
    // record it at the start and at the end; a changed duration fails the run (lib.seekPassed).
    run.duration = video.duration;
    log('measure', `${method}: playback of ${name} started in ${Math.round(run.startMs)} ms`);
    for (const pct of SEEK_POSITIONS) {
      await sleep(1500);
      const s = await measureSeek(pct);
      run.seeks.push(s);
      log(
        'measure',
        s.ok
          ? `${method}: seek to ${pct * 100} % seeked ${Math.round(s.seekedMs)} ms, playing ${Math.round(s.playingMs)} ms`
          : `${method}: seek to ${pct * 100} % FAILED: ${s.error}`,
      );
    }
  } catch (err) {
    run.error = errText(err);
    log('error', `${method} playback of ${name} failed: ${run.error}`);
  } finally {
    playBusy = false;
  }
  run.durationEnd = video.duration;
  run.durationStable = run.duration === run.durationEnd;
  results.seeks.push(run);
  saveResults();
  log('result', `seek test ${name} via ${method}: ${seekPassed(run) ? 'pass' : 'fail'}`, run);
  renderSeekResults();
}

function renderSeekResults() {
  const list = $('seek-results');
  list.replaceChildren();
  for (const r of results.seeks.slice(-10).reverse()) {
    const li = document.createElement('li');
    const seeks = r.seeks
      .map(
        (s) => `${s.pct * 100} %: ${s.ok ? `${Math.round(s.playingMs)} ms` : `FAIL (${s.error})`}`,
      )
      .join(', ');
    const duration =
      r.duration == null
        ? ''
        : ` (duration ${Math.round(r.duration)} s${r.durationStable === false ? ` -> ${Math.round(r.durationEnd)} s, unstable` : ''})`;
    li.textContent = `${r.t} ${r.file} via ${r.method}${duration}: start ${r.startMs == null ? 'FAIL' : `${Math.round(r.startMs)} ms`}; ${seeks || r.error || ''} -> ${seekPassed(r) ? 'pass' : 'fail'}`;
    li.className = seekPassed(r) ? 'ok' : 'bad';
    list.append(li);
  }
}

function renderSwStatus() {
  $('sw-status').textContent =
    `Service worker controls page: ${Boolean(navigator.serviceWorker?.controller)}. ` +
    `Range responses this source: ${swStats.count} (${formatBytes(swStats.bytes)} requested). ${swStats.last}`;
}

// ---------- soak and heartbeat ----------

let soakWakeLock = null;

function heapMB() {
  return performance.memory ? Math.round(performance.memory.usedJSHeapSize / 1e5) / 10 : null;
}

function onSoakWaiting() {
  const s = readJSON(KEY.soak, null);
  if (!s?.running) return;
  s.waiting = (s.waiting ?? 0) + 1;
  writeJSON(KEY.soak, s);
  log('measure', `soak: waiting event #${s.waiting} at ${video.currentTime.toFixed(1)} s`);
}

async function startSoak() {
  const name = $('play-file').value;
  if (!name || playBusy || readJSON(KEY.soak, null)?.running) return;
  const method = $('soak-method').value;
  const minutes = Math.max(1, Number($('soak-minutes').value) || 30);
  try {
    await startPlayback(method, name);
  } catch (err) {
    log('error', `soak could not start: ${errText(err)}`);
    return;
  }
  const heap = heapMB();
  writeJSON(KEY.soak, {
    running: true,
    startedAt: new Date().toISOString(),
    plannedMs: minutes * 60000,
    file: name,
    method,
    waiting: 0,
    heapStart: heap,
    heapEnd: heap,
    heapMax: heap,
  });
  try {
    soakWakeLock = (await navigator.wakeLock?.request('screen')) ?? null;
  } catch {
    soakWakeLock = null;
  }
  log('soak', `soak started: ${minutes} min, ${name} via ${method}`, {
    heap,
    wakeLock: Boolean(soakWakeLock),
  });
  heartbeat();
}

function finishSoak(outcome) {
  const s = readJSON(KEY.soak, null);
  if (!s?.running) return;
  s.running = false;
  s.outcome = outcome;
  s.endedAt = new Date().toISOString();
  s.playedMs = Date.parse(s.endedAt) - Date.parse(s.startedAt);
  s.mediaTime = video.currentTime;
  writeJSON(KEY.soak, s);
  results.soaks.push(s);
  saveResults();
  soakWakeLock?.release().catch(() => {});
  soakWakeLock = null;
  video.pause();
  log('result', `soak ${outcome}`, s);
  renderSoak();
}

function heartbeat() {
  const s = readJSON(KEY.soak, null);
  const beat = {
    t: new Date().toISOString(),
    clean: false,
    soak: Boolean(s?.running),
    download: active?.st.name ?? null,
    mediaTime: video.currentTime,
    heapMB: heapMB(),
  };
  writeJSON(KEY.heartbeat, beat);
  if (s?.running) {
    s.lastHeartbeat = beat.t;
    s.heapEnd = beat.heapMB;
    if (beat.heapMB != null) s.heapMax = Math.max(s.heapMax ?? 0, beat.heapMB);
    s.playedMs = Date.now() - Date.parse(s.startedAt);
    s.mediaTime = video.currentTime;
    writeJSON(KEY.soak, s);
    if (s.playedMs >= s.plannedMs) finishSoak('survived');
  }
  renderSoak();
}

function renderSoak() {
  const s = readJSON(KEY.soak, null);
  if (!s) {
    $('soak-status').textContent = 'No soak run yet.';
    return;
  }
  $('soak-status').textContent = s.running
    ? `Running since ${s.startedAt}: ${formatDuration(s.playedMs ?? 0)} of ${formatDuration(s.plannedMs)}, ` +
      `media ${Math.round(s.mediaTime ?? 0)} s, waiting ${s.waiting}, heap ${s.heapEnd ?? 'n/a'} MB (max ${s.heapMax ?? 'n/a'})`
    : `Last soak ${s.startedAt}: ${s.outcome}, played ${formatDuration(s.playedMs)}, waiting ${s.waiting ?? 0}`;
}

/** Called once on load: a soak still marked running means the page died during it. */
function checkPreviousSession() {
  const beat = readJSON(KEY.heartbeat, null);
  $('last-heartbeat').textContent = beat
    ? `Previous session: last heartbeat ${beat.t}` +
      `${beat.clean ? ' (page closed normally)' : ' (no pagehide: crash, kill or reload without unload)'}` +
      `${beat.soak ? ', soak was running' : ''}${beat.download ? `, download ${beat.download} was running` : ''}`
    : 'No previous session heartbeat.';
  if (beat) log('lifecycle', 'page loaded; previous session heartbeat', beat);
  const s = readJSON(KEY.soak, null);
  if (s?.running) {
    s.running = false;
    s.outcome = `interrupted (page reloaded or crashed; last heartbeat ${s.lastHeartbeat ?? 'none'})`;
    s.endedAt = s.lastHeartbeat ?? s.startedAt;
    writeJSON(KEY.soak, s);
    results.soaks.push(s);
    saveResults();
    log('interrupt', `soak ${s.outcome}`, s);
  }
}

// ---------- persistence: OPFS files ----------

let lastFiles = [];

async function listFiles() {
  if (!env.opfs) return [];
  const byName = new Map(allDownloads().map((d) => [d.name, d]));
  const root = await navigator.storage.getDirectory();
  const files = [];
  for await (const [name, handle] of root.entries()) {
    if (handle.kind !== 'file' || name.endsWith('.tmp')) continue;
    const file = await handle.getFile();
    const st = byName.get(name);
    files.push({
      name,
      size: file.size,
      lastModified: new Date(file.lastModified).toISOString(),
      downloadedAt: st?.completedAt ?? null,
      complete: Boolean(st?.completedAt && st.verified && file.size === st.total),
    });
  }
  return files.sort((a, b) => a.name.localeCompare(b.name));
}

async function refreshFiles() {
  try {
    lastFiles = await listFiles();
  } catch (err) {
    log('error', `listing OPFS failed: ${errText(err)}`);
    lastFiles = [];
  }
  renderFiles();
  const select = $('play-file');
  const keep = select.value;
  select.replaceChildren();
  for (const f of lastFiles) {
    const opt = document.createElement('option');
    opt.value = f.name;
    opt.textContent = `${f.name} (${formatBytes(f.size)}${f.complete ? '' : ', incomplete'})`;
    select.append(opt);
  }
  if (lastFiles.some((f) => f.name === keep)) select.value = keep;
  scheduleSummary();
}

function renderFiles() {
  const list = $('files');
  list.replaceChildren();
  if (lastFiles.length === 0) {
    const li = document.createElement('li');
    li.textContent = env.opfs ? 'No files in OPFS.' : 'OPFS not available.';
    list.append(li);
    return;
  }
  for (const f of lastFiles) {
    const li = document.createElement('li');
    const age = ageDays(f.downloadedAt);
    const p = document.createElement('p');
    p.textContent =
      `${f.name}: present, ${formatBytes(f.size)} (${f.size} B), ` +
      `${f.complete ? 'complete' : 'incomplete'}, downloaded ${f.downloadedAt ?? 'unknown'}` +
      `${age == null ? '' : ` (${age.toFixed(1)} days ago)`}, last modified ${f.lastModified}`;
    p.className = f.complete ? 'ok' : '';
    const row = document.createElement('div');
    row.className = 'row';
    const check = document.createElement('button');
    check.textContent = 'Play check';
    check.addEventListener('click', () => playCheck(f.name));
    const del = document.createElement('button');
    del.textContent = 'Delete';
    del.addEventListener('click', () => deleteFile(f.name));
    row.append(check, del);
    li.append(p, row);
    list.append(li);
  }
}

/** "Datei vorhanden und abspielbar": start via Blob URL and seek to the middle once. */
async function playCheck(name) {
  if (playBusy) return;
  playBusy = true;
  const check = { t: new Date().toISOString(), file: name, ok: false };
  try {
    check.startMs = await startPlayback('blob', name);
    const s = await measureSeek(0.5);
    check.seekMs = s.playingMs ?? null;
    check.ok = s.ok;
    if (!s.ok) check.error = s.error;
    video.pause();
  } catch (err) {
    check.error = errText(err);
  } finally {
    playBusy = false;
  }
  results.playChecks.push(check);
  saveResults();
  log(check.ok ? 'result' : 'error', `play check ${name}: ${check.ok ? 'ok' : 'FAIL'}`, check);
  renderFiles();
}

async function deleteFile(name) {
  if (active?.st.name === name) {
    log('info', `${name} is being downloaded; cancel first`);
    return;
  }
  if (!window.confirm(`Delete ${name} from OPFS and its download progress?`)) return;
  try {
    if (video.src) {
      video.removeAttribute('src');
      video.load();
    }
    const root = await navigator.storage.getDirectory();
    await root.removeEntry(name);
    for (const d of allDownloads().filter((x) => x.name === name)) {
      localStorage.removeItem(dlKey(d.variant, d.path));
    }
    log('storage', `deleted ${name}`);
  } catch (err) {
    log('error', `delete ${name} failed: ${errText(err)}`);
  }
  await refreshFiles();
  await refreshStorage();
  renderDlStats();
}

// ---------- summary and export ----------

let summaryTimer = null;
function scheduleSummary() {
  clearTimeout(summaryTimer);
  summaryTimer = setTimeout(renderSummary, 200);
}

function summaryText() {
  return buildSummary({
    env,
    downloads: allDownloads(),
    results,
    files: lastFiles,
    interruptions: logCache.filter(
      (e) => e.type === 'interrupt' || (e.type === 'lifecycle' && e.data?.download),
    ).length,
  });
}

function renderSummary() {
  $('summary').textContent = summaryText();
}

function exportObject() {
  return {
    exportedAt: new Date().toISOString(),
    summary: summaryText(),
    env,
    downloads: allDownloads(),
    files: lastFiles,
    results,
    log: logCache,
  };
}

async function copyText(text, what) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    document.body.append(ta);
    ta.select();
    document.execCommand('copy');
    ta.remove();
  }
  log('info', `${what} copied to the clipboard`);
}

function exportJSON() {
  const blob = new Blob([JSON.stringify(exportObject(), null, 2)], { type: 'application/json' });
  const a = $('export-link');
  if (a.href) URL.revokeObjectURL(a.href);
  a.href = URL.createObjectURL(blob);
  a.download = `slouchcat-spike-${new Date().toISOString().replace(/[:.]/g, '-')}.json`;
  a.click();
  log('info', `protocol exported as ${a.download}`);
}

// ---------- lifecycle and interruption events ----------

function lifecycle(msg, extra = {}) {
  log('lifecycle', msg, { download: active?.st.name ?? null, playing: !video.paused, ...extra });
}

document.addEventListener('visibilitychange', () =>
  lifecycle(`visibility ${document.visibilityState}`),
);
window.addEventListener('pagehide', (e) => {
  unloading = !e.persisted;
  lifecycle('pagehide', { persisted: e.persisted });
  const beat = readJSON(KEY.heartbeat, {});
  writeJSON(KEY.heartbeat, { ...beat, t: new Date().toISOString(), clean: true });
});
window.addEventListener('pageshow', (e) => {
  unloading = false;
  if (e.persisted) lifecycle('pageshow from back/forward cache');
});
document.addEventListener('freeze', () => lifecycle('freeze'));
document.addEventListener('resume', () => lifecycle('resume'));
window.addEventListener('online', () => lifecycle('online'));
window.addEventListener('offline', () => lifecycle('offline'));
window.addEventListener('error', (e) => log('error', `uncaught: ${e.message}`));
window.addEventListener('unhandledrejection', (e) =>
  log('error', `unhandled rejection: ${errText(e.reason)}`),
);
video.addEventListener('waiting', onSoakWaiting);

// ---------- service worker ----------

async function registerServiceWorker() {
  if (!('serviceWorker' in navigator)) {
    log('storage', 'service workers unsupported here');
    return;
  }
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    env.swControlled = Boolean(navigator.serviceWorker.controller);
    log('storage', `service worker controller changed: ${env.swControlled}`);
    renderEnv();
    renderSwStatus();
  });
  navigator.serviceWorker.addEventListener('message', (event) => {
    const m = event.data;
    if (m?.type !== 'range') return;
    if (m.error) {
      swStats.last = `Last: ${m.name} ${m.error}`;
    } else {
      swStats.count += 1;
      swStats.bytes += m.bytes ?? 0;
      swStats.last = `Last: ${m.range ?? 'no Range'} -> ${m.status}${m.ms == null ? '' : ` in ${m.ms} ms`}`;
    }
    renderSwStatus();
  });
  try {
    const reg = await navigator.serviceWorker.register(new URL('sw.js', import.meta.url).href, {
      scope: '/spike/',
      updateViaCache: 'none',
    });
    log('storage', `sw.js registered, scope ${reg.scope}`);
  } catch (err) {
    log('error', `sw.js registration failed: ${errText(err)}`);
  }
  renderSwStatus();
}

// ---------- wiring ----------

$('persist').addEventListener('click', () => requestPersist('button'));
$('refresh-all').addEventListener('click', async () => {
  await refreshStorage();
  await refreshFiles();
  renderDlStats();
});
$('copy-summary').addEventListener('click', () => copyText(summaryText(), 'summary'));
$('dl-start').addEventListener('click', () => runDownload('button'));
$('dl-cancel').addEventListener('click', cancelDownload);
$('dl-reset').addEventListener('click', resetProgress);
$('seek-blob').addEventListener('click', () => seekTest('blob'));
$('seek-sw').addEventListener('click', () => seekTest('sw'));
$('soak-start').addEventListener('click', startSoak);
$('soak-stop').addEventListener('click', () => finishSoak('stopped early by user'));
$('export').addEventListener('click', exportJSON);
$('copy-log').addEventListener('click', () =>
  copyText(JSON.stringify(exportObject(), null, 2), 'protocol JSON'),
);

async function init() {
  $('insecure').hidden = window.isSecureContext;
  renderLog();
  renderInterruptions();
  renderSeekResults();
  renderButtons();
  renderEnv();
  checkPreviousSession();
  log('lifecycle', 'page load', {
    url: location.href,
    standalone: env.standalone,
    secure: env.secure,
  });
  heartbeat();
  setInterval(heartbeat, HEARTBEAT_MS);
  await registerServiceWorker();
  await refreshStorage();
  // Not awaited: Firefox asks the user and the promise stays pending until they answer.
  requestPersist('on load');
  await refreshFiles();
  log(
    'persistence',
    `on load: ${lastFiles.length} file(s) in OPFS`,
    lastFiles.map((f) => ({
      name: f.name,
      size: f.size,
      complete: f.complete,
      downloadedAt: f.downloadedAt,
      ageDays: f.downloadedAt ? Number(ageDays(f.downloadedAt).toFixed(2)) : null,
    })),
  );
  renderDlStats();
  renderSoak();
  await probeWritePaths();
  // A download that was running when the page went away resumes on its own (reload, app kill).
  const pending = allDownloads().find((d) => d.running && !d.completedAt);
  if (pending) {
    $('variant').value = pending.variant;
    $('path').value = pending.path;
    renderDlStats();
    runDownload('auto-resume after reload');
  }
}

init();
