import { describe, expect, it } from 'vitest';
import { adminSearch } from './router';

describe('adminSearch (concept 12, S-14)', () => {
  it('keeps ids in ?tab= and ?item=', () => {
    expect(adminSearch({ tab: 'failed', item: '01J9ZK3Q4V8M2N5P7R0S1T2W3X' })).toEqual({
      tab: 'failed',
      item: '01J9ZK3Q4V8M2N5P7R0S1T2W3X',
    });
    expect(adminSearch({ item: 'me' })).toEqual({ item: 'me' });
    // The router parses ?item=12 as a number.
    expect(adminSearch({ item: 12 })).toEqual({ item: '12' });
  });

  it('drops anything that is not an id, and every other parameter', () => {
    expect(adminSearch({})).toEqual({});
    expect(adminSearch({ tab: '../admin', item: 'a b', library: 3, extra: 'x' })).toEqual({});
    expect(adminSearch({ tab: 'x'.repeat(65), item: 1.5 })).toEqual({});
    expect(adminSearch({ tab: ['failed'] })).toEqual({});
  });
});
