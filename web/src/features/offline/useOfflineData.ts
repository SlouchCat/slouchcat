import { useEffect, useState } from 'react';
import { listDownloads, listProfiles, subscribe } from './store';
import type { StoredDownload, StoredProfile } from './types';

/**
 * A value read from the offline database, read again after every change (this tab or another).
 * Undefined while the first read runs; a failed read (storage blocked) gives the fallback.
 */
function useStored<T>(load: () => Promise<T>, fallback: T): T | undefined {
  const [value, setValue] = useState<T | undefined>(undefined);
  useEffect(() => {
    let active = true;
    let version = 0;
    const read = () => {
      const current = ++version;
      load().then(
        (next) => {
          if (active && current === version) setValue(next);
        },
        () => {
          if (active && current === version) setValue(fallback);
        },
      );
    };
    read();
    const unsubscribe = subscribe(read);
    return () => {
      active = false;
      unsubscribe();
    };
  }, [load, fallback]);
  return value;
}

const NO_DOWNLOADS: StoredDownload[] = [];
const NO_PROFILES: StoredProfile[] = [];

/** Every download the device holds (all accounts and profiles; callers filter). */
export function useStoredDownloads(): StoredDownload[] | undefined {
  return useStored(listDownloads, NO_DOWNLOADS);
}

/** Every profile snapshot the device holds. */
export function useStoredProfiles(): StoredProfile[] | undefined {
  return useStored(listProfiles, NO_PROFILES);
}
