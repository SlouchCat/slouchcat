import { describe, expect, it } from 'vitest';
import { RADIO_SEGMENT_SECONDS, timeShiftOf } from './engine';

/** An element whose seekable window runs from `start` to `end`, playing at `currentTime`. */
function element(start: number, end: number, currentTime: number) {
  return {
    currentTime,
    seekable: {
      length: 1,
      start: () => start,
      end: () => end,
    } as unknown as TimeRanges,
  };
}

describe('timeShiftOf', () => {
  it('measures the offset from the live position and the window behind it', () => {
    const audio = element(100, 1908, 1150);
    const shift = timeShiftOf(audio, () => 1900);
    expect(shift.offsetSeconds()).toBe(750);
    expect(shift.seekableSeconds()).toBe(1800);
  });

  it('seeks behind live, never before the oldest position, and back to live', () => {
    const audio = element(100, 1908, 1900);
    const shift = timeShiftOf(audio, () => 1900);
    shift.seekTo(600);
    expect(audio.currentTime).toBe(1300);
    shift.seekTo(5000);
    expect(audio.currentTime).toBe(100);
    shift.toLive();
    expect(audio.currentTime).toBe(1900);
    expect(shift.offsetSeconds()).toBe(0);
  });

  it('does nothing while the live position is unknown', () => {
    const audio = element(100, 1908, 1500);
    const shift = timeShiftOf(audio, () => null);
    shift.seekTo(600);
    expect(audio.currentTime).toBe(1500);
    expect(shift.offsetSeconds()).toBe(0);
    expect(shift.seekableSeconds()).toBe(0);
  });

  it('jumps one segment after the oldest position when the window slid past a pause', () => {
    const audio = element(2400, 4208, 1200);
    const shift = timeShiftOf(audio, () => 4200);
    expect(shift.resumeAt()).toBe(true);
    expect(audio.currentTime).toBe(2400 + RADIO_SEGMENT_SECONDS);
    // Inside the window it stays where it is.
    expect(shift.resumeAt()).toBe(false);
    expect(audio.currentTime).toBe(2400 + RADIO_SEGMENT_SECONDS);
  });
});
