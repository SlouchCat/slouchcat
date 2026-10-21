import { describe, expect, it } from 'vitest';
import { searchRequest, validateSearchParams } from './searchParams';

describe('validateSearchParams', () => {
  it('keeps the valid parameters and drops the rest', () => {
    expect(
      validateSearchParams({
        q: 'Heat',
        genre: 'Drama',
        from: '1990',
        to: 2000,
        rating: '12',
        type: 'movie,episode,series',
        avail: 'local,cloud',
        other: 'x',
      }),
    ).toEqual({
      q: 'Heat',
      genre: 'Drama',
      from: 1990,
      to: 2000,
      rating: 12,
      type: 'movie,series',
      avail: 'local',
    });
  });

  it('an empty or null rating is no filter, not FSK 0', () => {
    expect(validateSearchParams({ rating: '' })).toEqual({});
    expect(validateSearchParams({ rating: null })).toEqual({});
    expect(validateSearchParams({ rating: 'null' })).toEqual({});
    expect(validateSearchParams({ rating: 7 })).toEqual({});
    expect(validateSearchParams({ rating: 0 })).toEqual({ rating: 0 });
    expect(validateSearchParams({ rating: '0' })).toEqual({ rating: 0 });
  });

  it('a text the router read as a number stays the search text', () => {
    // `?q=1917` arrives as the number 1917.
    expect(validateSearchParams({ q: 1917 })).toEqual({ q: '1917' });
    expect(searchRequest(validateSearchParams({ q: 1917 }))).toEqual({ query: '1917' });
  });
});
