import type { ItemRef } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { foldedSeasons, wallEntries } from './SeasonShelf';

const seasons = Array.from(
  { length: 11 },
  (_, index) => ({ id: `s${index + 1}`, title: `Staffel ${index + 1}` }) as ItemRef,
);
const ids = (list: readonly ItemRef[]) => list.map((season) => season.id);

describe('foldedSeasons (concept 21 P-6)', () => {
  it('shows the first two rows while folded', () => {
    expect(ids(foldedSeasons(seasons, 's1', 4, false))).toEqual([
      's1',
      's2',
      's3',
      's4',
      's5',
      's6',
      's7',
      's8',
    ]);
  });

  it('keeps the chosen season in the wall in place of the last one when it lies further down', () => {
    expect(ids(foldedSeasons(seasons, 's10', 4, false))).toEqual([
      's1',
      's2',
      's3',
      's4',
      's5',
      's6',
      's7',
      's10',
    ]);
  });

  it('shows every season unfolded, when two rows hold them all, or before the columns are known', () => {
    expect(foldedSeasons(seasons, 's1', 4, true)).toHaveLength(11);
    expect(foldedSeasons(seasons, 's1', 6, false)).toHaveLength(11);
    expect(foldedSeasons(seasons, 's1', 0, false)).toHaveLength(11);
  });
});

describe('wallEntries (concept 25 M-06)', () => {
  const library = [
    { id: 's1', title: 'Season 1', seasonNumber: 1 },
    { id: 's3', title: 'Season 3', seasonNumber: 3 },
    { id: 'sp', title: 'Specials', seasonNumber: 0 },
  ] as ItemRef[];
  const season = (number: number, state: 'present' | 'missing' | 'announced' | 'requested') => ({
    number,
    episodeCount: 8,
    episodes: state === 'present' ? 8 : 0,
    state,
  });

  it('is the library seasons alone without a catalog', () => {
    expect(wallEntries(library).map((entry) => entry.id)).toEqual(['s1', 's3', 'sp']);
  });

  it('puts the missing seasons between the present ones by number, the specials last', () => {
    const catalog = {
      inProduction: true,
      present: 2,
      total: 3,
      seasons: [
        season(1, 'present'),
        season(2, 'requested'),
        season(3, 'present'),
        season(4, 'announced'),
      ],
    };
    expect(wallEntries(library, catalog).map((entry) => entry.id)).toEqual([
      's1',
      'season-missing-2',
      's3',
      'season-missing-4',
      'sp',
    ]);
  });
});
