import type { BrowseRoot, DirectoryListing } from '@slouchcat/api-client';
import { infiniteQueryOptions, keepPreviousData, queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

/**
 * Queries of the folder browser (idea 02). The dialog opens on the roots and goes one level down
 * per request; a root id is stable while the server process runs, so it is part of the key.
 */
export const browseKey = ['fs', 'browse'] as const;

/** The directories the dialog may start from; the wizard has its own, shorter list. */
export const browseRootsQuery = queryOptions({
  queryKey: [...browseKey, 'roots'],
  queryFn: ({ signal }): Promise<BrowseRoot[]> => request(api.GET('/admin/fs/roots', { signal })),
  staleTime: 30_000,
});

/** The configured roots the setup wizard offers as suggestions; no account exists yet. */
export const setupRootsQuery = queryOptions({
  queryKey: [...browseKey, 'setupRoots'],
  queryFn: ({ signal }): Promise<BrowseRoot[]> => request(api.GET('/setup/roots', { signal })),
  staleTime: 30_000,
  retry: false,
});

/**
 * One directory below a root, paged. `keepPreviousData` holds the rows of the previous directory
 * on screen while the next one loads: an empty frame would take the focus out of the list, and a
 * folder with more than 200 entries loads the rest through `nextCursor`.
 */
export const directoryQuery = (rootId: string, rel: string, enabled: boolean) =>
  infiniteQueryOptions({
    queryKey: [...browseKey, 'dir', rootId, rel],
    queryFn: ({ pageParam, signal }): Promise<DirectoryListing> =>
      request(
        api.POST('/admin/fs/browse', {
          body: { rootId, rel, ...(pageParam ? { cursor: pageParam } : {}) },
          signal,
        }),
      ),
    initialPageParam: '',
    getNextPageParam: (last: DirectoryListing) => last.nextCursor ?? undefined,
    enabled: enabled && rootId !== '',
    staleTime: 30_000,
    placeholderData: keepPreviousData,
    retry: false,
  });
