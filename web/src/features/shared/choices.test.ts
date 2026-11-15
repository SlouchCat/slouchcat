import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { languageOptions, ratingLabel, ratingOf, ratingOptions } from './choices';

const t = i18n.t.bind(i18n);

describe('ratings (concept 22 stage 1: an age outside the FSK steps)', () => {
  it('names an FSK step «FSK n» and any other age «ab n»', () => {
    expect(ratingLabel(t, 12)).toBe('FSK 12');
    expect(ratingLabel(t, 0)).toBe('FSK 0');
    expect(ratingLabel(t, 13)).toBe('ab 13');
    expect(ratingLabel(t, 17)).toBe('ab 17');
  });

  it('offers a current age outside the steps, in order, so saving keeps it', () => {
    expect(ratingOptions(t, 18, 13).map((o) => o.label)).toEqual([
      'FSK 0',
      'FSK 6',
      'FSK 12',
      'ab 13',
      'FSK 16',
      'FSK 18',
    ]);
    expect(ratingOptions(t, 12, 13).find((o) => o.value === '13')?.isDisabled).toBe(true);
    expect(ratingOptions(t, 18, 12)).toHaveLength(5);
    expect(ratingOf('13')).toBe(13);
    expect(ratingOf('16')).toBe(16);
  });

  it('falls back to 0 for a value that is no age', () => {
    for (const value of ['', 'x', '-1', '19', '1.5']) expect(ratingOf(value)).toBe(0);
  });
});

describe('languageOptions', () => {
  it('names every shipped language in its own language, whatever the UI language', () => {
    expect(languageOptions()).toEqual([
      { value: 'de', label: 'Deutsch (Schweiz)' },
      { value: 'de-DE', label: 'Deutsch (Deutschland)' },
      { value: 'en', label: 'English' },
    ]);
  });
});
