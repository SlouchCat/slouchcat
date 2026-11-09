import { describe, expect, it } from 'vitest';
import { defaultOrder, filterCount, libraryRequest, validateLibraryParams } from './libraryParams';

describe('validateLibraryParams', () => {
  it('keeps the valid parameters and drops the rest', () => {
    expect(
      validateLibraryParams({
        content: 'series',
        sort: 'title',
        order: 'desc',
        letter: 'B',
        genre: 'Drama',
        status: 'unwatched,lost,watched',
        from: '1990',
        to: 2000,
        rating: '12',
        other: 'x',
      }),
    ).toEqual({
      content: 'series',
      sort: 'title',
      order: 'desc',
      genre: 'Drama',
      status: 'unwatched,watched',
      from: 1990,
      to: 2000,
      rating: 12,
    });
  });

  it('drops unknown values of every enumeration', () => {
    expect(
      validateLibraryParams({
        content: 'episode',
        sort: 'random',
        order: 'up',
        letter: 'ab',
        status: 'done',
        from: '19',
        to: 'soon',
        rating: 7,
      }),
    ).toEqual({});
  });

  it('drops the letter of an old link: the letter bar jumps since contract 1.21.0', () => {
    expect(validateLibraryParams({ letter: 'S' })).toEqual({});
    expect(validateLibraryParams({ letter: '#' })).toEqual({});
    expect(validateLibraryParams({ sort: 'added', letter: 'S' })).toEqual({ sort: 'added' });
  });

  it('an empty or null rating is no filter, not 0', () => {
    expect(validateLibraryParams({ rating: '' })).toEqual({});
    expect(validateLibraryParams({ rating: null })).toEqual({});
    expect(validateLibraryParams({ rating: 0 })).toEqual({ rating: 0 });
  });

  it('cuts a genre at 80 characters', () => {
    expect(validateLibraryParams({ genre: 'x'.repeat(90) }).genre).toHaveLength(80);
  });
});

describe('libraryRequest', () => {
  it('maps the URL state to the query of listLibraryItems', () => {
    expect(
      libraryRequest(
        validateLibraryParams({
          content: 'movie',
          sort: 'title',
          letter: '#',
          genre: 'Drama',
          status: 'started,watched',
          from: 1990,
          to: 2000,
          rating: 6,
        }),
      ),
    ).toEqual({
      content: 'movie',
      sort: 'title',
      genre: 'Drama',
      status: ['started', 'watched'],
      yearFrom: 1990,
      yearTo: 2000,
      maxRating: 6,
    });
    expect(libraryRequest({})).toEqual({});
  });

  it('counts each status and each year as a filter', () => {
    expect(filterCount({ genre: 'Drama', status: 'unwatched,started', from: 1990 })).toBe(4);
    expect(filterCount({ sort: 'added', content: 'movie' })).toBe(0);
  });
});

describe('defaultOrder', () => {
  it('mirrors the contract: title, age and author ascending, the rest descending', () => {
    expect(defaultOrder()).toBe('asc');
    expect(defaultOrder('title')).toBe('asc');
    expect(defaultOrder('age')).toBe('asc');
    expect(defaultOrder('author')).toBe('asc');
    for (const sort of ['added', 'released', 'vote', 'runtime', 'played'] as const) {
      expect(defaultOrder(sort)).toBe('desc');
    }
  });
});
