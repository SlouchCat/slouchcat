import type {
  HlsConfig,
  Loader,
  LoaderCallbacks,
  LoaderConfiguration,
  LoaderContext,
  LoaderStats,
} from 'hls.js/light';

// hls.js loader of offline playback: it never touches the network. The playlist request gets the
// playlist built from the manifest (playlist.ts); the init segment and every media segment are
// File.slice(offset, offset + length) of media.mp4 in OPFS. hls.js gives `rangeEnd` exclusive,
// as its own loaders send `bytes=start-(end-1)`.

export interface OfflineSource {
  /** Playlist text (buildPlaylist). */
  playlist: string;
  /** media.mp4 of the download (OPFS File). */
  media: Blob;
}

function emptyStats(): LoaderStats {
  return {
    aborted: false,
    loaded: 0,
    retry: 0,
    total: 0,
    chunkCount: 0,
    bwEstimate: 0,
    loading: { start: 0, first: 0, end: 0 },
    parsing: { start: 0, end: 0 },
    buffering: { start: 0, first: 0, end: 0 },
  };
}

/** The bytes a loader context asks for: [rangeStart, rangeEnd) or the whole file. */
export function readRange(
  media: Blob,
  context: Pick<LoaderContext, 'rangeStart' | 'rangeEnd'>,
): Promise<ArrayBuffer> {
  const start = context.rangeStart ?? 0;
  const end = context.rangeEnd ? Math.min(context.rangeEnd, media.size) : media.size;
  if (start < 0 || start > end) return Promise.reject(new RangeError(`range ${start}-${end}`));
  return media.slice(start, end).arrayBuffer();
}

type LoaderClass = new (config: HlsConfig) => Loader<LoaderContext>;

/** A loader class for `new Hls({ loader })` bound to one download. */
export function offlineLoader(source: OfflineSource): LoaderClass {
  return class OfflineLoader implements Loader<LoaderContext> {
    context: LoaderContext | null = null;
    stats: LoaderStats = emptyStats();
    private done = false;
    private callbacks: LoaderCallbacks<LoaderContext> | null = null;

    load(
      context: LoaderContext,
      _config: LoaderConfiguration,
      callbacks: LoaderCallbacks<LoaderContext>,
    ): void {
      this.context = context;
      this.callbacks = callbacks;
      this.done = false;
      const stats = emptyStats();
      this.stats = stats;
      stats.loading.start = performance.now();
      const read: Promise<string | ArrayBuffer> =
        context.responseType === 'arraybuffer'
          ? readRange(source.media, context)
          : Promise.resolve(source.playlist);
      read.then(
        (data) => {
          if (this.done) return;
          this.done = true;
          const size = typeof data === 'string' ? data.length : data.byteLength;
          const now = performance.now();
          stats.loading.first = now;
          stats.loading.end = now;
          stats.loaded = size;
          stats.total = size;
          stats.chunkCount = 1;
          callbacks.onSuccess({ url: context.url, data }, stats, context, null);
        },
        (error: unknown) => {
          if (this.done) return;
          this.done = true;
          callbacks.onError(
            { code: 0, text: error instanceof Error ? error.message : String(error) },
            context,
            null,
            stats,
          );
        },
      );
    }

    /**
     * hls.js's fragment loader settles its promise only through onAbort (after a seek it aborts
     * the fragment in flight); without the call that promise would hang (K1).
     */
    abort(): void {
      if (this.done) return;
      this.done = true;
      this.stats.aborted = true;
      const { callbacks, context } = this;
      if (callbacks?.onAbort && context) callbacks.onAbort(this.stats, context, null);
    }

    destroy(): void {
      this.done = true;
      this.context = null;
      this.callbacks = null;
    }
  };
}
