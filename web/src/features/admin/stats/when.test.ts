import { describe, expect, it } from 'vitest';
import { byHour, byWeekday, hourGrid, peakOf, windowWeeks } from './when';

// The quarter hours of the week (contract 1.8.0: 672 values, Monday 00:00 first) folded into
// hours, hours of the day and days of the week. The expected values are counted by hand.

/** 672 values: quarter `q` of the week holds `q % 4 + 1` (1, 2, 3, 4 in every hour). */
const ramp = Array.from({ length: 672 }, (_, quarter) => (quarter % 4) + 1);

describe('watch time by hour and weekday', () => {
  it('adds the four quarter hours of each hour', () => {
    const grid = hourGrid(ramp);
    expect(grid).toHaveLength(168);
    // 1 + 2 + 3 + 4 in every hour.
    expect(new Set(grid)).toEqual(new Set([10]));
  });

  it('sums hours over the days and days over the hours', () => {
    const quarters = Array.from({ length: 672 }, () => 0);
    quarters[0] = 5; // Monday 00:00
    quarters[3] = 7; // Monday 00:45
    quarters[4 * 96 + 20 * 4 + 2] = 30; // Friday 20:30
    quarters[6 * 96 + 20 * 4] = 11; // Sunday 20:00
    quarters[671] = 2; // Sunday 23:45
    const grid = hourGrid(quarters);
    const hours = byHour(grid);
    expect(hours).toHaveLength(24);
    expect(hours[0]).toBe(12);
    expect(hours[20]).toBe(41);
    expect(hours[23]).toBe(2);
    expect(hours.reduce((sum, value) => sum + value, 0)).toBe(55);
    expect(byWeekday(grid)).toEqual([12, 0, 0, 0, 30, 0, 13]);
    expect(peakOf(grid)).toEqual({ day: 'fri', hour: 20 });
  });

  it('takes the first of equal peaks and has none without watch time', () => {
    const quarters = Array.from({ length: 672 }, () => 0);
    quarters[2 * 96 + 8 * 4] = 9; // Wednesday 08:00
    quarters[5 * 96 + 8 * 4] = 9; // Saturday 08:00
    expect(peakOf(hourGrid(quarters))).toEqual({ day: 'wed', hour: 8 });
    expect(peakOf(hourGrid(Array.from({ length: 672 }, () => 0)))).toBeUndefined();
  });

  it('counts the window in whole weeks', () => {
    expect([7, 28, 84].map(windowWeeks)).toEqual([1, 4, 12]);
  });
});
