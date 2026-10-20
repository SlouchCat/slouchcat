import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { openMediaWriter } from './opfs';

// The sync write path of opfs.ts against a dedicated worker that behaves like the browser's:
// postMessage structured-clones the message and transfers the listed buffers, which are detached
// (0 bytes) on the page afterwards. jsdom has neither workers nor OPFS.

interface Command {
  id: number;
  cmd: string;
  buffer?: ArrayBuffer;
}

const worker = vi.hoisted(() => ({
  failWrite: null as { name: string; message: string } | null,
  received: [] as number[],
}));

class FakeWorker {
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: ErrorEvent) => void) | null = null;

  postMessage(message: Command, transfer: Transferable[] = []): void {
    const command = structuredClone(message, { transfer });
    setTimeout(() => this.onmessage?.({ data: this.answer(command) } as MessageEvent), 0);
  }

  private answer(command: Command): Record<string, unknown> {
    const { id } = command;
    switch (command.cmd) {
      case 'probe':
        return { id, result: { ok: true } };
      case 'open':
        return { id, result: { size: 0 } };
      case 'write':
        if (worker.failWrite) return { id, error: worker.failWrite };
        worker.received.push(command.buffer?.byteLength ?? -1);
        return { id, result: { written: command.buffer?.byteLength } };
      case 'close':
        return { id, result: { size: null } };
      default:
        return { id, error: { name: 'Error', message: `unknown ${command.cmd}` } };
    }
  }

  terminate(): void {}
}

const folder = {
  getDirectoryHandle: () => Promise.resolve(folder),
};

beforeEach(() => {
  worker.failWrite = null;
  worker.received.length = 0;
  vi.stubGlobal('Worker', FakeWorker);
  Object.defineProperty(navigator, 'storage', {
    configurable: true,
    value: { getDirectory: () => Promise.resolve(folder) },
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('sync write path (worker)', () => {
  it('counts the bytes of a block although postMessage detached its buffer', async () => {
    const writer = await openMediaWriter('01JB8Z6Q4M7N9P2R3S5T7V9D01');
    const block = new Uint8Array(1000).buffer;
    // The E2E run of M5 saw durable = offset + 0 for every block: the download never completed.
    await expect(writer.write(8000, block)).resolves.toBe(9000);
    expect(block.byteLength).toBe(0);
    expect(worker.received).toEqual([1000]);
    await expect(writer.close()).resolves.toBe(9000);
  });

  it('a full storage in the worker rejects as the QuotaExceededError it was (W7)', async () => {
    worker.failWrite = { name: 'QuotaExceededError', message: 'storage full' };
    const writer = await openMediaWriter('01JB8Z6Q4M7N9P2R3S5T7V9D01');
    const error = await writer
      .write(0, new Uint8Array(10).buffer)
      .catch((caught: unknown) => caught);
    expect(error).toBeInstanceOf(DOMException);
    expect(error).toMatchObject({ name: 'QuotaExceededError', message: 'storage full' });
  });
});
