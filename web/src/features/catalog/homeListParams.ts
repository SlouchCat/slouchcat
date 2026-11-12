import type { HomeListKey, RootContent } from '@slouchcat/api-client';

/** The rows whose title leads to a list page (concept 20 P-7, 8.2), in the order of the start page. */
export const HOME_LIST_KEYS: readonly HomeListKey[] = [
  'spotlight',
  'continue',
  'recent',
  'released',
];

/** The kinds of the top level a list may hold (RootContent), in the order of the filter. */
export const HOME_LIST_CONTENTS: readonly RootContent[] = [
  'movie',
  'series',
  'audiobook',
  'podcast',
];

/**
 * The kinds the filter of each list offers: only those that can match. «Kürzlich veröffentlicht»
 * dates films, series and audiobooks, never podcasts; the spotlight holds films and series of a
 * Trakt list.
 */
export const HOME_LIST_CONTENTS_OF: Readonly<Record<HomeListKey, readonly RootContent[]>> = {
  continue: HOME_LIST_CONTENTS,
  recent: HOME_LIST_CONTENTS,
  released: ['movie', 'series', 'audiobook'],
  spotlight: ['movie', 'series'],
};

/** The search parameters of the list page: `content`, one kind of the top level. */
export function validateHomeListSearch(search: Record<string, unknown>): { content?: RootContent } {
  const content = search.content;
  return typeof content === 'string' && (HOME_LIST_CONTENTS as readonly string[]).includes(content)
    ? { content: content as RootContent }
    : {};
}
