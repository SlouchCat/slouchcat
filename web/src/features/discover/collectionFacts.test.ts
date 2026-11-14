import type { CollectionRef } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import {
  arrangeCollections,
  collectionCaption,
  collectionProgress,
  collectionYears,
} from './collectionFacts';
import { validateCollectionsParams } from './collectionsParams';

const t = i18n.t.bind(i18n);
const ref = (name: string, count: number, started = false, seenCount = 0): CollectionRef => ({
  id: name,
  name,
  count,
  started,
  seenCount,
});

describe('collection facts (concept 21 P-10)', () => {
  it('says the years and the caption under a card', () => {
    expect(collectionYears(t, { firstYear: 2001, lastYear: 2011 })).toBe('2001–2011');
    expect(collectionYears(t, { firstYear: 2001, lastYear: 2001 })).toBe('2001');
    expect(collectionYears(t, {})).toBeUndefined();
    expect(collectionCaption(t, { count: 8, firstYear: 2001, lastYear: 2011 })).toBe(
      '8 Titel · 2001–2011',
    );
    expect(collectionCaption(t, { count: 1 })).toBe('1 Titel');
  });

  it('draws a bar only for a started collection, at least a sliver', () => {
    expect(collectionProgress(t, ref('A', 4))).toBeUndefined();
    expect(collectionProgress(t, ref('A', 4, true, 1))).toEqual({
      percent: 25,
      label: '1 von 4 gesehen',
    });
    expect(collectionProgress(t, ref('A', 4, true, 0))?.percent).toBe(4);
  });

  it('searches, filters by size and sorts by name, count or started first', () => {
    const all = [ref('Zorro', 2), ref('Ärger', 5, true), ref('Alien', 3), ref('Bond', 1, true)];
    const names = (list: CollectionRef[]) => list.map((c) => c.name);
    expect(names(arrangeCollections(all, { q: '', size: 'all', sort: 'name' }, 'de'))).toEqual([
      'Alien',
      'Ärger',
      'Bond',
      'Zorro',
    ]);
    expect(names(arrangeCollections(all, { q: '', size: '3', sort: 'name' }, 'de'))).toEqual([
      'Alien',
      'Ärger',
    ]);
    expect(names(arrangeCollections(all, { q: '', size: 'all', sort: 'count' }, 'de'))).toEqual([
      'Ärger',
      'Alien',
      'Zorro',
      'Bond',
    ]);
    expect(names(arrangeCollections(all, { q: '', size: '2', sort: 'started' }, 'de'))).toEqual([
      'Ärger',
      'Alien',
      'Zorro',
    ]);
    // Case and accents aside.
    expect(names(arrangeCollections(all, { q: ' arg ', size: 'all', sort: 'name' }, 'de'))).toEqual(
      ['Ärger'],
    );
  });

  it('keeps only valid parameters in the URL', () => {
    expect(validateCollectionsParams({ q: ' Bond ', min: 2, sort: 'count' })).toEqual({
      q: ' Bond ',
      min: 2,
      sort: 'count',
    });
    expect(validateCollectionsParams({ q: '  ', min: '4', sort: 'name', other: 1 })).toEqual({});
    expect(validateCollectionsParams({ q: 7, min: '5', sort: 'started' })).toEqual({
      q: '7',
      min: 5,
      sort: 'started',
    });
  });
});
