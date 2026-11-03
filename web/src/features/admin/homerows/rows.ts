import type { HomeRowKey, HomeRowSetting } from '@slouchcat/api-client';
import { paths } from '@/app/paths';

export { homeRowTitleKey as rowTitleKey } from '@/features/catalog/labels';

/** A row as the page edits it: its key and whether it is hidden; the position is its index. */
export type HomeRow = Pick<HomeRowSetting, 'key' | 'hidden'>;

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
  [...settings].sort((a, b) => a.position - b.position).map(({ key, hidden }) => ({ key, hidden }));

/** The body of `setHomeRows`: every row with its position. */
export const toSettings = (rows: readonly HomeRow[]): HomeRowSetting[] =>
  rows.map((row, position) => ({ key: row.key, hidden: row.hidden, position }));
