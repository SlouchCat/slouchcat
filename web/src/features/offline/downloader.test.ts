import { describe, expect, it, vi } from 'vitest';
import { backoffMs, FetchError, fetchFile, strongEtag, type BlockWriter } from './downloader';
import { blockCount, blockRange, parseContentRange, resumeOffset } from './range';

const BLOCK = 10;
const TOTAL = 35;
const FILE = Uint8Array.from({ length: TOTAL }, (_, index) => index);

/** A server that answers Range requests on FILE with 206, optionally failing first. */
function server(options: { fail?: (start: number, call: number) => Response | Error | null } = {}) {
  const calls: string[] = [];
  const respond = (init?: RequestInit): Response => {
    const range = new Headers(init?.headers).get('Range') ?? '';
    calls.push(range);
    const match = /^bytes=(\d+)-(\d+)$/.exec(range);
    if (!match) return new Response(FILE, { status: 200 });
    const start = Number(match[1]);
    const end = Number(match[2]);
    const failure = options.fail?.(start, calls.length);
    if (failure instanceof Error) throw failure;
    if (failure) return failure;
    return new Response(FILE.slice(start, end + 1), {
      status: 206,
      headers: { 'Content-Range': `bytes ${start}-${end}/${TOTAL}` },
    });
  };
  // A throw inside the executor rejects, like fetch does on a network error.
  const fetchImpl = vi.fn(
    (_url: RequestInfo | URL, init?: RequestInit) =>
      new Promise<Response>((resolve) => resolve(respond(init))),
  );
  return { calls, fetchImpl: fetchImpl as unknown as typeof fetch };
}

/** A writer like the sync path (each block durable) or the writable path (durable at close). */
function writer(mode: 'sync' | 'writable' = 'sync') {
  const bytes = new Uint8Array(TOTAL);
  let written = 0;
  const close = vi.fn(() => Promise.resolve(written));
  const target: BlockWriter = {
    write: (offset, data) => {
      bytes.set(new Uint8Array(data), offset);
      written = offset + data.byteLength;
      return Promise.resolve(mode === 'sync' ? written : null);
    },
    close,
  };
  return { target, bytes, close };
}

const noWait = () => Promise.resolve();
/** A recorded wait between retries (milliseconds as asked). */
const recordedWait = () => vi.fn<(ms: number, signal: AbortSignal) => Promise<void>>(noWait);

describe('range arithmetic', () => {
  it('splits a file into blocks with inclusive ends', () => {
    expect(blockCount(TOTAL, BLOCK)).toBe(4);
    expect(blockRange(0, TOTAL, BLOCK)).toEqual({ start: 0, end: 9, length: 10 });
    expect(blockRange(3, TOTAL, BLOCK)).toEqual({ start: 30, end: 34, length: 5 });
    expect(() => blockRange(4, TOTAL, BLOCK)).toThrow(RangeError);
  });

  it('parses Content-Range', () => {
    expect(parseContentRange('bytes 0-9/35')).toEqual({ start: 0, end: 9, total: 35 });
    expect(parseContentRange('bytes 0-9/*')).toEqual({ start: 0, end: 9, total: null });
    expect(parseContentRange(null)).toBeNull();
    expect(parseContentRange('items 0-9/35')).toBeNull();
  });

  it('resumes at the last durable block, never past what the file holds', () => {
    // Recorded 25 bytes, the file holds them: block 2 starts at 20.
    expect(resumeOffset(25, 25, TOTAL, BLOCK)).toBe(20);
    // The record says 30, but the file only kept 12 (lost commit): back to block 1.
    expect(resumeOffset(30, 12, TOTAL, BLOCK)).toBe(10);
    // The file is longer than the record (page died before the record): the record wins.
    expect(resumeOffset(10, 30, TOTAL, BLOCK)).toBe(10);
    expect(resumeOffset(0, 0, TOTAL, BLOCK)).toBe(0);
    expect(resumeOffset(TOTAL, TOTAL, TOTAL, BLOCK)).toBe(TOTAL);
  });

  it('backs off 2 s, doubling, at most 30 s', () => {
    expect([0, 1, 2, 3, 4, 5].map(backoffMs)).toEqual([2000, 4000, 8000, 16000, 30000, 30000]);
  });
});

describe('fetchFile', () => {
  it('fetches every block with Range and records each durable step', async () => {
    const { calls, fetchImpl } = server();
    const { target, bytes } = writer();
    const durable: number[] = [];
    const result = await fetchFile({
      url: '/api/v1/downloads/x/file?dt=t',
      total: TOTAL,
      start: 0,
      writer: target,
      signal: new AbortController().signal,
      onDurable: (value) => {
        durable.push(value);
      },
      fetchImpl,
      blockSize: BLOCK,
    });
    expect(result).toBe(TOTAL);
    expect(calls).toEqual(['bytes=0-9', 'bytes=10-19', 'bytes=20-29', 'bytes=30-34']);
    expect(durable).toEqual([10, 20, 30, 35]);
    expect([...bytes]).toEqual([...FILE]);
  });

  it('resumes after a reload at the block it stopped', async () => {
    const { calls, fetchImpl } = server();
    const { target } = writer();
    await fetchFile({
      url: '/f',
      total: TOTAL,
      start: resumeOffset(27, 27, TOTAL, BLOCK),
      writer: target,
      signal: new AbortController().signal,
      fetchImpl,
      blockSize: BLOCK,
    });
    expect(calls).toEqual(['bytes=20-29', 'bytes=30-34']);
  });

  it('retries network errors and 5xx with backoff, then goes on', async () => {
    const { calls, fetchImpl } = server({
      fail: (start, call) => {
        if (start === 10 && call === 2) return new TypeError('Failed to fetch');
        if (start === 10 && call === 3) return new Response('busy', { status: 503 });
        return null;
      },
    });
    const { target } = writer();
    const wait = recordedWait();
    const onRetry = vi.fn();
    await fetchFile({
      url: '/f',
      total: TOTAL,
      start: 0,
      writer: target,
      signal: new AbortController().signal,
      fetchImpl,
      wait,
      onRetry,
      blockSize: BLOCK,
    });
    expect(calls.filter((range) => range === 'bytes=10-19')).toHaveLength(3);
    expect(wait.mock.calls.map(([ms]) => ms)).toEqual([2000, 4000]);
    expect(onRetry).toHaveBeenCalledTimes(2);
  });

  it('gives up after the last attempt with a network error', async () => {
    const { fetchImpl } = server({ fail: () => new TypeError('offline') });
    const { target, close } = writer();
    await expect(
      fetchFile({
        url: '/f',
        total: TOTAL,
        start: 0,
        writer: target,
        signal: new AbortController().signal,
        fetchImpl,
        wait: noWait,
        maxAttempts: 3,
        blockSize: BLOCK,
      }),
    ).rejects.toMatchObject({ code: 'network' });
    expect(close).toHaveBeenCalled();
  });

  it('stops at once on 410 (evicted), 404 (gone) and 401', async () => {
    for (const [status, code] of [
      [410, 'evicted'],
      [404, 'gone'],
      [401, 'unauthorized'],
    ] as const) {
      const { calls, fetchImpl } = server({
        fail: () => new Response(null, { status }),
      });
      const error = await fetchFile({
        url: '/f',
        total: TOTAL,
        start: 0,
        writer: writer().target,
        signal: new AbortController().signal,
        fetchImpl,
        wait: noWait,
        blockSize: BLOCK,
      }).catch((caught: unknown) => caught);
      expect(error).toBeInstanceOf(FetchError);
      expect(error).toMatchObject({ code, status });
      expect(calls).toHaveLength(1);
    }
  });

  it('requires 206: a 200 with the whole file counts as a failed attempt', async () => {
    const { fetchImpl } = server({
      fail: (_start, call) => (call === 1 ? new Response(FILE, { status: 200 }) : null),
    });
    const wait = recordedWait();
    await fetchFile({
      url: '/f',
      total: TOTAL,
      start: 0,
      writer: writer().target,
      signal: new AbortController().signal,
      fetchImpl,
      wait,
      blockSize: BLOCK,
    });
    expect(wait).toHaveBeenCalledTimes(1);
  });

  it('stops when the server file has another size than the manifest', async () => {
    const { fetchImpl } = server({
      fail: (start) =>
        new Response(FILE.slice(start, start + BLOCK), {
          status: 206,
          headers: { 'Content-Range': `bytes ${start}-${start + BLOCK - 1}/99` },
        }),
    });
    await expect(
      fetchFile({
        url: '/f',
        total: TOTAL,
        start: 0,
        writer: writer().target,
        signal: new AbortController().signal,
        fetchImpl,
        wait: noWait,
        blockSize: BLOCK,
      }),
    ).rejects.toMatchObject({ code: 'size' });
  });

  it('with the writable path progress is durable only at the commit', async () => {
    const { fetchImpl } = server();
    const { target } = writer('writable');
    const durable: number[] = [];
    await fetchFile({
      url: '/f',
      total: TOTAL,
      start: 0,
      writer: target,
      signal: new AbortController().signal,
      onDurable: (value) => {
        durable.push(value);
      },
      fetchImpl,
      blockSize: BLOCK,
    });
    expect(durable).toEqual([TOTAL]);
  });

  it('stops on abort and keeps what was written', async () => {
    const controller = new AbortController();
    const { fetchImpl } = server({
      fail: (start) => {
        if (start === 20) controller.abort(new DOMException('stop', 'AbortError'));
        return null;
      },
    });
    const { target, close } = writer();
    const durable: number[] = [];
    await expect(
      fetchFile({
        url: '/f',
        total: TOTAL,
        start: 0,
        writer: target,
        signal: controller.signal,
        onDurable: (value) => {
          durable.push(value);
        },
        fetchImpl,
        blockSize: BLOCK,
      }),
    ).rejects.toThrow();
    expect(durable.at(-1)).toBeGreaterThanOrEqual(20);
    expect(close).toHaveBeenCalled();
  });

  it('refuses a start that is not on a block boundary', async () => {
    await expect(
      fetchFile({
        url: '/f',
        total: TOTAL,
        start: 5,
        writer: writer().target,
        signal: new AbortController().signal,
        fetchImpl: server().fetchImpl,
        blockSize: BLOCK,
      }),
    ).rejects.toThrow(RangeError);
  });
});

describe('fetchFile against a copy prepared again (O1) and a proxy without Range (K4)', () => {
  /** Answers like getDownloadFile with an ETag; If-Range with another one gets the whole file. */
  function etagServer(current: string) {
    const seen: { range: string; ifRange: string | null }[] = [];
    const fetchImpl = vi.fn((_url: RequestInfo | URL, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      const range = headers.get('Range') ?? '';
      const ifRange = headers.get('If-Range');
      seen.push({ range, ifRange });
      if (ifRange && ifRange !== current) {
        return Promise.resolve(new Response(FILE, { status: 200, headers: { ETag: current } }));
      }
      const match = /^bytes=(\d+)-(\d+)$/.exec(range);
      const start = Number(match?.[1]);
      const end = Number(match?.[2]);
      return Promise.resolve(
        new Response(FILE.slice(start, end + 1), {
          status: 206,
          headers: { 'Content-Range': `bytes ${start}-${end}/${TOTAL}`, ETag: current },
        }),
      );
    });
    return { seen, fetchImpl: fetchImpl as unknown as typeof fetch };
  }

  it('keeps the first ETag and asks every later block with If-Range', async () => {
    const { seen, fetchImpl } = etagServer('"v1"');
    const onEtag = vi.fn();
    await fetchFile({
      url: '/f',
      total: TOTAL,
      start: 0,
      writer: writer().target,
      signal: new AbortController().signal,
      fetchImpl,
      onEtag,
      blockSize: BLOCK,
    });
    expect(onEtag).toHaveBeenCalledTimes(1);
    expect(onEtag).toHaveBeenCalledWith('"v1"');
    expect(seen.map((entry) => entry.ifRange)).toEqual([null, '"v1"', '"v1"', '"v1"']);
  });

  it('a resume against another preparation stops with `changed` instead of mixing bytes', async () => {
    const { seen, fetchImpl } = etagServer('"v2"');
    const error = await fetchFile({
      url: '/f',
      total: TOTAL,
      start: 20,
      etag: '"v1"',
      writer: writer().target,
      signal: new AbortController().signal,
      fetchImpl,
      wait: noWait,
      blockSize: BLOCK,
    }).catch((caught: unknown) => caught);
    expect(error).toMatchObject({ code: 'changed' });
    // No retry: the bytes of the old copy are worthless.
    expect(seen).toEqual([{ range: 'bytes=20-29', ifRange: '"v1"' }]);
  });

  it('answers without the range on every attempt end as `range`, not as a network error', async () => {
    const { fetchImpl } = server({ fail: () => new Response(FILE, { status: 200 }) });
    await expect(
      fetchFile({
        url: '/f',
        total: TOTAL,
        start: 0,
        writer: writer().target,
        signal: new AbortController().signal,
        fetchImpl,
        wait: noWait,
        maxAttempts: 2,
        blockSize: BLOCK,
      }),
    ).rejects.toMatchObject({ code: 'range' });
  });

  it('weak ETags are not used for If-Range', () => {
    expect(strongEtag('W/"v1"')).toBeUndefined();
    expect(strongEtag(null)).toBeUndefined();
    expect(strongEtag(' "v1" ')).toBe('"v1"');
  });
});
