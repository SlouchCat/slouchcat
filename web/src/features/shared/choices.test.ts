import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { ratingSystemOf } from '@/test/fixtures';
import { languageOptions, ratingLabel, ratingOf, ratingOptions, titleRatingOf } from './choices';

const t = i18n.t.bind(i18n);

describe('ratings (concept 22 stage 3: the steps of the rating country)', () => {
  const de = ratingSystemOf('DE');
  const us = ratingSystemOf('US');
  const fr = ratingSystemOf('FR');

  it('names a step by the certificate of the country and any other age «ab n»', () => {
    expect(ratingLabel(t, 12, de)).toBe('FSK 12');
    expect(ratingLabel(t, 13, de)).toBe('ab 13');
    expect(ratingLabel(t, 13, us)).toBe('PG-13');
    expect(ratingLabel(t, 17, us)).toBe('R / TV-MA');
    expect(ratingLabel(t, 12, fr)).toBe('-12');
    expect(ratingLabel(t, 10, ratingSystemOf('CH'))).toBe('ab 10');
    // Without a rating system (an older server) every age reads «ab n».
    expect(ratingLabel(t, 12)).toBe('ab 12');
  });

  it('offers the steps of the country and keeps a current age outside them', () => {
    expect(ratingOptions(t, fr).map((o) => o.value)).toEqual(['0', '10', '12', '16', '18']);
    expect(ratingOptions(t, us, 18, 12).map((o) => o.label)).toEqual([
      'G / TV-G',
      'PG / TV-PG',
      'TV-Y7',
      'ab 12',
      'PG-13',
      'TV-14',
      'R / TV-MA',
      'NC-17',
    ]);
    expect(ratingOptions(t, us, 12).find((o) => o.value === '13')?.isDisabled).toBe(true);
    expect(ratingOptions(t, de, 18, 12)).toHaveLength(5);
    expect(ratingOptions(t, undefined).map((o) => o.value)).toEqual(['0', '6', '12', '16', '18']);
    expect(ratingOf('13')).toBe(13);
    expect(ratingOf('16')).toBe(16);
  });

  it('falls back to 0 for a value that is no age', () => {
    for (const value of ['', 'x', '-1', '19', '1.5']) expect(ratingOf(value)).toBe(0);
  });

  it('takes a title rating only for an age', () => {
    expect(titleRatingOf('13')).toBe(13);
    expect(titleRatingOf('0')).toBe(0);
    for (const value of ['', 'x', '19']) expect(titleRatingOf(value)).toBeUndefined();
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
