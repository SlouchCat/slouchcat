import { paths } from '@/app/paths';
import type { ExternalMedia } from '../queries';

// The pages «Filme», «Serien» and «Suche» of the wishes area keep their view in the URL: a link
// from «Entdecken» preselects the sort or the genre, a reload or the back button returns to the
// same view. Kept apart from the pages: the router imports it for `validateSearch` without
// loading their chunks.

/** The sort orders of «Filme» and «Serien» (listWishTitles); «Beliebtheit» is the default. */
export const TITLE_SORTS = ['popularity', 'newest', 'rating'] as const;
export type TitleSort = (typeof TITLE_SORTS)[number];

export interface TitlesParams {
  sort?: Exclude<TitleSort, 'popularity'>;
  /** A TMDB genre id. */
  genre?: number;
  /** «Vorhandene ausblenden». */
  hide?: true;
}

export interface WishSearchParams {
  q?: string;
}

/** Keeps only a known sort, a genre id and the hide switch; anything else is dropped. */
export function validateTitlesParams(search: Record<string, unknown>): TitlesParams {
  const params: TitlesParams = {};
  if (search.sort === 'newest' || search.sort === 'rating') params.sort = search.sort;
  const genre = Number(search.genre);
  if (Number.isSafeInteger(genre) && genre > 0) params.genre = genre;
  if (search.hide === true || search.hide === 'true' || search.hide === 1) params.hide = true;
  return params;
}

/** Keeps the search text, at most 100 characters (searchWishTitles). */
export function validateWishSearchParams(search: Record<string, unknown>): WishSearchParams {
  // The router reads a hand-written `?q=1917` as a number: it is still the text "1917".
  const raw = typeof search.q === 'number' ? String(search.q) : search.q;
  const q = typeof raw === 'string' ? raw.slice(0, 100) : '';
  return q.trim() === '' ? {} : { q };
}

/** The grid of films or series with a view: `/discover/movies?sort=newest&genre=18`. */
export function titlesHref(media: ExternalMedia, params: TitlesParams): string {
  const query = new URLSearchParams();
  if (params.sort) query.set('sort', params.sort);
  if (params.genre) query.set('genre', String(params.genre));
  if (params.hide) query.set('hide', 'true');
  const base = media === 'movie' ? paths.discoverMovies : paths.discoverSeries;
  const text = query.toString();
  return text ? `${base}?${text}` : base;
}

/** «Suche» with a text: `/discover/search?q=…`; without one the empty page. */
export const searchHref = (q: string) =>
  q ? `${paths.discoverSearch}?q=${encodeURIComponent(q)}` : paths.discoverSearch;
