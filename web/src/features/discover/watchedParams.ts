import type { RootContent, WatchedSort } from '@slouchcat/api-client';

// The page «Gesehen» keeps its kind and its order in the URL, like the library page (concept 08,
// 7.3): the browser's back button returns to the same view. Kept apart from the page: the router
// imports it for `validateSearch` without loading the page's chunk.

export const WATCHED_CONTENTS = [
  'movie',
  'series',
  'audiobook',
  'podcast',
] as const satisfies readonly RootContent[];

/** State of the page in the URL; without `sort` the newest watched come first. */
export interface WatchedParams {
  content?: RootContent;
  sort?: Exclude<WatchedSort, 'watched'>;
}

/** Keeps only valid parameters; anything else is dropped (`validateSearch`). */
export function validateWatchedParams(search: Record<string, unknown>): WatchedParams {
  const params: WatchedParams = {};
  const content = WATCHED_CONTENTS.find((entry) => entry === search.content);
  if (content) params.content = content;
  if (search.sort === 'title') params.sort = 'title';
  return params;
}
