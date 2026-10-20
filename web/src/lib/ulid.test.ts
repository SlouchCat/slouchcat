import { describe, expect, it } from 'vitest';
import { isUlid, ulid } from './ulid';

describe('ulid', () => {
  it('is 26 Crockford characters, the contract’s pattern', () => {
    for (let index = 0; index < 50; index += 1) expect(isUlid(ulid())).toBe(true);
  });

  it('starts with the time, so later ids sort after earlier ones', () => {
    const zero = (bytes: Uint8Array) => bytes.fill(0);
    expect(ulid(0, zero)).toBe('0'.repeat(26));
    expect(ulid(1_760_000_000_000, zero) < ulid(1_760_000_000_001, zero)).toBe(true);
    expect(ulid(1_760_000_000_000, zero).slice(0, 10)).toBe(ulid(1_760_000_000_000).slice(0, 10));
  });

  it('differs per call in the random part', () => {
    expect(ulid(5) === ulid(5)).toBe(false);
  });
});
