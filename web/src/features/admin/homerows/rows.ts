import type { HomeRowKey, HomeRowSetting } from '@slouchcat/api-client';
import { paths } from '@/app/paths';

export { homeRowTitle as rowTitle } from '@/features/catalog/labels';

/**
 * A row as the page edits it: its id (the key, or `library:<id>` for a library row, concept 15
 * P-7), its key, the library's name and whether it is hidden; the position is its index.
 */
export type HomeRow = Pick<HomeRowSetting, 'id' | 'key' | 'libraryName' | 'hidden'>;

/** The most rows the start page shows (F-12, E-9): hidden rows do not count. */
export const MAX_VISIBLE_ROWS = 15;

/** Rows whose content is kept on a page of its own: the fold-out links there. */
export const rowPage = {
  spotlight: { href: paths.adminSpotlight, titleKey: 'admin.spotlight.title' },
  picks: { href: paths.adminPicks, titleKey: 'admin.picks.title' },
  collections: { href: paths.adminCollections, titleKey: 'admin.collections.title' },
} as const satisfies Partial<Record<HomeRowKey, { href: string; titleKey: string }>>;

export const pageOf = (key: HomeRowKey) =>
  key in rowPage ? rowPage[key as keyof typeof rowPage] : undefined;

/** What the server holds, in the page's form: positions dropped, so an equal order is no draft. */
export const toRows = (settings: readonly HomeRowSetting[]): HomeRow[] =>
  [...settings]
    .sort((a, b) => a.position - b.position)
    .map(({ id, key, libraryName, hidden }) =>
      libraryName === undefined ? { id, key, hidden } : { id, key, libraryName, hidden },
    );

/** The body of `setHomeRows`: every row by its id, with its position. */
export const toSettings = (rows: readonly HomeRow[]): HomeRowSetting[] =>
  rows.map((row, position) => ({ id: row.id, key: row.key, hidden: row.hidden, position }));

/** The number of rows not hidden. */
export const visibleCount = (rows: readonly HomeRow[]): number =>
  rows.filter((row) => !row.hidden).length;

/**
 * The rows with `id` shown or hidden, or undefined when showing it would make more than
 * MAX_VISIBLE_ROWS visible (F-12): the page says so instead of changing anything.
 */
export function setHidden(
  rows: readonly HomeRow[],
  id: string,
  hidden: boolean,
): HomeRow[] | undefined {
  const row = rows.find((entry) => entry.id === id);
  if (!hidden && row?.hidden && visibleCount(rows) >= MAX_VISIBLE_ROWS) return undefined;
  return rows.map((entry) => (entry.id === id ? { ...entry, hidden } : entry));
}
