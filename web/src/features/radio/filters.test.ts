import { describe, expect, it } from 'vitest';
import {
  fromRadioFilter,
  hasRadioFilter,
  radioQueryParams,
  toRadioFilter,
  validateRadioSearch,
  valuesOf,
  withValues,
} from './filters';

describe('validateRadioSearch', () => {
  it('keeps valid values, several per filter, each once', () => {
    expect(
      validateRadioSearch({
        q: 'Swiss',
        country: 'ch,DE,CH',
        region: 'CH-ZH,CH-BE',
        language: 'DE',
        genre: 'pop,rock',
        quality: 'high,best',
        titles: true,
        fav: 'true',
        sort: 'name',
      }),
    ).toEqual({
      q: 'Swiss',
      country: 'CH,DE',
      region: 'CH-ZH,CH-BE',
      language: 'de',
      genre: 'pop,rock',
      quality: 'high',
      titles: true,
      fav: true,
      sort: 'name',
    });
  });

  it('drops what is not a filter value', () => {
    expect(
      validateRadioSearch({
        country: 'Schweiz,../x',
        genre: 'Pop Rock',
        sort: 'random',
        titles: 'no',
        extra: '1',
      }),
    ).toEqual({});
  });

  it('drops a region without a country and the default sort', () => {
    expect(validateRadioSearch({ region: 'CH-ZH', sort: 'popular' })).toEqual({});
  });

  it('reads a numeric text search as text', () => {
    expect(validateRadioSearch({ q: 1291 })).toEqual({ q: '1291' });
  });
});

describe('filter values', () => {
  it('sets and removes the values of a facet', () => {
    const search = withValues({ country: 'CH' }, 'genre', ['pop', 'jazz']);
    expect(valuesOf(search, 'genre')).toEqual(['pop', 'jazz']);
    expect(withValues(search, 'genre', [])).toEqual({ country: 'CH' });
    // Removing the country removes its regions too.
    expect(withValues({ country: 'CH', region: 'CH-ZH' }, 'country', [])).toEqual({});
  });

  it('knows whether anything filters', () => {
    expect(hasRadioFilter({})).toBe(false);
    expect(hasRadioFilter({ sort: 'name' })).toBe(false);
    expect(hasRadioFilter({ fav: true })).toBe(true);
    expect(hasRadioFilter({ top: true })).toBe(true);
    expect(hasRadioFilter({ mood: 'chill' })).toBe(true);
  });
});

describe('the server filter', () => {
  it('round-trips through the stored preferences', () => {
    const search = validateRadioSearch({
      q: 'jazz',
      country: 'CH,AT',
      region: 'CH-ZH',
      quality: 'good,high',
      titles: true,
      fav: true,
    });
    const filter = toRadioFilter(search);
    expect(filter).toEqual({
      q: 'jazz',
      country: ['CH', 'AT'],
      region: ['CH-ZH'],
      quality: ['good', 'high'],
      titles: true,
      favorites: true,
    });
    expect(fromRadioFilter(filter, 'quality')).toEqual({ ...search, sort: 'quality' });
  });

  it('asks for the popular order for «Beliebt» and by default', () => {
    expect(radioQueryParams({ top: true, sort: 'name' }).sort).toBe('popular');
    expect(radioQueryParams({}).sort).toBe('popular');
    expect(radioQueryParams({ sort: 'new', genre: 'pop' })).toEqual({
      genre: ['pop'],
      sort: 'new',
    });
  });
});
