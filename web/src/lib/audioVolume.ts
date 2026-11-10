import { useSyncExternalStore } from 'react';

// One volume per device for everything that is only heard (concept 19 E3, OE-1): the radio, the
// audio player and the offline player in audio mode share it; the video player keeps its own
// (`slouchcat.player`, V-3). A mute is not stored, the first value is 100 %, and without storage
// (a private window, blocked site data) the value lasts for this visit.

/** localStorage key of the shared volume, a number 0 … 1. */
export const AUDIO_VOLUME_KEY = 'slouchcat.audio.volume';

const listeners = new Set<() => void>();
/** The value of this visit while the storage cannot keep it; null while it can. */
let unstored: number | null = null;

const clamp = (value: number) => Math.round(Math.min(1, Math.max(0, value)) * 100) / 100;

/** The stored volume 0 … 1; 1 without one, for junk or without storage. */
export function readAudioVolume(): number {
  if (unstored !== null) return unstored;
  try {
    const raw = localStorage.getItem(AUDIO_VOLUME_KEY);
    const value = raw === null || raw.trim() === '' ? Number.NaN : Number(raw);
    return Number.isFinite(value) ? clamp(value) : 1;
  } catch {
    return 1;
  }
}

/** Stores `volume` (clamped to 0 … 1) for this device and tells every user. Returns it. */
export function writeAudioVolume(volume: number): number {
  const value = clamp(Number.isFinite(volume) ? volume : 0);
  try {
    localStorage.setItem(AUDIO_VOLUME_KEY, String(value));
    unstored = null;
  } catch {
    unstored = value;
  }
  for (const listener of listeners) listener();
  return value;
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  // Another tab of the app changed it.
  const onStorage = (event: StorageEvent) => {
    if (event.key === AUDIO_VOLUME_KEY || event.key === null) listener();
  };
  window.addEventListener('storage', onStorage);
  return () => {
    listeners.delete(listener);
    window.removeEventListener('storage', onStorage);
  };
}

/** The shared volume 0 … 1; changes in this tab and in others re-render. */
export function useAudioVolume(): number {
  return useSyncExternalStore(subscribe, readAudioVolume, readAudioVolume);
}
