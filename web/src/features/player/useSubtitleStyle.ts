import { useCallback, useState } from 'react';

/** localStorage key of the subtitle style on this device. */
export const SUBTITLE_STYLE_KEY = 'slouchcat.subtitles';

export const SUBTITLE_SIZES = ['s', 'm', 'l', 'xl'] as const;
export const SUBTITLE_BACKGROUNDS = ['none', 'shadow', 'translucent', 'box'] as const;
export const SUBTITLE_COLORS = ['white', 'yellow'] as const;
export const SUBTITLE_POSITIONS = ['bottom', 'raised'] as const;

/** One step of the offset (V-6: ±0.1 s). */
export const SUBTITLE_OFFSET_STEP_MS = 100;
/** The offset stays within ±30 s: further off it is the wrong file, not a late subtitle. */
export const SUBTITLE_OFFSET_MAX_MS = 30_000;

/**
 * How the player draws text subtitles (V-6), per device like the volume (F-2): size, background,
 * colour, position and an offset. A positive offset shows every cue later.
 */
export interface SubtitleStyle {
  size: (typeof SUBTITLE_SIZES)[number];
  background: (typeof SUBTITLE_BACKGROUNDS)[number];
  color: (typeof SUBTITLE_COLORS)[number];
  position: (typeof SUBTITLE_POSITIONS)[number];
  offsetMs: number;
}

export const DEFAULT_SUBTITLE_STYLE: SubtitleStyle = {
  size: 'm',
  background: 'shadow',
  color: 'white',
  position: 'bottom',
  offsetMs: 0,
};

const oneOf = <T extends string>(values: readonly T[], value: unknown, fallback: T): T =>
  values.includes(value as T) ? (value as T) : fallback;

/** An offset on the grid of 0.1 s within ±30 s. */
export function clampOffset(ms: number): number {
  if (!Number.isFinite(ms)) return 0;
  const stepped = Math.round(ms / SUBTITLE_OFFSET_STEP_MS) * SUBTITLE_OFFSET_STEP_MS;
  return Math.min(SUBTITLE_OFFSET_MAX_MS, Math.max(-SUBTITLE_OFFSET_MAX_MS, stepped));
}

/** The stored style; the defaults without storage or for junk, field by field. */
export function readSubtitleStyle(): SubtitleStyle {
  const fallback = DEFAULT_SUBTITLE_STYLE;
  try {
    const raw = localStorage.getItem(SUBTITLE_STYLE_KEY);
    const value: unknown = raw === null ? null : JSON.parse(raw);
    if (value === null || typeof value !== 'object') return fallback;
    const stored = value as Record<string, unknown>;
    return {
      size: oneOf(SUBTITLE_SIZES, stored.size, fallback.size),
      background: oneOf(SUBTITLE_BACKGROUNDS, stored.background, fallback.background),
      color: oneOf(SUBTITLE_COLORS, stored.color, fallback.color),
      position: oneOf(SUBTITLE_POSITIONS, stored.position, fallback.position),
      offsetMs: typeof stored.offsetMs === 'number' ? clampOffset(stored.offsetMs) : 0,
    };
  } catch {
    return fallback;
  }
}

/** The subtitle style of this device and a function that changes and stores part of it. */
export function useSubtitleStyle(): [SubtitleStyle, (patch: Partial<SubtitleStyle>) => void] {
  const [style, setStyle] = useState(readSubtitleStyle);
  const update = useCallback((patch: Partial<SubtitleStyle>) => {
    setStyle((current) => {
      const next = { ...current, ...patch };
      next.offsetMs = clampOffset(next.offsetMs);
      try {
        localStorage.setItem(SUBTITLE_STYLE_KEY, JSON.stringify(next));
      } catch {
        // Without storage the style lasts for this visit.
      }
      return next;
    });
  }, []);
  return [style, update];
}
