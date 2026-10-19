import { describe, expect, it } from 'vitest';
import { itemAccentStyle } from './accent';

describe('itemAccentStyle', () => {
  it('hands a valid title accent to CSS as --sc-item-accent', () => {
    expect(itemAccentStyle('#3a7bd5')).toEqual({ '--sc-item-accent': '#3a7bd5' });
    expect(itemAccentStyle('#3A7BD5')).toEqual({ '--sc-item-accent': '#3a7bd5' });
  });

  it('ignores missing and malformed values, so CSS falls back to the tokens', () => {
    for (const value of [undefined, '', '#fff', 'red', '#3a7bd5ff', 'var(--x)', '#3a7bd5;x:y']) {
      expect(itemAccentStyle(value)).toBeUndefined();
    }
  });
});
