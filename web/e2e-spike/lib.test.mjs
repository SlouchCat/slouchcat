// Unit tests of the offline spike helpers (web/public/spike/lib.js). Run with
// `node --test web/e2e-spike/lib.test.mjs`. Temporary: removed together with the spike after the
// M1b decision.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  BLOCK_SIZE,
  blockCount,
  blockRange,
  buildSummary,
  formatBytes,
  formatDuration,
  formatLogLine,
  formatRate,
  logEntry,
  parseContentRange,
  parseRangeRequest,
  rangeHeader,
  resumeBlock,
  seekPassed,
} from '../public/spike/lib.js';

// Size of testdata/media/spike/spike-fmp4.mp4: 192 full blocks of 8 MiB plus a short last block.
const FMP4 = 1613262329;

test('blocks cover the file exactly once', () => {
  assert.equal(BLOCK_SIZE, 8388608);
  assert.equal(blockCount(FMP4), 193);
  assert.equal(blockCount(BLOCK_SIZE * 2), 2);
  let sum = 0;
  let expectedStart = 0;
  for (let i = 0; i < blockCount(FMP4); i++) {
    const r = blockRange(i, FMP4);
    assert.equal(r.start, expectedStart);
    sum += r.length;
    expectedStart = r.end + 1;
  }
  assert.equal(sum, FMP4);
  const last = blockRange(192, FMP4);
  assert.deepEqual(last, {
    start: 192 * BLOCK_SIZE,
    end: FMP4 - 1,
    length: FMP4 - 192 * BLOCK_SIZE,
  });
  assert.throws(() => blockRange(193, FMP4), RangeError);
});

test('range header and Content-Range parsing', () => {
  assert.equal(rangeHeader(0, 8388607), 'bytes=0-8388607');
  assert.deepEqual(parseContentRange('bytes 0-99/1234'), { start: 0, end: 99, total: 1234 });
  assert.deepEqual(parseContentRange('bytes 5-9/*'), { start: 5, end: 9, total: null });
  assert.equal(parseContentRange(null), null);
  assert.equal(parseContentRange('bytes */1234'), null);
});

test('service worker range requests', () => {
  assert.deepEqual(parseRangeRequest('bytes=0-', 1000), { start: 0, end: 999 });
  assert.deepEqual(parseRangeRequest('bytes=100-199', 1000), { start: 100, end: 199 });
  assert.deepEqual(parseRangeRequest('bytes=900-5000', 1000), { start: 900, end: 999 });
  assert.deepEqual(parseRangeRequest('bytes=-100', 1000), { start: 900, end: 999 });
  assert.deepEqual(parseRangeRequest('bytes=-5000', 1000), { start: 0, end: 999 });
  assert.equal(parseRangeRequest('bytes=1000-', 1000), null);
  assert.equal(parseRangeRequest('bytes=5-4', 1000), null);
  assert.equal(parseRangeRequest('bytes=-0', 1000), null);
  assert.equal(parseRangeRequest('bytes=-', 1000), null);
  assert.equal(parseRangeRequest('bytes=0-1,5-9', 1000), null);
  assert.equal(parseRangeRequest('items=0-1', 1000), null);
  assert.equal(parseRangeRequest('bytes=0-', 0), null);
});

test('resume never passes what the file holds', () => {
  assert.equal(resumeBlock(10, 10 * BLOCK_SIZE, FMP4), 10);
  assert.equal(resumeBlock(10, 4 * BLOCK_SIZE + 5, FMP4), 4); // lost commit
  assert.equal(resumeBlock(3, 10 * BLOCK_SIZE, FMP4), 3); // progress lags the file: trust progress
  assert.equal(resumeBlock(0, 0, FMP4), 0);
  assert.equal(resumeBlock(193, FMP4, FMP4), 193);
  assert.equal(resumeBlock(500, FMP4, FMP4), 193);
});

test('formatting', () => {
  assert.equal(formatBytes(1613262329), '1.61 GB');
  assert.equal(formatBytes(2500000), '2.5 MB');
  assert.equal(formatBytes(null), 'n/a');
  assert.equal(formatDuration(1500), '1.5s');
  assert.equal(formatDuration(125000), '2m 05s');
  assert.equal(formatDuration(3723000), '1h 02m 03s');
  assert.equal(formatRate(10e6, 1000), '10.0 MB/s (80 Mbit/s)');
  assert.equal(formatRate(1, 0), 'n/a');
});

test('log entries carry an ISO timestamp', () => {
  const e = logEntry('measure', 'seek', { ms: 12 }, new Date('2026-10-18T10:00:00Z'));
  assert.deepEqual(e, {
    t: '2026-10-18T10:00:00.000Z',
    type: 'measure',
    msg: 'seek',
    data: { ms: 12 },
  });
  assert.equal(formatLogLine(e), '2026-10-18T10:00:00.000Z [measure] seek {"ms":12}');
  assert.equal('data' in logEntry('info', 'x'), false);
});

test('seek threshold is < 2 s at all three positions', () => {
  const seek = (pct, playingMs, ok = true) => ({ pct, playingMs, ok });
  assert.equal(seekPassed({ seeks: [seek(0.25, 300), seek(0.5, 1999), seek(0.75, 10)] }), true);
  assert.equal(seekPassed({ seeks: [seek(0.25, 300), seek(0.5, 2000), seek(0.75, 10)] }), false);
  assert.equal(seekPassed({ seeks: [seek(0.25, 300), seek(0.5, 300, false)] }), false);
  const fast = [seek(0.25, 30), seek(0.5, 30), seek(0.75, 30)];
  assert.equal(seekPassed({ durationStable: true, seeks: fast }), true);
  assert.equal(seekPassed({ durationStable: false, seeks: fast }), false); // growing fMP4 duration
});

test('device summary lists downloads, seeks, soak and files', () => {
  const now = new Date('2026-10-21T10:00:00Z');
  const text = buildSummary({
    now,
    env: {
      ua: 'TestUA',
      secure: true,
      standalone: true,
      persistResult: 'true',
      persisted: true,
      quota: 20e9,
      usage: 1.6e9,
      syncProbe: 'yes (probe write ok)',
      writableProbe: 'yes (probe write ok)',
      swControlled: true,
    },
    downloads: [
      {
        variant: 'fmp4',
        path: 'sync',
        total: FMP4,
        blockSize: BLOCK_SIZE,
        done: 193,
        completedAt: '2026-10-18T10:00:00Z',
        verified: true,
        attempts: 2,
        resumes: 1,
        retries: 0,
        activeMs: 100000,
        bytesFetched: FMP4,
      },
    ],
    results: {
      seeks: [
        {
          file: 'spike-fmp4-sync.mp4',
          method: 'sw',
          startMs: 250,
          duration: 3120,
          durationEnd: 3120,
          durationStable: true,
          seeks: [0.25, 0.5, 0.75].map((pct) => ({ pct, playingMs: 400, ok: true })),
        },
      ],
      soaks: [],
      playChecks: [{ t: '2026-10-21T09:00:00Z', file: 'spike-fmp4-sync.mp4', ok: true }],
    },
    files: [{ name: 'spike-fmp4-sync.mp4', size: FMP4, downloadedAt: '2026-10-18T10:00:00Z' }],
    interruptions: 2,
  });
  assert.match(text, /installed PWA \(standalone\): yes/);
  assert.match(text, /Quota: 20\.00 GB/);
  assert.match(text, /fmp4\/sync: complete, size ok, 1\.61 GB, attempts 2, resumes after reload 1/);
  assert.match(
    text,
    /spike-fmp4-sync\.mp4 via sw: start 250 ms, duration 3120 s; 25 %: 400 ms, 50 %: 400 ms, 75 %: 400 ms -> pass/,
  );
  assert.match(text, /age 3\.0 days, last play check ok/);
  assert.match(text, /Interruptions logged: 2/);
});
