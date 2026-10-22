import type { StatsDays } from '@slouchcat/api-client';

/** The periods of the statistics page (A-8), in days. */
export const STATS_DAYS = [7, 30, 90, 365] as const satisfies readonly StatsDays[];

/** The period without `?days=` (the server's default as well). */
export const DEFAULT_STATS_DAYS: StatsDays = 30;

export const isStatsDays = (value: number): value is StatsDays =>
  (STATS_DAYS as readonly number[]).includes(value);
