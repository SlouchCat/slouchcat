import type { HomeRowSetting } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import {
  MAX_VISIBLE_ROWS,
  setHidden,
  toRows,
  toSettings,
  visibleCount,
  type HomeRow,
} from './rows';

// The rows of the admin page «Startseite» by id (concept 15 P-7): the library rows share the key
// `library` and are told apart by `id`; at most fifteen rows are visible (F-12).

const settings: HomeRowSetting[] = [
  {
    id: 'library:2',
    key: 'library',
    libraryId: 2,
    libraryName: 'Serien',
    position: 2,
    hidden: true,
  },
  { id: 'spotlight', key: 'spotlight', position: 0, hidden: false },
  {
    id: 'library:1',
    key: 'library',
    libraryId: 1,
    libraryName: 'Filme',
    position: 1,
    hidden: false,
  },
];

describe('home rows of the admin page (concept 15 P-7)', () => {
  it('orders by position and keeps id, key and the library name', () => {
    expect(toRows(settings)).toEqual([
      { id: 'spotlight', key: 'spotlight', hidden: false },
      { id: 'library:1', key: 'library', libraryName: 'Filme', hidden: false },
      { id: 'library:2', key: 'library', libraryName: 'Serien', hidden: true },
    ]);
  });

  it('sends every row by id with its index as position', () => {
    expect(toSettings(toRows(settings))).toEqual([
      { id: 'spotlight', key: 'spotlight', hidden: false, position: 0 },
      { id: 'library:1', key: 'library', hidden: false, position: 1 },
      { id: 'library:2', key: 'library', hidden: true, position: 2 },
    ]);
  });

  it('hides and shows one library row by id, not every row of its key', () => {
    const rows = toRows(settings);
    expect(setHidden(rows, 'library:1', true)?.map((row) => row.hidden)).toEqual([
      false,
      true,
      true,
    ]);
    expect(setHidden(rows, 'library:2', false)?.map((row) => row.hidden)).toEqual([
      false,
      false,
      false,
    ]);
  });

  it('refuses a sixteenth visible row and lets one be hidden', () => {
    const rows: HomeRow[] = Array.from({ length: MAX_VISIBLE_ROWS + 1 }, (_, index) => ({
      id: `library:${index}`,
      key: 'library',
      libraryName: `L${index}`,
      hidden: index === MAX_VISIBLE_ROWS,
    }));
    expect(visibleCount(rows)).toBe(15);
    expect(setHidden(rows, `library:${MAX_VISIBLE_ROWS}`, false)).toBeUndefined();
    const fewer = setHidden(rows, 'library:0', true);
    expect(fewer && visibleCount(fewer)).toBe(14);
    expect(fewer && setHidden(fewer, `library:${MAX_VISIBLE_ROWS}`, false)).toBeDefined();
  });
});
