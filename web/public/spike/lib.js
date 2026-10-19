// Pure helpers of the offline spike page (plan 10.4): block planning, range math, formatting and
// the device summary. No DOM and no browser storage here, so web/e2e-spike/lib.test.mjs can run
// them under node --test. Temporary: removed together with the spike after the M1b decision.

export const MiB = 1024 * 1024;
export const BLOCK_SIZE = 8 * MiB;
export const SEEK_THRESHOLD_MS = 2000;
export const SEEK_POSITIONS = [0.25, 0.5, 0.75];

/** Number of blocks needed for total bytes. */
export function blockCount(total, blockSize = BLOCK_SIZE) {
  return Math.ceil(total / blockSize);
}

/** Byte range of block index; end is inclusive like in HTTP Range headers. */
export function blockRange(index, total, blockSize = BLOCK_SIZE) {
  const start = index * blockSize;
  if (index < 0 || start >= total) throw new RangeError(`block ${index} outside ${total} bytes`);
  const end = Math.min(start + blockSize, total) - 1;
  return { start, end, length: end - start + 1 };
}

export function rangeHeader(start, end) {
  return `bytes=${start}-${end}`;
}

/** Parses "bytes 0-99/1234" (Content-Range of a 206). Returns null when absent or malformed. */
export function parseContentRange(value) {
  const m = /^bytes (\d+)-(\d+)\/(\d+|\*)$/.exec((value ?? '').trim());
  if (!m) return null;
  return { start: Number(m[1]), end: Number(m[2]), total: m[3] === '*' ? null : Number(m[3]) };
}

/**
 * Parses a single-range Range request header against a file of size bytes, as the service worker
 * does (sw.js keeps a copy because it runs as a classic script). Returns {start, end} with an
 * inclusive end, or null when the range is malformed or not satisfiable (answer 416).
 */
export function parseRangeRequest(value, size) {
  const m = /^bytes=(\d*)-(\d*)$/.exec((value ?? '').trim());
  if (!m || (m[1] === '' && m[2] === '') || size === 0) return null;
  let start;
  let end;
  if (m[1] === '') {
    const suffix = Number(m[2]);
    if (suffix === 0) return null;
    start = Math.max(0, size - suffix);
    end = size - 1;
  } else {
    start = Number(m[1]);
    end = m[2] === '' ? size - 1 : Math.min(Number(m[2]), size - 1);
  }
  if (start >= size || start > end) return null;
  return { start, end };
}

/**
 * Block to resume from: the completed block count from localStorage, but never past what the OPFS
 * file really holds (a commit may have been lost when the page died).
 */
export function resumeBlock(doneBlocks, fileSize, total, blockSize = BLOCK_SIZE) {
  const n = blockCount(total, blockSize);
  if (fileSize >= total) return Math.min(doneBlocks, n);
  return Math.max(0, Math.min(doneBlocks, Math.floor(fileSize / blockSize), n));
}

export function formatBytes(bytes) {
  if (bytes == null || Number.isNaN(bytes)) return 'n/a';
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(2)} GB`;
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`;
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} kB`;
  return `${bytes} B`;
}

export function formatDuration(ms) {
  if (ms == null || Number.isNaN(ms)) return 'n/a';
  const s = Math.round(ms / 1000);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h > 0) return `${h}h ${String(m).padStart(2, '0')}m ${String(sec).padStart(2, '0')}s`;
  if (m > 0) return `${m}m ${String(sec).padStart(2, '0')}s`;
  return `${(ms / 1000).toFixed(1)}s`;
}

/** Throughput as "12.3 MB/s (98 Mbit/s)". */
export function formatRate(bytes, ms) {
  if (!ms || ms <= 0) return 'n/a';
  const bps = bytes / (ms / 1000);
  return `${(bps / 1e6).toFixed(1)} MB/s (${Math.round((bps * 8) / 1e6)} Mbit/s)`;
}

export function ageDays(fromIso, now = Date.now()) {
  if (!fromIso) return null;
  return (now - Date.parse(fromIso)) / 86400000;
}

/** One protocol entry; the log is append-only and every entry carries an ISO timestamp. */
export function logEntry(type, msg, data, now = new Date()) {
  const entry = { t: now.toISOString(), type, msg };
  if (data !== undefined) entry.data = data;
  return entry;
}

export function formatLogLine(entry) {
  const data = entry.data === undefined ? '' : ` ${JSON.stringify(entry.data)}`;
  return `${entry.t} [${entry.type}] ${entry.msg}${data}`;
}

/** A seek run passes when every position reached playback in under the threshold. */
export function seekPassed(run) {
  return (
    run.durationStable !== false &&
    run.seeks.length === SEEK_POSITIONS.length &&
    run.seeks.every((s) => s.ok && s.playingMs < SEEK_THRESHOLD_MS)
  );
}

function yesNo(v) {
  if (v === true) return 'yes';
  if (v === false) return 'no';
  return 'n/a';
}

/** Latest entry per key, keeping insertion order of first appearance. */
function latestBy(list, key) {
  const map = new Map();
  for (const item of list) map.set(key(item), item);
  return [...map.values()];
}

/**
 * Plain-text device summary for docs/perf/M1-ios-offline.md. env, downloads (list of download
 * states), results ({seeks, soaks, playChecks}) and files (OPFS listing) come from the page.
 */
export function buildSummary({ env, downloads, results, files, interruptions, now = new Date() }) {
  const L = [];
  L.push(`Device summary ${now.toISOString()}`);
  L.push(`- User agent: ${env.ua}`);
  L.push(
    `- Secure context: ${yesNo(env.secure)}; installed PWA (standalone): ${yesNo(env.standalone)}`,
  );
  L.push(`- persist(): ${env.persistResult ?? 'n/a'}; persisted(): ${yesNo(env.persisted)}`);
  L.push(`- Quota: ${formatBytes(env.quota)}; usage: ${formatBytes(env.usage)}`);
  L.push(
    `- createSyncAccessHandle (worker): ${env.syncProbe ?? 'n/a'}; createWritable: ${env.writableProbe ?? 'n/a'}`,
  );
  L.push(`- Service worker controls page: ${yesNo(env.swControlled)}`);
  L.push('Downloads (variant/path):');
  if (downloads.length === 0) L.push('- none');
  for (const d of downloads) {
    const status = d.completedAt
      ? `complete${d.verified ? ', size ok' : ', SIZE MISMATCH'}`
      : `incomplete (${d.done ?? 0}/${d.total ? blockCount(d.total, d.blockSize) : '?'} blocks)`;
    L.push(
      `- ${d.variant}/${d.path}: ${status}, ${formatBytes(d.total)}, attempts ${d.attempts ?? 0}, ` +
        `resumes after reload ${d.resumes ?? 0}, retries after error ${d.retries ?? 0}, ` +
        `active ${formatDuration(d.activeMs)}, avg ${formatRate(d.bytesFetched ?? 0, d.activeMs)}`,
    );
  }
  L.push(`Seek tests (latest per file and way, 25/50/75 %, threshold < ${SEEK_THRESHOLD_MS} ms):`);
  const seeks = latestBy(results.seeks ?? [], (r) => `${r.file}|${r.method}`);
  if (seeks.length === 0) L.push('- none');
  for (const r of seeks) {
    const parts = r.seeks.map(
      (s) =>
        `${Math.round(s.pct * 100)} %: ${s.ok ? `${Math.round(s.playingMs)} ms` : `FAIL (${s.error})`}`,
    );
    const start = r.startMs == null ? 'FAIL' : `${Math.round(r.startMs)} ms`;
    const duration =
      r.duration == null
        ? ''
        : `, duration ${Math.round(r.duration)} s${r.durationStable === false ? ` -> ${Math.round(r.durationEnd)} s (UNSTABLE)` : ''}`;
    L.push(
      `- ${r.file} via ${r.method}: start ${start}${duration}; ${parts.join(', ') || r.error || 'no seeks'} -> ${seekPassed(r) ? 'pass' : 'fail'}`,
    );
  }
  L.push('Soak (30 min playback):');
  const soaks = results.soaks ?? [];
  if (soaks.length === 0) L.push('- none');
  for (const s of soaks.slice(-3)) {
    L.push(
      `- ${s.startedAt} ${s.file} via ${s.method}: ${s.outcome}, played ${formatDuration(s.playedMs)}, ` +
        `waiting events ${s.waiting ?? 0}, JS heap ${s.heapStart ?? 'n/a'} -> ${s.heapEnd ?? 'n/a'} MB (max ${s.heapMax ?? 'n/a'})`,
    );
  }
  L.push('Persistence (OPFS files now):');
  if (files.length === 0) L.push('- no files');
  for (const f of files) {
    const age = ageDays(f.downloadedAt, now.getTime());
    const check = latestBy(results.playChecks ?? [], (c) => c.file).find((c) => c.file === f.name);
    L.push(
      `- ${f.name}: ${formatBytes(f.size)} (${f.size} B), downloaded ${f.downloadedAt ?? 'unknown'}` +
        `${age == null ? '' : `, age ${age.toFixed(1)} days`}` +
        `${check ? `, last play check ${check.ok ? 'ok' : 'FAIL'} at ${check.t}` : ''}`,
    );
  }
  L.push(`Interruptions logged: ${interruptions}`);
  return L.join('\n');
}
