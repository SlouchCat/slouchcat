// The page «Sammlungen» keeps its search, size and order in the URL, like the page «Gesehen»
// (concept 21 P-10): the browser's back button returns to the same view. Kept apart from the
// page: the router imports it for `validateSearch` without loading the page's chunk.

export const COLLECTION_SIZES = ['all', '2', '3', '5'] as const;
export const COLLECTION_SORTS = ['name', 'count', 'started'] as const;

export type CollectionSize = (typeof COLLECTION_SIZES)[number];
export type CollectionSort = (typeof COLLECTION_SORTS)[number];

/** State of the page in the URL; without `min` every collection, without `sort` by name. */
export interface CollectionsParams {
  q?: string;
  /** A number in the URL (`?min=2`); the router would quote a text that looks like one. */
  min?: 2 | 3 | 5;
  sort?: Exclude<CollectionSort, 'name'>;
}

/** Keeps only valid parameters; anything else is dropped (`validateSearch`). */
export function validateCollectionsParams(search: Record<string, unknown>): CollectionsParams {
  const params: CollectionsParams = {};
  // The router reads a typed `?q=1291` as a number: it is still the text.
  const text = (value: unknown) => (typeof value === 'number' ? String(value) : value);
  const q = text(search.q);
  if (typeof q === 'string' && q.trim() !== '') params.q = q.slice(0, 100);
  const min = Number(search.min);
  if (min === 2 || min === 3 || min === 5) params.min = min;
  if (search.sort === 'count' || search.sort === 'started') params.sort = search.sort;
  return params;
}
