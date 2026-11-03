import { t } from 'i18next';
import { describe, expect, it } from 'vitest';
import { typeName } from './jobsFormat';

describe('job type names', () => {
  it('names metadata.apply from the locale instead of showing the code (concept 12, 10.3)', () => {
    expect(typeName(t, 'metadata.apply')).toBe('Metadaten anwenden');
    expect(t('admin.jobs.types.metadata_apply.trigger')).toBe(
      '«Jetzt anwenden» auf der Seite Metadaten',
    );
  });
});
