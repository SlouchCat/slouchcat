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
): Promise<ArrayBuffer> {
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
    const response = await fetchImpl(options.url, {
      headers: { Range: rangeHeader(range.start, range.end) },
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
      throw new Error(`HTTP ${response.status} instead of 206`);
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
      throw new Error(
        `Content-Range ${contentRange.start}-${contentRange.end} != ${range.start}-${range.end}`,
      );
    }
    const data = await response.arrayBuffer();
    if (data.byteLength !== range.length) {
      throw new Error(`body ${data.byteLength} B != ${range.length} B`);
    }
    return data;
  } finally {
    clearTimeout(timer);
    signal.removeEventListener('abort', onAbort);
  }
}

/**
 * Fetches the file from `start` to its end block by block into `writer` (spike runDownload):
 * every block is retried with backoff (network errors, 5xx, a wrong range); 401, 404, 410 and a
 * changed size stop at once with a FetchError. Resolves with the durable bytes at the end.
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
  try {
    const blocks = blockCount(total, blockSize);
    for (let index = start / blockSize; index < blocks; index += 1) {
      signal.throwIfAborted();
      const range = blockRange(index, total, blockSize);
      let data: ArrayBuffer | undefined;
      for (let attempt = 0; data === undefined; attempt += 1) {
        try {
          data = await fetchBlock(options, range);
        } catch (error) {
          if (signal.aborted || error instanceof FetchError) throw error;
          if (attempt + 1 >= maxAttempts) {
            throw new FetchError('network', error instanceof Error ? error.message : 'failed');
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
