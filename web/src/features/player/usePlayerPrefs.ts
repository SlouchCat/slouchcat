import type { PlaybackRequest, Quality } from '@slouchcat/api-client';
import { useCallback, useState } from 'react';

/** localStorage key of the player's settings on this device. */
export const PLAYER_PREFS_KEY = 'slouchcat.player';

/**
 * What the video player remembers on this device (F-2: per device, not per profile; a phone and a
 * TV browser play at different levels). Fields the stored value lacks fall back to their default.
 */
export interface PlayerPrefs {
  /** Volume 0 … 1 (V-3). Muting is not remembered: the next title starts audible. */
  volume?: number;
  /** The time above the timeline: elapsed and total, or remaining and the end time (V-4). */
  timeDisplay?: 'elapsed' | 'remaining';
  /**
   * The quality last chosen in the quality panel on this device (V-17, 11.5), never one taken
   * after the hint of a slow connection (V-18). A start sends it with `rememberedQuality`, and the
   * server keeps deciding (rule 12).
   */
  quality?: Quality;
}

const QUALITIES: readonly Quality[] = [
  'auto',
  'original',
  '2160p',
  '1080p',
  '720p',
  '480p',
  '360p',
];

/**
 * The fields of startPlayback for the quality remembered on this device (V-17, 11.5): a rung below
 * 2160p caps the output (`qualityCap`: a file within it still plays directly), original and 2160p
 * go as `quality`, auto as nothing.
 */
export function rememberedQuality(
  quality: Quality | undefined,
): Pick<PlaybackRequest, 'quality' | 'qualityCap'> {
  switch (quality) {
    case undefined:
    case 'auto':
      return {};
    case 'original':
    case '2160p':
      return { quality };
    default:
      return { qualityCap: quality };
  }
}

/** The stored settings; empty without storage (private window, blocked site data) or for junk. */
export function readPlayerPrefs(): PlayerPrefs {
  try {
    const raw = localStorage.getItem(PLAYER_PREFS_KEY);
    const value: unknown = raw === null ? null : JSON.parse(raw);
    if (value === null || typeof value !== 'object') return {};
    const { volume, timeDisplay, quality } = value as Record<string, unknown>;
    const prefs: PlayerPrefs = {};
    if (QUALITIES.includes(quality as Quality)) prefs.quality = quality as Quality;
    if (typeof volume === 'number' && Number.isFinite(volume)) {
      prefs.volume = Math.min(1, Math.max(0, volume));
    }
    if (timeDisplay === 'elapsed' || timeDisplay === 'remaining') prefs.timeDisplay = timeDisplay;
    return prefs;
  } catch {
    return {};
  }
}

/** Merges `patch` into the stored settings; without storage the change lasts for this visit. */
export function writePlayerPrefs(patch: Partial<PlayerPrefs>): void {
  try {
    localStorage.setItem(PLAYER_PREFS_KEY, JSON.stringify({ ...readPlayerPrefs(), ...patch }));
  } catch {
    // Nothing to do: the player keeps the value in its state.
  }
}

/** The player's settings on this device and a function that changes and stores some of them. */
export function usePlayerPrefs(): [PlayerPrefs, (patch: Partial<PlayerPrefs>) => void] {
  const [prefs, setPrefs] = useState(readPlayerPrefs);
  const update = useCallback((patch: Partial<PlayerPrefs>) => {
    setPrefs((current) => ({ ...current, ...patch }));
    writePlayerPrefs(patch);
  }, []);
  return [prefs, update];
}
