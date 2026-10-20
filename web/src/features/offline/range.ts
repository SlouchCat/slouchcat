// Byte-range arithmetic of the download fetch (spike lib.js, M1a): the file comes in 8-MiB blocks
// with `Range` requests, and a fetch that stopped (reload, screen lock, app switch) resumes at the
// last block the device wrote durably.

export const MiB = 1024 * 1024;
export const BLOCK_SIZE = 8 * MiB;

export interface ByteRange {
  start: number;
  /** Inclusive, as in HTTP Range headers. */
  end: number;
  length: number;
}

/** Number of blocks of `total` bytes. */
export function blockCount(total: number, blockSize = BLOCK_SIZE): number {
  return Math.ceil(total / blockSize);
}

/** Byte range of block `index` of a file of `total` bytes. */
export function blockRange(index: number, total: number, blockSize = BLOCK_SIZE): ByteRange {
  const start = index * blockSize;
  if (index < 0 || start >= total) throw new RangeError(`block ${index} outside ${total} bytes`);
  const end = Math.min(start + blockSize, total) - 1;
  return { start, end, length: end - start + 1 };
}

export const rangeHeader = (start: number, end: number) => `bytes=${start}-${end}`;

/** "bytes 0-99/1234" of a 206 answer; null when absent or malformed. */
export function parseContentRange(
  value: string | null,
): { start: number; end: number; total: number | null } | null {
  const match = /^bytes (\d+)-(\d+)\/(\d+|\*)$/.exec((value ?? '').trim());
  if (!match) return null;
  return {
    start: Number(match[1]),
    end: Number(match[2]),
    total: match[3] === '*' ? null : Number(match[3]),
  };
}

/**
 * Offset to resume a fetch at: the durable progress the database recorded, but never past what
 * the file really holds (the page may have died between the write and the record, or a write path
 * may have lost its last commit), rounded down to a block boundary. `total` means complete.
 */
export function resumeOffset(
  recordedBytes: number,
  fileSize: number,
  total: number,
  blockSize = BLOCK_SIZE,
): number {
  const durable = Math.max(0, Math.min(recordedBytes, fileSize));
  if (durable >= total) return total;
  return Math.floor(durable / blockSize) * blockSize;
}
