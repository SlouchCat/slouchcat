import type { MyMediaSetting } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { setHidden, stateOf, toRows, toSettings, type MyMediaRow } from './entries';

const stored: MyMediaSetting[] = [
  { id: 'radio', key: 'radio', position: 1, hideSidebar: false, hideHome: false, count: 3 },
  {
    id: 'library:2',
    key: 'library',
    libraryId: 2,
    libraryName: 'Serien',
    libraryKind: 'video',
    position: 0,
    hideSidebar: false,
    hideHome: true,
    count: 504,
  },
];

describe('«Meine Medien» entries (concept 21 P-11)', () => {
  it('orders by position and drops it, so the same order is no draft', () => {
    const rows = toRows(stored);
    expect(rows.map((row) => row.id)).toEqual(['library:2', 'radio']);
    expect(rows[0]).not.toHaveProperty('position');
    expect(toRows([...stored].reverse())).toEqual(rows);
  });

  it('sends every entry by id with its index as position', () => {
    expect(toSettings(toRows(stored)).map((entry) => [entry.id, entry.position])).toEqual([
      ['library:2', 0],
      ['radio', 1],
    ]);
  });

  it('hides per place and names the state from both flags (O-21)', () => {
    const rows = toRows(stored);
    const [series, radio] = rows as [MyMediaRow, MyMediaRow];
    expect(stateOf(series)).toBe('sidebarOnly');
    expect(stateOf(radio)).toBe('visible');
    const sidebar = setHidden(rows, 'radio', 'sidebar', true);
    expect(stateOf(sidebar[1] as MyMediaRow)).toBe('homeOnly');
    expect(stateOf(setHidden(sidebar, 'radio', 'home', true)[1] as MyMediaRow)).toBe('hidden');
    expect(sidebar[0]).toBe(series);
  });
});
