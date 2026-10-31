import type { JobRun, JobTypeInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';

// Text helpers of the jobs page (concept 09): durations rounded the way the page shows them
// (seconds below a minute, minutes, hours from 90 minutes on, "over a day"), the name of a job
// type from the locale with the code as fallback, and the state of a type.

/** Locale key of a job type: `scan.library` → `scan_library` (i18next reads `.` as a separator). */
export const typeKey = (type: string) => type.replace(/\./g, '_');

/** The name of a job type for the page; the code when the locale has none. */
export const typeName = (t: TFunction, type: string) =>
  t(`admin.jobs.types.${typeKey(type)}.name`, { defaultValue: type });

/** A duration in seconds as the page shows it: never finer than it is known. */
export function formatSeconds(t: TFunction, seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return t('admin.jobs.duration.seconds', { count: s });
  const minutes = Math.round(s / 60);
  if (minutes < 90) return t('admin.jobs.duration.minutes', { count: minutes });
  const hours = Math.floor(s / 3600);
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    return days === 1
      ? t('admin.jobs.duration.overDay')
      : t('admin.jobs.duration.days', { count: days });
  }
  const rest = Math.round((s - hours * 3600) / 60);
  if (rest === 0) return t('admin.jobs.duration.hours', { count: hours });
  return t('admin.jobs.duration.hoursMinutes', { hours, minutes: rest });
}

/** An interval of a schedule (every 6 h, daily): exact, not rounded. */
export function formatInterval(t: TFunction, seconds: number): string {
  if (seconds % 86_400 === 0) return t('admin.jobs.duration.days', { count: seconds / 86_400 });
  if (seconds % 3600 === 0) return t('admin.jobs.duration.hours', { count: seconds / 3600 });
  if (seconds % 60 === 0) return t('admin.jobs.duration.minutes', { count: seconds / 60 });
  return t('admin.jobs.duration.seconds', { count: seconds });
}

/** Minutes of the day as "03:00". */
export const formatMinute = (minute: number) =>
  `${String(Math.floor(minute / 60)).padStart(2, '0')}:${String(minute % 60).padStart(2, '0')}`;

export type TypeState = 'idle' | 'queued' | 'running' | 'paused' | 'stopping' | 'unknown';

/** The one word on the card's chip. */
export function typeState(info: JobTypeInfo): TypeState {
  if (!info.known) return 'unknown';
  if (info.paused) return info.stopping ? 'stopping' : 'paused';
  if (info.counts.running > 0) return 'running';
  if (info.counts.queued > 0) return 'queued';
  return 'idle';
}

/** Seconds since the run started, from the clock `now` (the client counts up between polls). */
export function elapsedSeconds(run: JobRun, now: number): number {
  if (run.state !== 'running' || !run.startedAt) return run.durationSeconds ?? 0;
  return Math.max(run.durationSeconds ?? 0, (now - Date.parse(run.startedAt)) / 1000);
}
