import type { JobRun, JobTypeInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';

// Text helpers of the jobs page (concept 09): durations rounded the way the page shows them
// (seconds below a minute, minutes, hours from 90 minutes on, "over a day"), the name of a job
// type from the locale with the code as fallback, its description, and the state of a type.

/** Locale key of a job type: `scan.library` → `scan_library` (i18next reads `.` as a separator). */
export const typeKey = (type: string) => type.replace(/\./g, '_');

/** The name of a job type for the page; the code when the locale has none. */
export const typeName = (t: TFunction, type: string) =>
  t(`admin.jobs.types.${typeKey(type)}.name`, { defaultValue: type });

/**
 * What a job type does, in one sentence (concept 15, P-2); empty for a type this server or the
 * locale does not know, never the key or the code.
 */
export const typeDescription = (t: TFunction, info: Pick<JobTypeInfo, 'type' | 'known'>) =>
  info.known ? t(`admin.jobs.types.${typeKey(info.type)}.description`, { defaultValue: '' }) : '';

/** A duration in seconds as the page shows it: never finer than it is known. */
export function formatSeconds(t: TFunction, seconds: number): string {
  const s = Math.max(0, Math.round(seconds));
  if (s < 60) return t('admin.jobs.duration.seconds', { count: s });
  const minutes = Math.round(s / 60);
  if (minutes < 90) return t('admin.jobs.duration.minutes', { count: minutes });
  // Whole minutes first, then hours: 5 h 59 min 30 s carries into 6 h, never "5 h 60 min".
  const hours = Math.floor(minutes / 60);
  if (hours >= 24) {
    const days = Math.floor(hours / 24);
    return days === 1
      ? t('admin.jobs.duration.overDay')
      : t('admin.jobs.duration.days', { count: days });
  }
  const rest = minutes % 60;
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

/**
 * An instant as "01:00" on the clock of `timeZone` (the server's zone, for example when a job
 * type's window opens). Without a zone, or one the browser does not know (a server without `TZ`
 * names its zone "Local"), the browser's zone: in a household mostly the same.
 */
export function formatClock(instant: string, timeZone: string | undefined, language: string) {
  const options: Intl.DateTimeFormatOptions = {
    hour: '2-digit',
    minute: '2-digit',
    hourCycle: 'h23',
  };
  let format: Intl.DateTimeFormat;
  try {
    format = new Intl.DateTimeFormat(language, timeZone ? { ...options, timeZone } : options);
  } catch {
    format = new Intl.DateTimeFormat(language, options);
  }
  return format.format(new Date(instant));
}

/**
 * A one-off date of a schedule (`JobSchedule.once.local`, wall time of the server's zone) as
 * "So., 8. Nov., 03:30": formatted as it stands, never converted to the browser's zone. The text
 * as it is when it is no wall time.
 */
export function formatOnce(local: string, language: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
  if (!match) return local;
  const [year, month, day, hour, minute] = match.slice(1).map(Number) as [
    number,
    number,
    number,
    number,
    number,
  ];
  return new Intl.DateTimeFormat(language, {
    weekday: 'short',
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'UTC',
  }).format(new Date(Date.UTC(year, month - 1, day, hour, minute)));
}

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
