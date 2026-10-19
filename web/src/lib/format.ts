import type { TFunction } from 'i18next';

const pad = (value: number) => String(value).padStart(2, '0');

/** Clock time `hh:mm:ss` (always with hours), for resume labels. */
export function formatClock(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor((total % 3600) / 60))}:${pad(total % 60)}`;
}

/** Player time: `h:mm:ss` when the title runs an hour or longer, otherwise `m:ss`. */
export function formatPlayerTime(ms: number, durationMs: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = pad(total % 60);
  return durationMs >= 3_600_000 ? `${hours}:${pad(minutes)}:${seconds}` : `${minutes}:${seconds}`;
}

/** Runtime such as "2 h 15 min", "45 min" or "2 h". */
export function formatRuntime(t: TFunction, minutes: number): string {
  const hours = Math.floor(minutes / 60);
  const rest = Math.round(minutes % 60);
  if (hours === 0) return t('format.minutes', { minutes: rest });
  if (rest === 0) return t('format.hours', { hours });
  return t('format.hoursMinutes', { hours, minutes: rest });
}

/** File size in the largest fitting unit, localised ("4,2 GB"). */
export function formatBytes(bytes: number, language: string): string {
  const units = ['byte', 'kilobyte', 'megabyte', 'gigabyte', 'terabyte'] as const;
  let value = bytes;
  let unit = 0;
  while (value >= 1000 && unit < units.length - 1) {
    value /= 1000;
    unit += 1;
  }
  return new Intl.NumberFormat(language, {
    style: 'unit',
    unit: units[unit],
    unitDisplay: 'short',
    maximumFractionDigits: unit >= 3 ? 1 : 0,
  }).format(value);
}

/**
 * Short resolution class. Width counts as well, so letterboxed scope films (1920 × 800) still
 * read as 1080p.
 */
export function resolutionClass(t: TFunction, width: number, height: number): string {
  if (width >= 3800 || height >= 2000) return t('format.uhd');
  if (width >= 1900 || height >= 1000) return t('format.fhd');
  if (width >= 1260 || height >= 700) return t('format.hd');
  return t('format.sd');
}

/** "1920 × 1080 · 1080p": the size of a video and its class. */
export function formatResolution(t: TFunction, width: number, height: number): string {
  return t('format.resolutionWithClass', {
    resolution: t('format.resolution', { width, height }),
    class: resolutionClass(t, width, height),
  });
}

export function formatDateTime(iso: string, language: string): string {
  return new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(iso),
  );
}
