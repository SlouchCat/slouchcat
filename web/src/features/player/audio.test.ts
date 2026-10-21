import type { Chapter } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import {
  chapterPosition,
  clampRate,
  nextChapterStart,
  previousChapterStart,
  RATE_PRESETS,
  sleepChoiceOf,
  sleepDue,
  sleepRemainingMs,
  startSleep,
  stepRate,
} from './audio';

// Twelve chapters of 5 s, like the M4B test book (H-1).
const book: Chapter[] = Array.from({ length: 12 }, (_, index) => ({
  startMs: index * 5000,
  endMs: (index + 1) * 5000,
  title: `Kapitel ${index + 1}`,
}));

describe('speed', () => {
  it('steps by 0.1 within 0.5 … 3.0 without float noise', () => {
    expect(stepRate(1, 1)).toBe(1.1);
    expect(stepRate(1.1, 1)).toBe(1.2);
    expect(stepRate(1.25, -1)).toBe(1.15);
    expect(stepRate(2.95, 1)).toBe(3);
    expect(stepRate(3, 1)).toBe(3);
    expect(stepRate(0.55, -1)).toBe(0.5);
    expect(stepRate(0.5, -1)).toBe(0.5);
    // Ten steps up from 1.0 land exactly on 2.0 (H-3 by keyboard).
    let rate = 1;
    for (let step = 0; step < 10; step += 1) rate = stepRate(rate, 1);
    expect(rate).toBe(2);
  });

  it('clamps any value into the range; the presets are valid as they are', () => {
    expect(clampRate(0.1)).toBe(0.5);
    expect(clampRate(7)).toBe(3);
    expect(clampRate(Number.NaN)).toBe(1);
    expect(RATE_PRESETS.map(clampRate)).toEqual([...RATE_PRESETS]);
  });
});

describe('chapter math', () => {
  it('finds the chapter, the time into it and its length', () => {
    expect(chapterPosition(book, 12_340, 60_000)).toEqual({
      index: 2,
      chapter: book[2],
      offsetMs: 2340,
      lengthMs: 5000,
    });
    expect(chapterPosition(book, 0, 60_000).index).toBe(0);
    expect(chapterPosition(book, 59_999, 60_000)).toMatchObject({ index: 11, offsetMs: 4999 });
    expect(chapterPosition([], 1000, 60_000)).toEqual({
      index: -1,
      chapter: undefined,
      offsetMs: 0,
      lengthMs: 0,
    });
  });

  it('takes the next start or the book end when a chapter has no end of its own', () => {
    const marks: Chapter[] = [
      { startMs: 0, endMs: 0 },
      { startMs: 8000, endMs: 0 },
    ];
    expect(chapterPosition(marks, 1000, 20_000).lengthMs).toBe(8000);
    expect(chapterPosition(marks, 9000, 20_000).lengthMs).toBe(12_000);
  });

  it('previous chapter: the current start, or the one before near the start', () => {
    expect(previousChapterStart(book, 28_500)).toBe(25_000);
    expect(previousChapterStart(book, 26_000)).toBe(20_000);
    expect(previousChapterStart(book, 1000)).toBe(0);
    expect(previousChapterStart([], 1000)).toBe(0);
  });

  it('next chapter: the following start, none in the last one', () => {
    expect(nextChapterStart(book, 27_500)).toBe(30_000);
    expect(nextChapterStart(book, 0)).toBe(5000);
    expect(nextChapterStart(book, 57_000)).toBeNull();
    expect(nextChapterStart([], 0)).toBeNull();
  });
});

describe('sleep timer', () => {
  const now = 1_000_000;

  it('minutes run on the wall clock, independent of the speed', () => {
    const timer = startSleep('15', now, book, 12_000, 60_000);
    expect(timer).toEqual({ kind: 'minutes', minutes: 15, endsAt: now + 900_000 });
    expect(sleepChoiceOf(timer)).toBe('15');
    expect(sleepRemainingMs(timer, now + 60_000, 0, 2)).toBe(840_000);
    expect(sleepDue(timer, now + 899_999, 0)).toBe(false);
    expect(sleepDue(timer, now + 900_000, 0)).toBe(true);
  });

  it('the end of the chapter counts in book time, faster at a higher speed', () => {
    const timer = startSleep('chapter', now, book, 12_000, 60_000);
    expect(timer).toEqual({ kind: 'chapter', endMs: 15_000 });
    expect(sleepChoiceOf(timer)).toBe('chapter');
    expect(sleepRemainingMs(timer, now, 12_000, 1)).toBe(3000);
    expect(sleepRemainingMs(timer, now, 12_000, 2)).toBe(1500);
    expect(sleepDue(timer, now, 14_900)).toBe(false);
    expect(sleepDue(timer, now, 15_000)).toBe(true);
    // A title without chapters sleeps at its end.
    expect(startSleep('chapter', now, [], 1000, 8000)).toEqual({ kind: 'chapter', endMs: 8000 });
  });

  it('off never fires', () => {
    const timer = startSleep('off', now, book, 0, 60_000);
    expect(sleepChoiceOf(timer)).toBe('off');
    expect(sleepRemainingMs(timer, now, 0, 1)).toBeNull();
    expect(sleepDue(timer, now + 1e9, 1e9)).toBe(false);
  });
});
