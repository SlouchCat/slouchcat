import { BLOCK_SIZE, blockCount, blockRange, parseContentRange, rangeHeader } from './range';

/** Where the blocks go (opfs.ts): the OPFS file behind a sync access handle or a writable. */
export interface BlockWriter {
  /**
   * Writes `data` at `offset`. Resolves with the bytes now durable from offset 0, or null when
   * this write is not durable yet (the writable path commits every few blocks).
   */
  write(offset: number, data: ArrayBuffer): Promise<number | null>;
  /** Flushes and closes; resolves with the durable bytes, or null when nothing changed. */
  close(): Promise<number | null>;
}

/** Why a fetch stopped for good; the engine decides what comes next. */
export type FetchErrorCode =
  /** 410: the server removed its prepared copy; the next sync prepares it again. */
  | 'evicted'
  /** 404: the download is gone or its title is no longer visible to the token's profile. */
  | 'gone'
  /** 401: the download token is not valid (any more). */
  | 'unauthorized'
  /** The server's file has another size than the manifest: it was prepared again. */
  | 'size'
  /** The ETag (If-Range) no longer matches: the server prepared the file again. */
  | 'changed'
  /** Every attempt got an answer without the requested range (a proxy dropping Range?). */
  | 'range'
  /** The device could not keep the bytes (the writer confirmed less than the whole file). */
  | 'storage'
  /** The network failed on every attempt. */
  | 'network';

export class FetchError extends Error {
  readonly code: FetchErrorCode;
  readonly status: number | undefined;

  constructor(code: FetchErrorCode, message: string, status?: number) {
    super(message);
    this.name = 'FetchError';
    this.code = code;
    this.status = status;
  }
}

/** Wait before retry `attempt` (0-based): 2 s, doubling, at most 30 s (spike runDownload). */
export const backoffMs = (attempt: number) => Math.min(30_000, 2000 * 2 ** attempt);

/** Attempts per block before the fetch gives up until the next trigger (online, focus, sync). */
export const MAX_ATTEMPTS = 6;
/** A block that takes longer counts as a network failure. */
export const BLOCK_TIMEOUT_MS = 120_000;

export function sleep(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) {
      reject(signal.reason as Error);
      return;
    }
    const timer = setTimeout(resolve, ms);
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(timer);
        reject(signal.reason as Error);
      },
      { once: true },
    );
  });
}

/** An answer that is not the requested range; retried, then reported as `range`. */
class RangeProblem extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'RangeProblem';
  }
}

/** A strong ETag (If-Range needs one); weak or missing gives undefined. */
export function strongEtag(value: string | null): string | undefined {
  const etag = value?.trim();
  return etag && !etag.startsWith('W/') ? etag : undefined;
}

const fatalStatus: Partial<Record<number, FetchErrorCode>> = {
  401: 'unauthorized',
  403: 'unauthorized',
  404: 'gone',
  410: 'evicted',
  416: 'size',
};

export interface FetchFileOptions {
  /** Same-origin URL of the file with the download token. */
  url: string;
  /** File size from the manifest. */
  total: number;
  /** Block-aligned offset to start at (resumeOffset). */
  start: number;
  writer: BlockWriter;
  signal: AbortSignal;
  /**
   * Strong ETag of the copy the bytes before `start` came from. Every block asks with If-Range,
   * so a copy the server prepared again meanwhile answers 200 and the fetch stops with `changed`
   * instead of mixing bytes of two preparations.
   */
  etag?: string;
  /** Called once with the ETag of the first answer that carried one (store it for a resume). */
  onEtag?: (etag: string) => Promise<void> | void;
  /** Called whenever more bytes are durable, to record the progress. */
  onDurable?: (bytes: number) => Promise<void> | void;
  /** Called after every fetched block with the bytes fetched so far (for the progress bar). */
  onProgress?: (bytes: number) => void;
  /** Called before a retry with the attempt number. */
  onRetry?: (attempt: number, error: unknown) => void;
  fetchImpl?: typeof fetch;
  wait?: (ms: number, signal: AbortSignal) => Promise<void>;
  blockSize?: number;
  maxAttempts?: number;
  blockTimeoutMs?: number;
}

/** One ranged GET: requires 206, the requested Content-Range and a body of exactly that length. */
async function fetchBlock(
  options: FetchFileOptions,
  range: { start: number; end: number; length: number },
  etag: string | undefined,
): Promise<{ data: ArrayBuffer; etag: string | undefined }> {
  const { signal, fetchImpl = fetch, blockTimeoutMs = BLOCK_TIMEOUT_MS } = options;
  signal.throwIfAborted();
  const controller = new AbortController();
  const onAbort = () => controller.abort(signal.reason);
  signal.addEventListener('abort', onAbort, { once: true });
  const timer = setTimeout(
    () => controller.abort(new Error(`block timeout after ${blockTimeoutMs} ms`)),
    blockTimeoutMs,
  );
  try {
    const headers: Record<string, string> = { Range: rangeHeader(range.start, range.end) };
    if (etag) headers['If-Range'] = etag;
    const response = await fetchImpl(options.url, {
      headers,
      cache: 'no-store',
      credentials: 'same-origin',
      signal: controller.signal,
    });
    const fatal = fatalStatus[response.status];
    if (fatal) {
      void response.body?.cancel();
      throw new FetchError(fatal, `HTTP ${response.status}`, response.status);
    }
    if (response.status !== 206) {
      // A 200 would be the whole file: never read it into memory.
      void response.body?.cancel();
      if (etag && response.status === 200) {
        throw new FetchError('changed', `If-Range ${etag} no longer matches`, 200);
      }
      throw new RangeProblem(`HTTP ${response.status} instead of 206`);
    }
    const answered = strongEtag(response.headers.get('ETag'));
    if (etag && answered && answered !== etag) {
      void response.body?.cancel();
      throw new FetchError('changed', `ETag ${answered} instead of ${etag}`, 206);
    }
    const contentRange = parseContentRange(response.headers.get('Content-Range'));
    if (contentRange?.total != null && contentRange.total !== options.total) {
      void response.body?.cancel();
      throw new FetchError(
        'size',
        `file has ${contentRange.total} bytes, manifest ${options.total}`,
      );
    }
    if (contentRange && (contentRange.start !== range.start || contentRange.end !== range.end)) {
      void response.body?.cancel();
      throw new RangeProblem(
        `Content-Range ${contentRange.start}-${contentRange.end} != ${range.start}-${range.end}`,
      );
    }
    const data = await response.arrayBuffer();
    if (data.byteLength !== range.length) {
      throw new Error(`body ${data.byteLength} B != ${range.length} B`);
    }
    return { data, etag: etag ?? answered };
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', onAbort);
  }
}

/**
 * Fetches the file from `start` to its end block by block into `writer` (spike runDownload):
 * every block is retried with backoff (network errors, 5xx, a wrong range); 401, 404, 410, a
 * changed size and a changed ETag stop at once with a FetchError. After the last attempt a block
 * fails with `range` (answers without the range) or `network`. Resolves with the durable bytes.
 */
export async function fetchFile(options: FetchFileOptions): Promise<number> {
  const {
    total,
    start,
    writer,
    signal,
    blockSize = BLOCK_SIZE,
    maxAttempts = MAX_ATTEMPTS,
    wait = sleep,
  } = options;
  if (start % blockSize !== 0 && start !== total) {
    throw new RangeError(`start ${start} is not on a block boundary`);
  }
  let durable = start;
  const record = async (bytes: number | null) => {
    if (bytes === null || bytes <= durable) return;
    durable = bytes;
    await options.onDurable?.(bytes);
  };
  let closed = false;
  let etag = options.etag;
  try {
    const blocks = blockCount(total, blockSize);
    for (let index = start / blockSize; index < blocks; index += 1) {
      signal.throwIfAborted();
      const range = blockRange(index, total, blockSize);
      let data: ArrayBuffer | undefined;
      for (let attempt = 0; data === undefined; attempt += 1) {
        try {
          const answer = await fetchBlock(options, range, etag);
          data = answer.data;
          if (!etag && answer.etag) {
            etag = answer.etag;
            await options.onEtag?.(answer.etag);
          }
        } catch (error) {
          if (signal.aborted || error instanceof FetchError) throw error;
          if (attempt + 1 >= maxAttempts) {
            throw new FetchError(
              error instanceof RangeProblem ? 'range' : 'network',
              error instanceof Error ? error.message : 'failed',
            );
          }
          options.onRetry?.(attempt + 1, error);
          await wait(backoffMs(attempt), signal);
        }
      }
      await record(await writer.write(range.start, data));
      options.onProgress?.(range.end + 1);
    }
    closed = true;
    await record(await writer.close());
    return durable;
  } finally {
    if (!closed) {
      // Keep what is written: the next attempt resumes there.
      await record(await writer.close().catch(() => null));
    }
  }
}
