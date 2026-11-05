import { t } from 'i18next';
import { describe, expect, it } from 'vitest';
import { formatOnce, typeName } from './jobsFormat';

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
