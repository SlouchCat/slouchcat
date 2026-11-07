import type { components } from '@slouchcat/api-client';

// The radio page keeps its filter and sort in the URL (concept 16, 10.1: `?country=CH&genre=pop`),
// so Back and a shared link work. Several values of a filter are a comma-separated list. Kept apart
// from the page: the router imports it for `validateSearch` without loading the page's chunk.

export type RadioSort = components['schemas']['RadioSort'];
export type RadioQuality = components['schemas']['RadioQuality'];
export type RadioFilter = components['schemas']['RadioFilter'];

/** The facets with values to choose, in the order of the filter row. */
export const RADIO_FACETS = [
  'country',
  'region',
  'language',
  'genre',
  'kind',
  'mood',
  'quality',
] as const;
export type RadioFacetKey = (typeof RADIO_FACETS)[number];

export const RADIO_SORTS = ['popular', 'name', 'quality', 'new'] as const satisfies RadioSort[];

/** The radio page's state in the URL. Absent = not set; `sort` absent = `popular`. */
export interface RadioSearch {
  q?: string;
  country?: string;
  region?: string;
  language?: string;
  genre?: string;
  kind?: string;
  mood?: string;
  quality?: string;
  /** Only stations that show titles. */
  titles?: true;
  /** Only the profile's favourites («Meine Sender»). */
  fav?: true;
  /** «Beliebt»: the most popular stations under the other filters. */
  top?: true;
  sort?: RadioSort;
}

/** How many stations «Beliebt» shows (the mockup's top list). */
export const TOP_COUNT = 15;
/** More values of one filter are not a filter any more. */
const MAX_VALUES = 30;

/** What a value of each facet may look like; anything else is dropped. */
const valuePattern: Record<RadioFacetKey, RegExp> = {
  country: /^[A-Z]{2}$/,
  region: /^[A-Z]{2}-[A-Z0-9]{1,3}$/,
  language: /^[a-z]{2,3}$/,
  genre: /^[a-z0-9][a-z0-9_-]{0,39}$/,
  kind: /^[a-z0-9][a-z0-9_-]{0,39}$/,
  mood: /^[a-z0-9][a-z0-9_-]{0,39}$/,
  quality: /^(high|good|basic)$/,
};

const normalise: Record<RadioFacetKey, (value: string) => string> = {
  country: (value) => value.toUpperCase(),
  region: (value) => value.toUpperCase(),
  language: (value) => value.toLowerCase(),
  genre: (value) => value.toLowerCase(),
  kind: (value) => value.toLowerCase(),
  mood: (value) => value.toLowerCase(),
  quality: (value) => value.toLowerCase(),
};

const isTrue = (value: unknown) =>
  value === true || value === 'true' || value === 1 || value === '1';

/** The valid values of a facet in a URL value, each once, in their order. */
function facetList(facet: RadioFacetKey, value: unknown): string[] {
  if (typeof value !== 'string') return [];
  const values = value
    .split(',')
    .map((entry) => normalise[facet](entry.trim()))
    .filter((entry) => valuePattern[facet].test(entry));
  return [...new Set(values)].slice(0, MAX_VALUES);
}

/** Keeps only valid parameters (`validateSearch` of `/radio`); anything else is dropped. */
export function validateRadioSearch(search: Record<string, unknown>): RadioSearch {
  const params: RadioSearch = {};
  // The router reads a typed `?q=1291` as a number: it is still the text.
  const q = typeof search.q === 'number' ? String(search.q) : search.q;
  if (typeof q === 'string' && q.trim() !== '') params.q = q.slice(0, 100);
  for (const facet of RADIO_FACETS) {
    const values = facetList(facet, search[facet]);
    if (values.length > 0) params[facet] = values.join(',');
  }
  // A region belongs to a chosen country (10.1): without one it is dropped.
  if (!params.country) delete params.region;
  if (isTrue(search.titles)) params.titles = true;
  if (isTrue(search.fav)) params.fav = true;
  if (isTrue(search.top)) params.top = true;
  if ((RADIO_SORTS as readonly unknown[]).includes(search.sort) && search.sort !== 'popular') {
    params.sort = search.sort as RadioSort;
  }
  return params;
}

/** The chosen values of a facet. */
export const valuesOf = (search: RadioSearch, facet: RadioFacetKey): string[] =>
  facetList(facet, search[facet]);

/** The search with other values for one facet; no values removes the filter. */
export function withValues(
  search: RadioSearch,
  facet: RadioFacetKey,
  values: readonly string[],
): RadioSearch {
  const next: RadioSearch = { ...search };
  const valid = facetList(facet, values.join(','));
  if (valid.length > 0) next[facet] = valid.join(',');
  else delete next[facet];
  if (!next.country) delete next.region;
  return next;
}

/** Whether anything narrows the list (the browse rows show only without). */
export const hasRadioFilter = (search: RadioSearch): boolean =>
  search.q !== undefined ||
  RADIO_FACETS.some((facet) => search[facet] !== undefined) ||
  search.titles === true ||
  search.fav === true ||
  search.top === true;

/** The filter without the text search and the quick filters' own flags: the server's object. */
export function toRadioFilter(search: RadioSearch): RadioFilter {
  const filter: RadioFilter = {};
  if (search.q) filter.q = search.q;
  for (const facet of RADIO_FACETS) {
    const values = valuesOf(search, facet);
    if (values.length === 0) continue;
    if (facet === 'quality') filter.quality = values as RadioQuality[];
    else filter[facet] = values;
  }
  if (search.titles) filter.titles = true;
  if (search.fav) filter.favorites = true;
  return filter;
}

/** The URL state for a filter the server stored (`lastFilter`, `lastSort`). */
export function fromRadioFilter(filter: RadioFilter | undefined, sort?: RadioSort): RadioSearch {
  const search: Record<string, unknown> = { q: filter?.q, sort };
  for (const facet of RADIO_FACETS) {
    const values = filter?.[facet];
    if (values && values.length > 0) search[facet] = values.join(',');
  }
  if (filter?.titles) search.titles = true;
  if (filter?.favorites) search.fav = true;
  return validateRadioSearch(search);
}

/** The query parameters of `listRadioStations` and `getRadioFacets` for the URL state. */
export function radioQueryParams(search: RadioSearch) {
  const filter = toRadioFilter(search);
  return {
    ...filter,
    sort: search.top ? ('popular' as const) : (search.sort ?? ('popular' as const)),
  };
}

export type RadioQueryParams = ReturnType<typeof radioQueryParams>;
