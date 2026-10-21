import type { Rating, SearchRequest } from '@slouchcat/api-client';

// The search page keeps its state in the URL (design 4.1), so a result can be shared, reloaded
// and reached again with the browser's back button. Kept apart from the page: the router imports
// it for `validateSearch` without loading the page's chunk.

export const MEDIA_TYPES = ['movie', 'series', 'audiobook', 'podcast'] as const;
export const AVAILABILITY = ['local', 'download'] as const;
export const RATINGS = [0, 6, 12, 16, 18] as const satisfies readonly Rating[];

export type MediaType = (typeof MEDIA_TYPES)[number];
export type Availability = (typeof AVAILABILITY)[number];

/** Search state in the URL: `type` and `avail` are comma-separated lists. */
export interface SearchParams {
  q?: string;
  genre?: string;
  from?: number;
  to?: number;
  rating?: Rating;
  type?: string;
  avail?: string;
}

const year = (value: unknown): number | undefined => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isInteger(parsed) && parsed >= 1000 && parsed <= 9999 ? parsed : undefined;
};

const list = <T extends string>(value: unknown, allowed: readonly T[]): T[] =>
  typeof value === 'string' ? allowed.filter((entry) => value.split(',').includes(entry)) : [];

/** Keeps only valid search parameters; anything else is dropped (`validateSearch`). */
export function validateSearchParams(search: Record<string, unknown>): SearchParams {
  const params: SearchParams = {};
  if (typeof search.q === 'string' && search.q !== '') params.q = search.q.slice(0, 100);
  if (typeof search.genre === 'string' && search.genre !== '') {
    params.genre = search.genre.slice(0, 80);
  }
  const from = year(search.from);
  if (from !== undefined) params.from = from;
  const to = year(search.to);
  if (to !== undefined) params.to = to;
  const rating = Number(search.rating);
  if (search.rating !== undefined && (RATINGS as readonly number[]).includes(rating)) {
    params.rating = rating as Rating;
  }
  const types = list(search.type, MEDIA_TYPES);
  if (types.length > 0) params.type = types.join(',');
  const avail = list(search.avail, AVAILABILITY);
  if (avail.length > 0) params.avail = avail.join(',');
  return params;
}

export const mediaTypesOf = (params: SearchParams): MediaType[] => list(params.type, MEDIA_TYPES);
export const availabilityOf = (params: SearchParams): Availability[] =>
  list(params.avail, AVAILABILITY);

/** Whether any filter is set (a search without text browses by the filters). */
export const hasFilter = (params: SearchParams): boolean =>
  params.genre !== undefined ||
  params.from !== undefined ||
  params.to !== undefined ||
  params.rating !== undefined ||
  params.type !== undefined ||
  params.avail !== undefined;

/** The request body for the URL state, without paging; null when there is nothing to ask. */
export function searchRequest(
  params: SearchParams,
): Omit<SearchRequest, 'offset' | 'limit'> | null {
  const query = (params.q ?? '').trim();
  // One character is too short (the server answers 422); no text and no filter asks nothing.
  if (query.length === 1 || (query.length === 0 && !hasFilter(params))) return null;
  const types = mediaTypesOf(params);
  const availability = availabilityOf(params);
  return {
    ...(query ? { query } : {}),
    ...(params.genre !== undefined ? { genre: params.genre } : {}),
    ...(params.from !== undefined ? { yearFrom: params.from } : {}),
    ...(params.to !== undefined ? { yearTo: params.to } : {}),
    ...(params.rating !== undefined ? { maxRating: params.rating } : {}),
    ...(types.length > 0 ? { mediaTypes: types } : {}),
    ...(availability.length > 0 ? { availability } : {}),
  };
}
