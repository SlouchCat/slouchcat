import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { storedDownload } from '@/test/offlineFixtures';
import { subscribe, updateDownload } from './store';
import type { StoredDownload } from './types';

// The listeners of the offline data (pages that list downloads) read every download again after
// each change. A fetch writes its progress after every block: those writes tell them at most once
// a second (K8). jsdom has no IndexedDB; the database is a map behind the same calls.

const records = vi.hoisted(() => new Map<string, unknown>());

vi.mock('./db', () => ({
  offlineDb: () =>
    Promise.resolve({
      transaction: () => {
        const store = {
          get: (id: string) => Promise.resolve(records.get(id)),
          put: (value: { id: string }) => {
            records.set(value.id, value);
            return Promise.resolve(value.id);
          },
        };
        return { store, done: Promise.resolve() };
      },
    }),
}));

beforeEach(() => {
  vi.useFakeTimers();
  records.clear();
  records.set(storedDownload().id, storedDownload({ complete: false, bytesWritten: 0 }));
});

afterEach(() => {
  vi.useRealTimers();
});

const step = (bytes: number) => (download: StoredDownload | undefined) =>
  download && { ...download, bytesWritten: bytes };

describe('offline store notifications', () => {
  it('progress writes reach the listeners at most once a second (K8)', async () => {
    const listener = vi.fn();
    const unsubscribe = subscribe(listener);
    for (let block = 1; block <= 5; block += 1) {
      await updateDownload(storedDownload().id, step(block * 100), { progress: true });
    }
    expect(listener).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(1000);
    expect(listener).toHaveBeenCalledTimes(1);
    expect((records.get(storedDownload().id) as StoredDownload).bytesWritten).toBe(500);
    unsubscribe();
  });

  it('every other change reaches them at once', async () => {
    const listener = vi.fn();
    const unsubscribe = subscribe(listener);
    await updateDownload(storedDownload().id, step(9000));
    await updateDownload(
      storedDownload().id,
      (download) => download && { ...download, complete: true },
    );
    expect(listener).toHaveBeenCalledTimes(2);
    unsubscribe();
  });
});
