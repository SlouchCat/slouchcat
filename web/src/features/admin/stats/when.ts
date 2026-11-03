// «Wann geschaut wird» (concept 12, 10.2 and 11.4): the server sends the played time per quarter
// hour of the week (672 values, Monday 00:00 first, in its own time zone); the page only adds them
// up. No rule is decided here (rule 12): it is the same sum, folded three ways.

/** The days of the week in the contract's order, as the keys of `ui.weekdays`. */
export const WEEKDAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;

export type Weekday = (typeof WEEKDAYS)[number];

const QUARTERS_PER_HOUR = 4;
const HOURS_PER_DAY = 24;

/** 7 × 24 values, Monday 00:00 first: each hour the sum of its four quarter hours. */
export function hourGrid(quarters: readonly number[]): number[] {
  return Array.from({ length: WEEKDAYS.length * HOURS_PER_DAY }, (_, hour) => {
    let sum = 0;
    for (let quarter = 0; quarter < QUARTERS_PER_HOUR; quarter++) {
      sum += quarters[hour * QUARTERS_PER_HOUR + quarter] ?? 0;
    }
    return sum;
  });
}

/** «Nach Tageszeit»: 24 values, each hour summed over the seven days. */
export function byHour(grid: readonly number[]): number[] {
  return Array.from({ length: HOURS_PER_DAY }, (_, hour) =>
    WEEKDAYS.reduce((sum, _day, day) => sum + (grid[day * HOURS_PER_DAY + hour] ?? 0), 0),
  );
}

/** «Nach Wochentag»: 7 values Monday to Sunday, each day summed over its 24 hours. */
export function byWeekday(grid: readonly number[]): number[] {
  return WEEKDAYS.map((_day, day) =>
    grid
      .slice(day * HOURS_PER_DAY, (day + 1) * HOURS_PER_DAY)
      .reduce((sum, value) => sum + value, 0),
  );
}

/** The index of the highest value (the first of equal ones), or undefined when all are 0. */
export function peakIndex(values: readonly number[]): number | undefined {
  let peak: number | undefined;
  values.forEach((value, index) => {
    if (value > 0 && (peak === undefined || value > (values[peak] ?? 0))) peak = index;
  });
  return peak;
}

/** The hour of the week with the most watch time: its day and hour, or undefined when none. */
export function peakOf(grid: readonly number[]): { day: Weekday; hour: number } | undefined {
  const index = peakIndex(grid);
  if (index === undefined) return undefined;
  const day = WEEKDAYS[Math.floor(index / HOURS_PER_DAY)];
  return day && { day, hour: index % HOURS_PER_DAY };
}

/** The whole weeks the grid covers (`timeWindowDays` is 7, 28 or 84). */
export const windowWeeks = (timeWindowDays: number) => Math.max(1, Math.round(timeWindowDays / 7));
