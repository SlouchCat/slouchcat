import type {
  AdminAccount,
  AdminCollection,
  AdminSettings,
  AdminSettingsUpdate,
  AdminDevice,
  AdminItemHit,
  AdminPick,
  AdminPodcast,
  AdminItem,
  AdminStream,
  Announcement,
  Backup,
  CollectionRequest,
  Dashboard,
  DuplicateGroup,
  Guest,
  HomeRowSetting,
  Invitation,
  IssueKind,
  IssuePage,
  JobSummary,
  Library,
  PickRequest,
  PickUpdate,
  SystemInfo,
  Stats,
  StatsDays,
  TraktStatus,
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

// Curation (M7): Cat's recommendations, collections, start page rows and the Trakt lists.

/** Cat's recommendations in carousel order (E-2), global ones and those of single profiles. */
export const picksQuery = queryOptions({
  queryKey: ['admin', 'picks'],
  queryFn: ({ signal }): Promise<AdminPick[]> => request(api.GET('/admin/picks', { signal })),
});

/** Shortest search text the server takes (`q` minLength). */
export const MIN_SEARCH = 2;

/** Titles by name for the pick and collection dialogs (unfiltered, no images). */
export const adminItemSearchQuery = (text: string) =>
  queryOptions({
    queryKey: ['admin', 'items', 'search', text],
    queryFn: ({ signal }): Promise<AdminItemHit[]> =>
      request(
        api.GET('/admin/items/search', { params: { query: { q: text, limit: 20 } }, signal }),
      ),
    enabled: text.length >= MIN_SEARCH,
    staleTime: 30_000,
  });

export const collectionsQuery = queryOptions({
  queryKey: ['admin', 'collections'],
  queryFn: ({ signal }): Promise<AdminCollection[]> =>
    request(api.GET('/admin/collections', { signal })),
});

/** One collection with its members in order. */
export const adminCollectionQuery = (id: string) =>
  queryOptions({
    queryKey: ['admin', 'collections', id],
    queryFn: ({ signal }): Promise<AdminCollection> =>
      request(api.GET('/admin/collections/{id}', { params: { path: { id } }, signal })),
  });

/** The eight start page rows in display order, with their hidden flag (E-9). */
export const homeRowsQuery = queryOptions({
  queryKey: ['admin', 'home-rows'],
  queryFn: ({ signal }): Promise<HomeRowSetting[]> =>
    request(api.GET('/admin/home-rows', { signal })),
});

/** State of the daily Trakt lists (E-7, I-7). */
export const traktStatusQuery = queryOptions({
  queryKey: ['admin', 'trakt'],
  queryFn: ({ signal }): Promise<TraktStatus> => request(api.GET('/admin/trakt', { signal })),
});

// Administration (M8): accounts, backups, statistics and announcements.

/** The main account first, then the guests, with profiles and channels (A-2). */
export const accountsQuery = queryOptions({
  queryKey: ['admin', 'accounts'],
  queryFn: ({ signal }): Promise<AdminAccount[]> => request(api.GET('/admin/accounts', { signal })),
});

/** Manual and automatic backups in the backup directory, newest first (A-4). */
export const backupsQuery = queryOptions({
  queryKey: ['admin', 'backups'],
  queryFn: ({ signal }): Promise<Backup[]> => request(api.GET('/admin/backups', { signal })),
});

/** Usage over the last `days` days (A-8). */
export const statsQuery = (days: StatsDays) =>
  queryOptions({
    queryKey: ['admin', 'stats', days],
    queryFn: ({ signal }): Promise<Stats> =>
      request(api.GET('/admin/stats', { params: { query: { days } }, signal })),
  });

/** Every announcement of the main user, newest first (A-7). */
export const adminAnnouncementsQuery = queryOptions({
  queryKey: ['admin', 'announcements'],
  queryFn: ({ signal }): Promise<Announcement[]> =>
    request(api.GET('/admin/announcements', { signal })),
});

export const createPicks = (body: PickRequest): Promise<AdminPick[]> =>
  request(api.POST('/admin/picks', { body }));

export const updatePick = (id: number, body: PickUpdate): Promise<AdminPick> =>
  request(api.PATCH('/admin/picks/{id}', { params: { path: { id } }, body }));

export const deletePick = (id: number) =>
  request(api.DELETE('/admin/picks/{id}', { params: { path: { id } } }));

/** Every pick id exactly once, in the new order. */
export const reorderPicks = (ids: number[]) =>
  request(api.PUT('/admin/picks/order', { body: { ids } }));

export const createCollection = (body: CollectionRequest): Promise<AdminCollection> =>
  request(api.POST('/admin/collections', { body }));

export const renameCollection = (id: string, body: CollectionRequest): Promise<AdminCollection> =>
  request(api.PATCH('/admin/collections/{id}', { params: { path: { id } }, body }));

export const deleteCollection = (id: string) =>
  request(api.DELETE('/admin/collections/{id}', { params: { path: { id } } }));

/** Replaces the members of a manual collection, in this order. */
export const setCollectionItems = (id: string, itemIds: string[]): Promise<AdminCollection> =>
  request(
    api.PUT('/admin/collections/{id}/items', { params: { path: { id } }, body: { itemIds } }),
  );

export const setHomeRows = (rows: HomeRowSetting[]): Promise<HomeRowSetting[]> =>
  request(api.PUT('/admin/home-rows', { body: rows }));

export const refreshTrakt = () => request(api.POST('/admin/trakt/refresh'));

// Administration (M8): dashboard, system state, settings and integrations.

/** Poll interval of the dashboard (A-3: a new error shows within 10 s). */
export const DASHBOARD_POLL_MS = 5000;

/** Streams, jobs, storage, integrations, errors and update state in one request (A-3). */
export const dashboardQuery = queryOptions({
  queryKey: ['admin', 'dashboard'],
  queryFn: ({ signal }): Promise<Dashboard> => request(api.GET('/admin/dashboard', { signal })),
  refetchInterval: DASHBOARD_POLL_MS,
});

/** Version, transcode hardware, inotify limit and file systems; they change with a restart only. */
export const systemQuery = queryOptions({
  queryKey: ['admin', 'system'],
  queryFn: ({ signal }): Promise<SystemInfo> => request(api.GET('/admin/system', { signal })),
  staleTime: 60_000,
});

export const retryJob = (id: number): Promise<JobSummary> =>
  request(api.POST('/admin/jobs/{id}/retry', { params: { path: { id } } }));

export const cancelJob = (id: number) =>
  request(api.DELETE('/admin/jobs/{id}', { params: { path: { id } } }));

/** Keys, e-mail server, Telegram bot, Web Push, update check and public URL (A-2). */
export const settingsQuery = queryOptions({
  queryKey: ['admin', 'settings'],
  queryFn: ({ signal }): Promise<AdminSettings> => request(api.GET('/admin/settings', { signal })),
});

export const updateSettings = (body: AdminSettingsUpdate): Promise<AdminSettings> =>
  request(api.PUT('/admin/settings', { body }));

/** A test mail to the main account's address through the configured e-mail server. */
export const sendTestEmail = () => request(api.POST('/admin/settings/email/test'));

/** A new VAPID key pair; every push subscription goes with the old one. */
export const rotatePushKeys = (): Promise<AdminSettings> =>
  request(api.POST('/admin/settings/push/rotate'));
