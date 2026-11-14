import type { MyMediaSetting } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';

/**
 * An entry of «Meine Medien» as the page edits it (concept 21 P-11): its id (the key, or
 * `library:<id>` for a library), its key, the library's name and kind, the hiding per place (O-21)
 * and what it holds; the position is its index.
 */
export type MyMediaRow = Pick<
  MyMediaSetting,
  'id' | 'key' | 'libraryName' | 'libraryKind' | 'hideSidebar' | 'hideHome' | 'count'
>;

/** Where an entry can be hidden. */
export type MyMediaPlace = 'sidebar' | 'home';

/** The four words of the state column (O-21). */
export type MyMediaState = 'visible' | 'sidebarOnly' | 'homeOnly' | 'hidden';

/** What the server holds, in the page's form: positions dropped, so an equal order is no draft. */
export const toRows = (settings: readonly MyMediaSetting[]): MyMediaRow[] =>
  [...settings]
    .sort((a, b) => a.position - b.position)
    .map(({ id, key, libraryName, libraryKind, hideSidebar, hideHome, count }) =>
      key === 'library'
        ? { id, key, libraryName, libraryKind, hideSidebar, hideHome, count }
        : { id, key, hideSidebar, hideHome, count },
    );

/** The body of `setMyMedia`: every entry by its id, with its position. */
export const toSettings = (rows: readonly MyMediaRow[]): MyMediaSetting[] =>
  rows.map((row, position) => ({
    id: row.id,
    key: row.key,
    hideSidebar: row.hideSidebar,
    hideHome: row.hideHome,
    count: row.count,
    position,
  }));

/** The rows with `id` shown or hidden in `place`. */
export const setHidden = (
  rows: readonly MyMediaRow[],
  id: string,
  place: MyMediaPlace,
  hidden: boolean,
): MyMediaRow[] =>
  rows.map((row) =>
    row.id !== id
      ? row
      : place === 'sidebar'
        ? { ...row, hideSidebar: hidden }
        : { ...row, hideHome: hidden },
  );

export const stateOf = (row: MyMediaRow): MyMediaState =>
  row.hideSidebar && row.hideHome
    ? 'hidden'
    : row.hideSidebar
      ? 'homeOnly'
      : row.hideHome
        ? 'sidebarOnly'
        : 'visible';

/** The entry's name: the library's, or «Radio», «Sammlungen», «User-Empfehlungen». */
export const entryName = (t: TFunction, row: MyMediaRow): string =>
  row.key === 'library' ? (row.libraryName ?? '') : t(`admin.myMedia.name.${row.key}`);

/** The text of what radio, collections and picks hold. */
const CONTENT = {
  radio: 'admin.myMedia.content.stations',
  collections: 'admin.myMedia.content.collections',
  picks: 'admin.myMedia.content.titles',
} as const;

/** What the entry holds, with its noun: «1828 Titel», «4 Hörbücher», «3 Sender», «333 Sammlungen». */
export function contentOf(t: TFunction, number: Intl.NumberFormat, row: MyMediaRow): string {
  const count = row.count;
  if (row.key === 'library') {
    return t('library.page.count', {
      count,
      noun: t(`library.page.noun.${row.libraryKind ?? 'video'}`, { count }),
    });
  }
  return t(CONTENT[row.key], { count, number: number.format(count) });
}
