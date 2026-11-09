import type { components, ItemRef } from '@slouchcat/api-client';
import { api, request } from '@/api/client';
import {
  PAGED_LIST_PAGE,
  pagedListQuery,
  usePagedList,
  type PagedList,
  type PagedSource,
  type PlaceRange,
} from '@/lib/pagedList';
import type { LibraryRequest } from './libraryParams';

// The library grid's pages with gaps (concept 17 section 6, D-6) on the general page model
// (lib/pagedList, concept 18 C-3): page 0 also carries the facets and the letter starts.

type LibraryItemsPage = components['schemas']['LibraryItemsPage'];

/** Titles per page of the library grid: the contract's maximum. */
export const LIBRARY_GRID_PAGE = PAGED_LIST_PAGE;

/** A library view as a list operation. The prefix `['library']` keeps the invalidations of the app. */
export const librarySource = (
  id: number,
  query: LibraryRequest,
): PagedSource<LibraryItemsPage, ItemRef> => ({
  key: ['library', id, query],
  fetch: (offset, limit, signal) =>
    request(
      api.GET('/libraries/{id}/items', {
        params: { path: { id }, query: { ...query, offset, limit } },
        signal,
      }),
    ),
  items: (page) => page.items,
  total: (page) => page.total,
  id: (item) => item.id,
});

/** Page `page` of a library view. */
export const libraryPageQuery = (id: number, query: LibraryRequest, page: number) =>
  pagedListQuery(librarySource(id, query), page);

/** The places of the list the grid draws: `start` to `end`, both included (title indices). */
export type DrawnRange = PlaceRange;

export type LibraryPages = PagedList<LibraryItemsPage, ItemRef>;

/** The titles of a library view as places with gaps (usePagedList). */
export function useLibraryPages(
  id: number | null,
  query: LibraryRequest,
  range: DrawnRange,
): LibraryPages {
  return usePagedList(librarySource(id ?? 0, query), range, { enabled: id !== null });
}
