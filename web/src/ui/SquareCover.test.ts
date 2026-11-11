import { describe, expect, it } from 'vitest';
import { coverInitials } from './SquareCover';

describe('coverInitials', () => {
  it('leaves the signs of a title out', () => {
    expect(coverInitials('{ungeskriptet} - Der Meinungsfreiheit verpflichtet.')).toBe('UDM');
    expect(coverInitials('ZEIT WISSEN. Woher weisst Du das?')).toBe('ZWW');
    expect(coverInitials('Tagesgespräch')).toBe('Ta');
    expect(coverInitials('???')).toBe('?');
  });
});
