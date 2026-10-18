import { describe, expect, it } from 'vitest';
import de from './de.json';
import en from './en.json';

function keysOf(value: unknown, prefix = ''): string[] {
  if (typeof value !== 'object' || value === null) return [prefix];
  return Object.entries(value).flatMap(([key, child]) =>
    keysOf(child, prefix ? `${prefix}.${key}` : key),
  );
}

describe('locales', () => {
  it('de.json and en.json define the same keys', () => {
    expect(keysOf(en).sort()).toEqual(keysOf(de).sort());
  });

  it('German texts use Swiss spelling (no ß)', () => {
    expect(JSON.stringify(de)).not.toContain('ß');
  });
});
