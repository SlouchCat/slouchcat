import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { clampVolume } from '@/ui/aria';
import { readPlayerPrefs, writePlayerPrefs } from './usePlayerPrefs';

export interface Volume {
  /** The level 0 … 1; it stays while muted, so unmuting and the arrow keys start from it. */
  volume: number;
  muted: boolean;
  /** Sets the level (the slider), unmutes and stores it for this device. Returns the new level. */
  setVolume: (volume: number) => number;
  /** ↑/↓: changes the level by `delta` from the level before a mute, unmutes. Returns it. */
  step: (delta: number) => number;
  /** M and the mute button. Returns whether the element is muted now. */
  toggleMute: () => boolean;
}

// A module function: the React compiler does not let a hook write to an object it received.
function applyVolume(element: HTMLMediaElement, volume: number, muted: boolean): void {
  element.volume = volume;
  element.muted = muted;
}

/**
 * The volume of the media element (V-3): the level is remembered per device (F-2), starts at the
 * stored level (100 % without one), muted as `initialMuted` says (a switch keeps a mute), and is applied to the element whenever it changes. Up to 100 %
 * only; the boost above it comes with the Web Audio gain (concept 13, step 4).
 */
export function useVolume(media: RefObject<HTMLMediaElement | null>, initialMuted = false): Volume {
  const [volume, setLevel] = useState(() => readPlayerPrefs().volume ?? 1);
  const [muted, setMuted] = useState(initialMuted);
  // The latest values for key handlers that run before the next render.
  const level = useRef(volume);
  const mutedNow = useRef(muted);

  useEffect(() => {
    if (media.current) applyVolume(media.current, volume, muted);
  }, [media, muted, volume]);

  const setVolume = useCallback((next: number) => {
    const value = clampVolume(next, 1);
    level.current = value;
    mutedNow.current = false;
    setLevel(value);
    setMuted(false);
    writePlayerPrefs({ volume: value });
    return value;
  }, []);

  const step = useCallback((delta: number) => setVolume(level.current + delta), [setVolume]);

  const toggleMute = useCallback(() => {
    mutedNow.current = !mutedNow.current;
    setMuted(mutedNow.current);
    return mutedNow.current;
  }, []);

  return { volume, muted, setVolume, step, toggleMute };
}
