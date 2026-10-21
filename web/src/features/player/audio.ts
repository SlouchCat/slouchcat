import type { Chapter } from '@slouchcat/api-client';
import { chapterAt } from './labels';

// Pure rules of the audio player (M6, design 4): speed steps, where the position lies inside the
// book's chapters, the chapter jumps and the sleep timer. No DOM, so the player, the offline
// player and the unit tests share them.

/** Speeds offered in the menu (H-3). */
export const RATE_PRESETS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3] as const;
export const MIN_RATE = 0.5;
export const MAX_RATE = 3;
/** One press of `[` or `]`. */
export const RATE_STEP = 0.1;

/** A speed in the range the contract accepts, rounded to hundredths (no 1.2000000000000002). */
export function clampRate(rate: number): number {
  if (!Number.isFinite(rate)) return 1;
  return Math.round(Math.min(MAX_RATE, Math.max(MIN_RATE, rate)) * 100) / 100;
}

/** The speed one step slower (-1) or faster (+1). */
export const stepRate = (rate: number, direction: 1 | -1): number =>
  clampRate(rate + direction * RATE_STEP);

/** Where the position lies in the chapters: the chapter, the time into it and its length. */
export interface ChapterPosition {
  /** Index of the chapter, -1 before the first one or without chapters. */
  index: number;
  chapter: Chapter | undefined;
  /** Milliseconds since the chapter started. */
  offsetMs: number;
  /** Length of the chapter: up to its end, else up to the next start or the end of the book. */
  lengthMs: number;
}

export function chapterPosition(
  chapters: readonly Chapter[],
  positionMs: number,
  durationMs: number,
): ChapterPosition {
  const index = chapterAt(chapters, positionMs);
  const chapter = chapters[index];
  if (!chapter) return { index: -1, chapter: undefined, offsetMs: 0, lengthMs: 0 };
  const next = chapters[index + 1];
  const end = chapter.endMs > chapter.startMs ? chapter.endMs : (next?.startMs ?? durationMs);
  return {
    index,
    chapter,
    offsetMs: Math.max(0, positionMs - chapter.startMs),
    lengthMs: Math.max(0, end - chapter.startMs),
  };
}

/** Within this much of a chapter's start, "previous chapter" goes to the one before. */
export const RESTART_CHAPTER_MS = 3000;

/**
 * Target of "previous chapter": the start of the current chapter, or of the one before when the
 * position is still near the start (like the previous-track button of a music player).
 */
export function previousChapterStart(chapters: readonly Chapter[], positionMs: number): number {
  const index = chapterAt(chapters, positionMs);
  const current = chapters[index];
  if (!current) return 0;
  if (positionMs - current.startMs > RESTART_CHAPTER_MS) return current.startMs;
  return chapters[index - 1]?.startMs ?? 0;
}

/** Target of "next chapter": the start of the following one, or null in the last chapter. */
export function nextChapterStart(chapters: readonly Chapter[], positionMs: number): number | null {
  return chapters[chapterAt(chapters, positionMs) + 1]?.startMs ?? null;
}

/** Choices of the sleep timer menu (`T`). */
export const SLEEP_MINUTES = [15, 30, 45, 60] as const;
export type SleepChoice = 'off' | 'chapter' | `${(typeof SLEEP_MINUTES)[number]}`;

/**
 * A running sleep timer: after a span of wall-clock time, or at the end of the chapter that was
 * playing when it was set. It runs on the device only; when it fires, the player pauses (and the
 * pause reports the progress like any other).
 */
export type SleepTimer =
  | { kind: 'off' }
  | { kind: 'minutes'; minutes: number; endsAt: number }
  | { kind: 'chapter'; endMs: number };

export const SLEEP_OFF: SleepTimer = { kind: 'off' };

/** The timer a menu choice starts at device time `now` and book position `positionMs`. */
export function startSleep(
  choice: SleepChoice,
  now: number,
  chapters: readonly Chapter[],
  positionMs: number,
  durationMs: number,
): SleepTimer {
  if (choice === 'off') return SLEEP_OFF;
  if (choice === 'chapter') {
    const { chapter, lengthMs } = chapterPosition(chapters, positionMs, durationMs);
    // Without chapters the "chapter" is the whole title.
    return { kind: 'chapter', endMs: chapter ? chapter.startMs + lengthMs : durationMs };
  }
  const minutes = Number(choice);
  return { kind: 'minutes', minutes, endsAt: now + minutes * 60_000 };
}

/** The menu key of a running timer. */
export const sleepChoiceOf = (timer: SleepTimer): SleepChoice =>
  timer.kind === 'off'
    ? 'off'
    : timer.kind === 'chapter'
      ? 'chapter'
      : (String(timer.minutes) as SleepChoice);

/**
 * Wall-clock milliseconds until the timer fires, or null when it is off. At the end of a chapter
 * the rest of the chapter plays at `rate`, so it counts faster at a higher speed.
 */
export function sleepRemainingMs(
  timer: SleepTimer,
  now: number,
  positionMs: number,
  rate: number,
): number | null {
  switch (timer.kind) {
    case 'off':
      return null;
    case 'minutes':
      return Math.max(0, timer.endsAt - now);
    case 'chapter':
      return Math.max(0, (timer.endMs - positionMs) / (rate > 0 ? rate : 1));
  }
}

/** The timer has fired: the player pauses now. */
export function sleepDue(timer: SleepTimer, now: number, positionMs: number): boolean {
  const left = sleepRemainingMs(timer, now, positionMs, 1);
  return left !== null && left <= 0;
}
