import type {
  AdminDevice,
  AdminPodcast,
  AdminItem,
  AdminStream,
  DuplicateGroup,
  Guest,
  Invitation,
  IssueKind,
  IssuePage,
  Library,
} from '@slouchcat/api-client';
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

/** Poll interval while a scan is queued or running. */
export const SCAN_POLL_MS = 5000;

export const isScanning = (library: Library) =>
  library.scan.state === 'queued' || library.scan.state === 'running';

export const librariesQuery = queryOptions({
  queryKey: ['admin', 'libraries'],
  queryFn: ({ signal }): Promise<Library[]> => request(api.GET('/admin/libraries', { signal })),
  refetchInterval: (query) => (query.state.data?.some(isScanning) ? SCAN_POLL_MS : false),
});

export const invitationsQuery = queryOptions({
  queryKey: ['admin', 'invitations'],
  queryFn: ({ signal }): Promise<Invitation[]> =>
    request(api.GET('/admin/invitations', { signal })),
});

export const guestsQuery = queryOptions({
  queryKey: ['admin', 'guests'],
  queryFn: ({ signal }): Promise<Guest[]> => request(api.GET('/admin/guests', { signal })),
});

/** Issue kinds of the maintenance page (B-9), in the order of its tabs. */
export const issueKinds = [
  'unmatched',
  'no_images',
  'unrated',
  'orphaned',
  'unplayable',
  'rating_mismatch',
] as const satisfies readonly IssueKind[];

/** Page size of the issue lists ("Mehr laden" fetches the next one). */
export const ISSUE_PAGE = 50;

const fetchIssues = (
  kind: IssueKind,
  limit: number,
  offset: number,
  signal: AbortSignal,
): Promise<IssuePage> =>
  request(
    api.GET('/admin/maintenance/issues', {
      params: { query: { kind, limit, offset } },
      signal,
    }),
  );

/** Everything under this key changes with a maintenance action (lists, counts, duplicates). */
export const maintenanceKey = ['admin', 'maintenance'] as const;

export const issuesQuery = (kind: IssueKind) =>
  infiniteQueryOptions({
    queryKey: [...maintenanceKey, 'issues', kind],
    queryFn: ({ pageParam, signal }) => fetchIssues(kind, ISSUE_PAGE, pageParam, signal),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((sum, page) => sum + page.items.length, 0);
      return loaded < last.total && last.items.length > 0 ? loaded : undefined;
    },
  });

/** The number of issues of a kind (one small request per tab label). */
export const issueCountQuery = (kind: IssueKind) =>
  queryOptions({
    queryKey: [...maintenanceKey, 'count', kind],
    queryFn: async ({ signal }) => (await fetchIssues(kind, 1, 0, signal)).total,
  });

export const duplicatesQuery = queryOptions({
  queryKey: [...maintenanceKey, 'duplicates'],
  queryFn: ({ signal }): Promise<DuplicateGroup[]> =>
    request(api.GET('/admin/maintenance/duplicates', { signal })),
});

export const adminItemQuery = (id: string) =>
  queryOptions({
    queryKey: ['admin', 'item', id],
    queryFn: ({ signal }): Promise<AdminItem> =>
      request(api.GET('/admin/items/{id}', { params: { path: { id } }, signal })),
  });

/** W-7: devices with owner, last contact and capability profile. */
export const devicesQuery = queryOptions({
  queryKey: ['admin', 'devices'],
  queryFn: ({ signal }): Promise<AdminDevice[]> => request(api.GET('/admin/devices', { signal })),
});

/** Poll interval of the stream overview (W-6). */
export const STREAMS_POLL_MS = 5000;

/** W-6: running playback sessions, asked again every 5 s while the page is open. */
export const streamsQuery = queryOptions({
  queryKey: ['admin', 'streams'],
  queryFn: ({ signal }): Promise<AdminStream[]> => request(api.GET('/admin/streams', { signal })),
  refetchInterval: STREAMS_POLL_MS,
});

/** Podcast subscriptions with their feed state (M6, H-2), newest first. */
export const podcastsQuery = queryOptions({
  queryKey: ['admin', 'podcasts'],
  queryFn: ({ signal }): Promise<AdminPodcast[]> => request(api.GET('/admin/podcasts', { signal })),
});
