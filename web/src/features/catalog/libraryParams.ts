import type { components, paths, Rating, RootContent } from '@slouchcat/api-client';

// The library page keeps its state in the URL (concept 08, 7.3), like the search page: a view can
// be shared, reloaded and reached again with the browser's back button. Kept apart from the page:
// the router imports it for `validateSearch` without loading the page's chunk.

export type LibrarySort = components['schemas']['LibrarySort'];
export type LibraryItemStatus = components['schemas']['LibraryItemStatus'];
export type SortOrder = 'asc' | 'desc';
/** The query of listLibraryItems without paging. */
export type LibraryRequest = Omit<
  NonNullable<paths['/libraries/{id}/items']['get']['parameters']['query']>,
  'offset' | 'limit'
>;

export const CONTENTS = [
  'movie',
  'series',
  'audiobook',
  'podcast',
] as const satisfies readonly RootContent[];
export const SORTS = [
  'title',
  'added',
  'released',
  'vote',
  'runtime',
  'age',
  'played',
  'author',
] as const satisfies readonly LibrarySort[];
export const STATUSES = [
  'unwatched',
  'started',
  'watched',
] as const satisfies readonly LibraryItemStatus[];
export const RATINGS = [0, 6, 12, 16, 18] as const satisfies readonly Rating[];
/** The letter bar: digits and other characters (`#`) first, then A to Z. */
export const LETTERS = ['#', ...'ABCDEFGHIJKLMNOPQRSTUVWXYZ'] as const;

/** Library state in the URL: `status` is a comma-separated list, `from` and `to` are years. */
export interface LibraryParams {
  content?: RootContent;
  sort?: LibrarySort;
  order?: SortOrder;
  letter?: string;
  genre?: string;
  status?: string;
  from?: number;
  to?: number;
  rating?: Rating;
}

const year = (value: unknown): number | undefined => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isInteger(parsed) && parsed >= 1000 && parsed <= 9999 ? parsed : undefined;
};

const oneOf = <T extends string>(value: unknown, allowed: readonly T[]): T | undefined =>
  allowed.find((entry) => entry === value);

const list = <T extends string>(value: unknown, allowed: readonly T[]): T[] =>
  typeof value === 'string' ? allowed.filter((entry) => value.split(',').includes(entry)) : [];

/** Keeps only valid library parameters; anything else is dropped (`validateSearch`). */
export function validateLibraryParams(search: Record<string, unknown>): LibraryParams {
  const params: LibraryParams = {};
  const content = oneOf(search.content, CONTENTS);
  if (content) params.content = content;
  const sort = oneOf(search.sort, SORTS);
  if (sort) params.sort = sort;
  const order = oneOf(search.order, ['asc', 'desc'] as const);
  if (order) params.order = order;
  // The letter filters only a list sorted by title (the server answers 422 otherwise).
  const letter = oneOf(search.letter, LETTERS);
  if (letter && (sort ?? 'title') === 'title') params.letter = letter;
  if (typeof search.genre === 'string' && search.genre !== '') {
    params.genre = search.genre.slice(0, 80);
  }
  const status = list(search.status, STATUSES);
  if (status.length > 0) params.status = status.join(',');
  const from = year(search.from);
  if (from !== undefined) params.from = from;
  const to = year(search.to);
  if (to !== undefined) params.to = to;
  // Only a number or a non-empty string: `?rating=` or `?rating=null` would read as 0.
  const rating =
    typeof search.rating === 'number' || (typeof search.rating === 'string' && search.rating !== '')
      ? Number(search.rating)
      : undefined;
  if (rating !== undefined && (RATINGS as readonly number[]).includes(rating)) {
    params.rating = rating as Rating;
  }
  return params;
}

export const statusesOf = (params: LibraryParams): LibraryItemStatus[] =>
  list(params.status, STATUSES);

/**
 * The direction a sort order has without `order`, as the contract defines it: title, age rating
 * and author ascending, everything else descending. The server applies it; the client only
 * labels the direction button with it.
 */
export const defaultOrder = (sort: LibrarySort = 'title'): SortOrder =>
  sort === 'title' || sort === 'age' || sort === 'author' ? 'asc' : 'desc';

/** Number of active filters of the filter panel (each status counts, the years count apart). */
export const filterCount = (params: LibraryParams): number =>
  (params.genre !== undefined ? 1 : 0) +
  statusesOf(params).length +
  (params.from !== undefined ? 1 : 0) +
  (params.to !== undefined ? 1 : 0) +
  (params.rating !== undefined ? 1 : 0);

/** The query of listLibraryItems for the URL state, without paging. */
export function libraryRequest(params: LibraryParams): LibraryRequest {
  const status = statusesOf(params);
  return {
    ...(params.content !== undefined ? { content: params.content } : {}),
    ...(params.sort !== undefined ? { sort: params.sort } : {}),
    ...(params.order !== undefined ? { order: params.order } : {}),
    ...(params.letter !== undefined ? { letter: params.letter } : {}),
    ...(params.genre !== undefined ? { genre: params.genre } : {}),
    ...(status.length > 0 ? { status } : {}),
    ...(params.from !== undefined ? { yearFrom: params.from } : {}),
    ...(params.to !== undefined ? { yearTo: params.to } : {}),
    ...(params.rating !== undefined ? { maxRating: params.rating } : {}),
  };
}
