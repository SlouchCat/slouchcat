import { useEffect, useState } from 'react';
import { POSTER_FILE, readFile } from './opfs';
import type { StoredDownload } from './types';

/**
 * Object URL of the poster stored with a download (OPFS), so the offline pages show it without
 * the server or the service worker's image cache; `fallback` until it is there (online pages).
 */
export function usePosterUrl(
  download: StoredDownload | undefined,
  fallback?: string,
): string | undefined {
  const [url, setUrl] = useState<{ id: string; url: string } | null>(null);
  const id = download?.id;
  const stored = download?.posterStored === true;
  useEffect(() => {
    if (!id || !stored) return;
    let active = true;
    let objectUrl: string | undefined;
    readFile(id, POSTER_FILE).then(
      (file) => {
        if (!active || !file || file.size === 0) return;
        objectUrl = URL.createObjectURL(file);
        setUrl({ id, url: objectUrl });
      },
      () => undefined,
    );
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [id, stored]);
  return url && url.id === id ? url.url : fallback;
}
