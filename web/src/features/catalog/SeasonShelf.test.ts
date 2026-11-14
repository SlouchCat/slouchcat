import type { ItemRef } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { foldedSeasons } from './SeasonShelf';

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
