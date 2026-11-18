import { t } from 'i18next';
import { describe, expect, it } from 'vitest';
import { etaText } from './progress';

describe('the time left of a download', () => {
  it('names minutes, hours with minutes, and whole hours without «0 min»', () => {
    expect(etaText(t, 720)).toBe('noch 12 min');
    expect(etaText(t, 2 * 3600 + 300)).toBe('noch 2 h 5 min');
    expect(etaText(t, 2 * 3600)).toBe('noch 2 h');
  });
});
