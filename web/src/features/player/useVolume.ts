import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { clampVolume } from '@/ui/aria';
import { readPlayerPrefs, writePlayerPrefs } from './usePlayerPrefs';

export interface Volume {
  /** The level 0 … max; it stays while muted, so unmuting and the arrow keys start from it. */
  volume: number;
  muted: boolean;
  /** The part above 100 % for the Web Audio gain: 1 up to 100 %, else the level (≤ 1.5). */
  gain: number;
  /** Sets the level (the slider), unmutes and stores it for this device. Returns the new level. */
  setVolume: (volume: number) => number;
  /** ↑/↓: changes the level by `delta` from the level before a mute, unmutes. Returns it. */
  step: (delta: number) => number;
  /** M and the mute button. Returns whether the element is muted now. */
  toggleMute: () => boolean;
}

interface VolumeOptions {
  /** A switch keeps a mute. */
  initialMuted?: boolean;
  /** The level of the previous session of the title (a boost above 100 % is not stored). */
  initialVolume?: number;
  /** 1, or 1.5 where the page has a Web Audio gain (BOOST_MAX). */
  max?: number;
}

// A module function: the React compiler does not let a hook write to an object it received.
function applyVolume(element: HTMLMediaElement, volume: number, muted: boolean): void {
  element.volume = volume;
  element.muted = muted;
}

/**
 * The volume of the media element (V-3): the level is remembered per device (F-2), starts at the
 * stored level (100 % without one), muted as `initialMuted` says (a switch keeps a mute), and is
 * applied to the element whenever it changes. With `max` 1.5 the level goes up to 150 %: the
 * element plays at most 100 % (`element.volume`), the rest is `gain` for the Web Audio graph
 * (useAudioGraph). The device remembers at most 100 %: a boost needs a viewer's action each time
 * the player starts.
 */
export function useVolume(
  media: RefObject<HTMLMediaElement | null>,
  { initialMuted = false, initialVolume, max = 1 }: VolumeOptions = {},
): Volume {
  const [volume, setLevel] = useState(() =>
    clampVolume(initialVolume ?? readPlayerPrefs().volume ?? 1, max),
  );
  const [muted, setMuted] = useState(initialMuted);
  // The latest values for key handlers that run before the next render.
  const level = useRef(volume);
  const mutedNow = useRef(muted);

  useEffect(() => {
    if (media.current) applyVolume(media.current, Math.min(1, volume), muted);
  }, [media, muted, volume]);

  const setVolume = useCallback(
    (next: number) => {
      const value = clampVolume(next, max);
      level.current = value;
      mutedNow.current = false;
      setLevel(value);
      setMuted(false);
      writePlayerPrefs({ volume: Math.min(1, value) });
      return value;
    },
    [max],
  );

  const step = useCallback((delta: number) => setVolume(level.current + delta), [setVolume]);

  const toggleMute = useCallback(() => {
    mutedNow.current = !mutedNow.current;
    setMuted(mutedNow.current);
    return mutedNow.current;
  }, []);

  return { volume, muted, gain: Math.max(1, volume), setVolume, step, toggleMute };
}
