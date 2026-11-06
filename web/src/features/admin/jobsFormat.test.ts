import { t } from 'i18next';
import { describe, expect, it } from 'vitest';
import { formatClock, formatOnce, formatSeconds, typeName } from './jobsFormat';

describe('job type names', () => {
  it('names metadata.apply from the locale instead of showing the code (concept 12, 10.3)', () => {
    expect(typeName(t, 'metadata.apply')).toBe('Metadaten anwenden');
    expect(t('admin.jobs.types.metadata_apply.trigger')).toBe(
      '«Jetzt anwenden» auf der Seite Metadaten',
    );
  });

  it('names the keyframe index types (concept 14, 7.3)', () => {
    expect(typeName(t, 'keyframes.backfill')).toBe('Keyframe-Index nachholen');
    expect(typeName(t, 'probe.keyframes')).toBe('Keyframe-Index (ganze Datei lesen)');
  });
});

describe('one-off dates', () => {
  it('formats the server wall time as it stands, whatever zone the browser is in', () => {
    const expected = new Intl.DateTimeFormat('de', {
      weekday: 'short',
      day: 'numeric',
      month: 'short',
      hour: '2-digit',
      minute: '2-digit',
      timeZone: 'UTC',
    }).format(Date.UTC(2026, 10, 7, 3, 30));
    expect(formatOnce('2026-11-07T03:30', 'de')).toBe(expected);
    expect(expected).toMatch(/^Sa\.?, 7\. Nov\.?, 03:30$/);
  });

  it('shows a value that is no wall time as it is', () => {
    expect(formatOnce('soon', 'de')).toBe('soon');
  });
});

describe('window clock', () => {
  it('shows an instant on the clock of the server zone, not the browser one', () => {
    // 00:00 UTC is 01:00 in Zurich in winter and 19:00 the day before in New York.
    expect(formatClock('2026-11-07T00:00:00Z', 'Europe/Zurich', 'de')).toBe('01:00');
    expect(formatClock('2026-11-07T00:00:00Z', 'America/New_York', 'en')).toBe('19:00');
  });

  it('takes the browser zone for a zone it does not know (a server without TZ says "Local")', () => {
    const browser = new Intl.DateTimeFormat('de', {
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    }).format(new Date('2026-11-07T00:00:00Z'));
    expect(formatClock('2026-11-07T00:00:00Z', 'Local', 'de')).toBe(browser);
  });
});

describe('durations', () => {
  it('carries rounded minutes into the hours: 5 h 59 min 30 s is 6 h, never 5 h 60 min', () => {
    expect(formatSeconds(t, 6 * 3600 - 30)).toBe('6 h');
    expect(formatSeconds(t, 6 * 3600 - 31)).toBe('5 h 59 min');
    expect(formatSeconds(t, 2 * 3600 + 40 * 60)).toBe('2 h 40 min');
    expect(formatSeconds(t, 89 * 60)).toBe('89 min');
    expect(formatSeconds(t, 24 * 3600 - 30)).toBe('über 1 Tag');
  });
});
