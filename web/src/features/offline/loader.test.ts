import type { LoaderCallbacks, LoaderConfiguration, LoaderContext } from 'hls.js/light';
import { describe, expect, it, vi } from 'vitest';
import { offlineLoader, readRange } from './loader';

const file = new Blob([Uint8Array.from({ length: 64 }, (_, index) => index)]);

function callbacks() {
  return {
    onSuccess: vi.fn<LoaderCallbacks<LoaderContext>['onSuccess']>(),
    onError: vi.fn<LoaderCallbacks<LoaderContext>['onError']>(),
    onTimeout: vi.fn<LoaderCallbacks<LoaderContext>['onTimeout']>(),
  };
}

const context = (overrides: Partial<LoaderContext>): LoaderContext =>
  ({
    url: 'https://app.test/offline/media/x/media.mp4',
    responseType: 'arraybuffer',
    ...overrides,
  }) as LoaderContext;

const config = {} as LoaderConfiguration;

describe('readRange', () => {
  it('reads [rangeStart, rangeEnd): rangeEnd is exclusive, as hls.js gives it', async () => {
    const bytes = new Uint8Array(await readRange(file, { rangeStart: 10, rangeEnd: 14 }));
    expect([...bytes]).toEqual([10, 11, 12, 13]);
  });

  it('reads the whole file without a range and stops at its end', async () => {
    expect((await readRange(file, {})).byteLength).toBe(64);
    expect((await readRange(file, { rangeStart: 60, rangeEnd: 100 })).byteLength).toBe(4);
  });

  it('rejects a range that ends before it starts', async () => {
    await expect(readRange(file, { rangeStart: 20, rangeEnd: 10 })).rejects.toThrow(RangeError);
  });
});

describe('offlineLoader', () => {
  it('serves the playlist text and the segment bytes without the network', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    const Loader = offlineLoader({ playlist: '#EXTM3U\n', media: file });
    const loader = new Loader({} as never);

    const playlist = callbacks();
    loader.load(
      context({ url: 'https://app.test/offline/media/x/index.m3u8', responseType: 'text' }),
      config,
      playlist,
    );
    await vi.waitFor(() => expect(playlist.onSuccess).toHaveBeenCalled());
    const [response, stats] = playlist.onSuccess.mock.calls[0]!;
    expect(response.data).toBe('#EXTM3U\n');
    expect(response.url).toBe('https://app.test/offline/media/x/index.m3u8');
    expect(stats.loaded).toBe(8);

    const segment = callbacks();
    loader.load(context({ rangeStart: 32, rangeEnd: 40 }), config, segment);
    await vi.waitFor(() => expect(segment.onSuccess).toHaveBeenCalled());
    const data = segment.onSuccess.mock.calls[0]![0].data as ArrayBuffer;
    expect([...new Uint8Array(data)]).toEqual([32, 33, 34, 35, 36, 37, 38, 39]);
    expect(loader.stats.total).toBe(8);
    expect(loader.stats.loading.end).toBeGreaterThanOrEqual(loader.stats.loading.start);
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it('reports nothing after abort', async () => {
    const Loader = offlineLoader({ playlist: '', media: file });
    const loader = new Loader({} as never);
    const aborted = callbacks();
    loader.load(context({ rangeStart: 0, rangeEnd: 8 }), config, aborted);
    loader.abort();
    await new Promise((resolve) => setTimeout(resolve, 10));
    expect(aborted.onSuccess).not.toHaveBeenCalled();
    expect(aborted.onError).not.toHaveBeenCalled();
    expect(loader.stats.aborted).toBe(true);
  });

  it('reports an error for a range outside the file', async () => {
    const Loader = offlineLoader({ playlist: '', media: file });
    const loader = new Loader({} as never);
    const broken = callbacks();
    loader.load(context({ rangeStart: 50, rangeEnd: 10 }), config, broken);
    await vi.waitFor(() => expect(broken.onError).toHaveBeenCalled());
    expect(broken.onSuccess).not.toHaveBeenCalled();
  });
});
