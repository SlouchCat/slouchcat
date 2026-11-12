import type {
  AdminAccount,
  AdminCollection,
  AdminSettings,
  AdminSettingsUpdate,
  AdminDevice,
  AdminItemHit,
  AdminUserPick,
  AdminPodcast,
  AdminPodcastEpisodePage,
  AdminItem,
  AdminStream,
  Announcement,
  Backup,
  CollectionRequest,
  Dashboard,
  DuplicateGroup,
  HomeRowSetting,
  Integration,
  IntegrationCreated,
  IntegrationEvent,
  IntegrationRequest,
  Invitation,
  Issue,
  IssueKind,
  IssuePage,
  JobCancelResult,
  JobDismissResult,
  JobRetryFailedResult,
  JobOverview,
  JobRun,
  JobRunPage,
  JobScheduleUpdate,
  JobTypeWindowUpdate,
  JobSummary,
  RelinkPairList,
  JobTypeInfo,
  Library,
  LibraryMetadata,
  LibraryMetadataUpdate,
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
  ApplyMetadataRequest,
  ApplyMetadataResponse,
  PathMapping,
  PicksSettings,
  PodcastDownloadPage,
  SystemInfo,
  Stats,
  StatsDays,
  TraktStatus,
  components,
} from '@slouchcat/api-client';
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query';
import { api, isApiError, request } from '@/api/client';
import type { PagedSource } from '@/lib/pagedList';

/** Poll interval while a scan is queued or running. */
export const SCAN_POLL_MS = 5000;

export const isScanning = (library: Library) =>
  library.scan.state === 'queued' || library.scan.state === 'running';

/**
 * Whether "Scan again" does anything: a library without a folder has nothing to walk (idea 02,
 * E13), and the server queues no job for it either. A podcast library has never had a folder;
 * its button fetches the subscriptions instead (M6).
 */
export const canScan = (library: Library) =>
  library.kind === 'podcasts' || library.roots.length > 0;

export const librariesQuery = queryOptions({
  queryKey: ['admin', 'libraries'],
  queryFn: ({ signal }): Promise<Library[]> => request(api.GET('/admin/libraries', { signal })),
  refetchInterval: (query) => (query.state.data?.some(isScanning) ? SCAN_POLL_MS : false),
});

/**
 * The four metadata providers as the server describes them (ADR 0015): whether a key is present,
 * what each one may do and for which folder contents. The folder forms build their checkbox
 * lists from this, never from a table of their own.
 */
export const metadataProvidersQuery = queryOptions({
  queryKey: ['admin', 'metadata', 'providers'],
  queryFn: ({ signal }): Promise<MetadataProviderInfo[]> =>
    request(api.GET('/admin/metadata/providers', { signal })),
  staleTime: 60_000,
});

export const invitationsQuery = queryOptions({
  queryKey: ['admin', 'invitations'],
  queryFn: ({ signal }): Promise<Invitation[]> =>
    request(api.GET('/admin/invitations', { signal })),
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

/** The issues of a kind, read page by page as the tab scrolls (concept 18 C-3). */
export const issuesSource = (kind: IssueKind): PagedSource<IssuePage, Issue> => ({
  key: [...maintenanceKey, 'issues', kind],
  fetch: (offset, limit, signal) => fetchIssues(kind, limit, offset, signal),
  items: (page) => page.items,
  total: (page) => page.total,
  id: (issue) => `${issue.itemId}:${issue.mediaFileId ?? ''}`,
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

/** Deleted titles with a live copy and the user data left on them (ADR 0023). */
export const relinkQuery = queryOptions({
  queryKey: [...maintenanceKey, 'relink'],
  queryFn: ({ signal }): Promise<RelinkPairList> =>
    request(api.GET('/admin/maintenance/relink', { signal })),
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

/**
 * One podcast for the page «Podcast» (concept 19 part A, 12.2; contract 1.29.0). Below the key of
 * the list, so every invalidation of the list reaches it too.
 */
export const podcastQuery = (id: string) =>
  queryOptions({
    queryKey: ['admin', 'podcasts', id],
    queryFn: ({ signal }): Promise<AdminPodcast> =>
      request(api.GET('/admin/podcasts/{id}', { params: { path: { id } }, signal })),
  });

/** The view of the episode list of the page «Podcast»: all, with a file, without one. */
export type PodcastEpisodeFilter = 'all' | 'with_file' | 'without_file';

/** Episodes asked for per page of the page «Podcast». */
export const PODCAST_EPISODE_PAGE = 50;

/** One page of the episodes of a podcast in a view, from `offset`. */
export const podcastEpisodePage = (
  id: string,
  filter: PodcastEpisodeFilter,
  offset: number,
  signal?: AbortSignal,
): Promise<AdminPodcastEpisodePage> =>
  request(
    api.GET('/admin/podcasts/{id}/episodes', {
      params: { path: { id }, query: { filter, offset, limit: PODCAST_EPISODE_PAGE } },
      signal,
    }),
  );

/** The episodes of a podcast with the state of their file, page by page (12.2). */
export const podcastEpisodesQuery = (id: string, filter: PodcastEpisodeFilter) =>
  infiniteQueryOptions({
    queryKey: ['admin', 'podcasts', id, 'episodes', filter],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) => podcastEpisodePage(id, filter, pageParam, signal),
    getNextPageParam: (last) =>
      last.offset + last.episodes.length < last.total
        ? last.offset + last.episodes.length
        : undefined,
  });

/** The job type of a download of a podcast episode (the pause of the section «Downloads»). */
export const PODCAST_DOWNLOAD_JOB = 'podcast.download';

/** The queue of the section «Downloads» (12.3): what runs and waits in every podcast library. */
export const podcastDownloadsQuery = queryOptions({
  queryKey: ['admin', 'podcasts', 'downloads'],
  queryFn: ({ signal }): Promise<PodcastDownloadPage> =>
    request(api.GET('/admin/podcasts/downloads', { params: { query: { limit: 100 } }, signal })),
});

// Curation (M7): the user picks, collections, start page rows and the Trakt lists.

/**
 * The user picks (concept 18 part B, ADR 0028): one row per title, newest pick first, with the
 * number of picks and the days left, never a profile.
 */
export const picksQuery = queryOptions({
  queryKey: ['admin', 'picks'],
  queryFn: ({ signal }): Promise<AdminUserPick[]> => request(api.GET('/admin/picks', { signal })),
});

/** How many days a title stays after its newest pick (7 to 365). */
export const picksSettingsQuery = queryOptions({
  queryKey: ['admin', 'picks', 'settings'],
  queryFn: ({ signal }): Promise<PicksSettings> =>
    request(api.GET('/admin/picks/settings', { signal })),
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

// The spotlight list of the start page (M11, ADR 0011).

export type SpotlightConfig = components['schemas']['SpotlightConfig'];
export type SpotlightConfigUpdate = components['schemas']['SpotlightConfigUpdate'];
export type SpotlightEntries = components['schemas']['SpotlightEntries'];
export type SpotlightEntry = components['schemas']['SpotlightEntry'];
export type SpotlightEntryOverride = components['schemas']['SpotlightEntryOverride'];

/** The chosen list, its numbers and the catalogue of list kinds (`listKinds`). */
export const spotlightConfigQuery = queryOptions({
  queryKey: ['admin', 'spotlight', 'config'],
  queryFn: ({ signal }): Promise<SpotlightConfig> =>
    request(api.GET('/admin/spotlight', { signal })),
});

/** The fetched entries in list order with library hit and override, and the orphaned overrides. */
export const spotlightEntriesQuery = queryOptions({
  queryKey: ['admin', 'spotlight', 'entries'],
  queryFn: ({ signal }): Promise<SpotlightEntries> =>
    request(api.GET('/admin/spotlight/entries', { signal })),
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

/** Removes every pick of a title. */
export const removeUserPick = (itemId: string) =>
  request(api.DELETE('/admin/picks/{itemId}', { params: { path: { itemId } } }));

export const updatePicksSettings = (body: PicksSettings): Promise<PicksSettings> =>
  request(api.PUT('/admin/picks/settings', { body }));

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

export const setSpotlightConfig = (body: SpotlightConfigUpdate): Promise<SpotlightConfig> =>
  request(api.PUT('/admin/spotlight', { body }));

/** Hides, pins or annotates one entry; `{}` removes its override (an orphan's "Entfernen"). */
export const setSpotlightEntryOverride = (
  media: SpotlightEntry['media'],
  tmdbId: number,
  body: SpotlightEntryOverride,
): Promise<SpotlightEntry> =>
  request(
    api.PATCH('/admin/spotlight/entries/{media}/{tmdbId}', {
      params: { path: { media, tmdbId } },
      body,
    }),
  );

/** Queues the spotlight fetch out of schedule ("Jetzt abrufen"); at most once a minute (429). */
export const refreshSpotlight = () => request(api.POST('/admin/spotlight/refresh'));

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

// Background jobs (concept 09, ADR 0017): the page /admin/jobs polls the overview like the
// dashboard; the runs of one type are asked while its row is open on the page.
export const JOBS_POLL_MS = 5000;

export const jobTypesQuery = queryOptions({
  queryKey: ['admin', 'job-types'],
  queryFn: ({ signal }): Promise<JobOverview> => request(api.GET('/admin/job-types', { signal })),
  refetchInterval: JOBS_POLL_MS,
});

export type JobRunState = 'queued' | 'planned' | 'running' | 'failed' | 'done' | 'cancelled';

export const jobRunsQuery = (type: string, state: JobRunState) =>
  queryOptions({
    queryKey: [...jobRunsKey, type, state],
    queryFn: ({ signal }): Promise<JobRunPage> =>
      request(api.GET('/admin/jobs', { params: { query: { type, state, limit: 200 } }, signal })),
    refetchInterval: JOBS_POLL_MS,
  });

/** Every query of the runs of a type and of the failed runs of every type (concept 20, P-6). */
export const jobRunsKey = ['admin', 'job-runs'] as const;

/** The failed runs of every type, newest first, read page by page as the tab scrolls (P-6). */
export const failedRunsSource: PagedSource<JobRunPage, JobRun> = {
  key: [...jobRunsKey, 'failed-all'],
  fetch: (offset, limit, signal) =>
    request(
      api.GET('/admin/jobs', { params: { query: { state: 'failed', offset, limit } }, signal }),
    ),
  items: (page) => page.runs,
  total: (page) => page.total,
  id: (job) => job.id,
};

/** Queues the failed runs of a type, or of every type, again in one request (P-6). */
export const retryFailedJobs = (type?: string): Promise<JobRetryFailedResult> =>
  request(api.POST('/admin/jobs/failed/retry', { params: { query: type ? { type } : {} } }));

/** Dismisses the failed runs of a type, or of every type, in one request (P-6). */
export const dismissFailedJobs = (type?: string): Promise<JobDismissResult> =>
  request(api.DELETE('/admin/jobs/failed', { params: { query: type ? { type } : {} } }));

export const runJobType = (type: string): Promise<JobRun[]> =>
  request(api.POST('/admin/job-types/{type}/run', { params: { path: { type } } }));

export const pauseJobType = (type: string): Promise<JobTypeInfo> =>
  request(api.POST('/admin/job-types/{type}/pause', { params: { path: { type } } }));

export const resumeJobType = (type: string): Promise<JobTypeInfo> =>
  request(api.POST('/admin/job-types/{type}/resume', { params: { path: { type } } }));

export const updateJobSchedule = (type: string, body: JobScheduleUpdate): Promise<JobTypeInfo> =>
  request(api.PATCH('/admin/job-types/{type}/schedule', { params: { path: { type } }, body }));

/** The time window of a windowable event type (concept 15, 4.3; contract 1.14.0). */
export const updateJobTypeWindow = (
  type: string,
  body: JobTypeWindowUpdate,
): Promise<JobTypeInfo> =>
  request(api.PUT('/admin/job-types/{type}/window', { params: { path: { type } }, body }));

export const cancelJobRuns = (type: string, scope: 'queued' | 'all'): Promise<JobCancelResult> =>
  request(
    api.DELETE('/admin/job-types/{type}/runs', { params: { path: { type }, query: { scope } } }),
  );

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

/** Webhook integrations of Radarr and Sonarr (M9, I-8). */
export const integrationsQuery = queryOptions({
  queryKey: ['admin', 'integrations'],
  queryFn: ({ signal }): Promise<Integration[]> =>
    request(api.GET('/admin/integrations', { signal })),
});

/** The event log of one integration: the newest 20 reports. */
export const integrationEventsQuery = (id: number) =>
  queryOptions({
    queryKey: ['admin', 'integrations', id, 'events'],
    queryFn: ({ signal }): Promise<IntegrationEvent[]> =>
      request(api.GET('/admin/integrations/{id}/events', { params: { path: { id } }, signal })),
  });

/** Creates an integration; the answer carries the address, which is never shown again. */
export const createIntegration = (body: IntegrationRequest): Promise<IntegrationCreated> =>
  request(api.POST('/admin/integrations', { body }));

/** A new token: the old address stops working, the new one appears once. */
export const renewIntegrationToken = (id: number): Promise<IntegrationCreated> =>
  request(api.POST('/admin/integrations/{id}/token', { params: { path: { id } } }));

/** Replaces the prefix pairs of an integration. */
export const setIntegrationPathMap = (id: number, pathMap: PathMapping[]): Promise<Integration> =>
  request(
    api.PUT('/admin/integrations/{id}/path-map', { params: { path: { id } }, body: { pathMap } }),
  );

/** Removes an integration and its event log; its address stops working at once. */
export const deleteIntegration = (id: number) =>
  request(api.DELETE('/admin/integrations/{id}', { params: { path: { id } } }));

// Concept 10 (ADR 0018): the metadata settings, a library's deviation and the apply run.
export const metadataSettingsQuery = queryOptions({
  queryKey: ['admin', 'metadata-settings'],
  queryFn: ({ signal }): Promise<MetadataSettings> =>
    request(api.GET('/admin/settings/metadata', { signal })),
});

export const updateMetadataSettings = (body: MetadataSettingsUpdate): Promise<MetadataSettings> =>
  request(api.PUT('/admin/settings/metadata', { body }));

export const libraryMetadataQuery = (id: number) =>
  queryOptions({
    queryKey: ['admin', 'library-metadata', id],
    queryFn: ({ signal }): Promise<LibraryMetadata> =>
      request(api.GET('/admin/libraries/{id}/metadata', { params: { path: { id } }, signal })),
  });

export const updateLibraryMetadata = (
  id: number,
  body: LibraryMetadataUpdate,
): Promise<LibraryMetadata> =>
  request(api.PUT('/admin/libraries/{id}/metadata', { params: { path: { id } }, body }));

export const applyMetadata = (body: ApplyMetadataRequest): Promise<ApplyMetadataResponse> =>
  request(api.POST('/admin/metadata/apply', { body }));

// Concept 19 D3: the book sources, the comparison of the editor and the page «Hörbücher: Quellen».
export type AudibleRegion = components['schemas']['AudibleRegion'];
export type BookMatchField = components['schemas']['BookMatchField'];
export type MatchPreview = components['schemas']['MatchPreview'];
export type AudiobookSources = components['schemas']['AudiobookSources'];
export type AudiobookSourcesUpdate = components['schemas']['AudiobookSourcesUpdate'];
export type AudiobookAutoMatch = components['schemas']['AudiobookAutoMatch'];
export type BookReviewEntry = components['schemas']['BookReviewEntry'];

/** The book sources in the order of the contract (`MetadataProvider`). */
export const BOOK_PROVIDERS = [
  'audible',
  'dnb',
  'applebooks',
  'openlibrary',
  'googlebooks',
] as const;
export type BookProvider = (typeof BOOK_PROVIDERS)[number];
export const isBookProvider = (provider: string): provider is BookProvider =>
  (BOOK_PROVIDERS as readonly string[]).includes(provider);

/** Poll interval of the page «Hörbücher: Quellen» while a match of audiobooks runs. */
export const AUDIOBOOK_MATCH_POLL_MS = 5000;

export const audiobookSourcesQuery = queryOptions({
  queryKey: ['admin', 'audiobook-sources'],
  queryFn: ({ signal }): Promise<AudiobookSources> =>
    request(api.GET('/admin/metadata/audiobooks', { signal })),
  refetchInterval: (query) => (query.state.data?.running ? AUDIOBOOK_MATCH_POLL_MS : false),
});

export const updateAudiobookSources = (body: AudiobookSourcesUpdate): Promise<AudiobookSources> =>
  request(api.PATCH('/admin/metadata/audiobooks', { body }));

export const matchAllAudiobooks = (): Promise<{ queued: number }> =>
  request(api.POST('/admin/metadata/audiobooks/match-all'));

export const audiobookReviewQuery = queryOptions({
  queryKey: ['admin', 'audiobook-sources', 'review'],
  queryFn: ({ signal }): Promise<BookReviewEntry[]> =>
    request(api.GET('/admin/metadata/audiobooks/review', { signal })),
});

/** The comparison of an entry of a book source with an audiobook (`getMatchPreview`). */
export const matchPreviewQuery = (
  id: string,
  provider: BookProvider,
  providerId: string,
  region?: AudibleRegion,
) =>
  queryOptions({
    queryKey: ['admin', 'item', id, 'match-preview', provider, providerId, region ?? null],
    queryFn: ({ signal }): Promise<MatchPreview> =>
      request(
        api.GET('/admin/items/{id}/match/preview', {
          params: { path: { id }, query: { provider, providerId, region } },
          signal,
        }),
      ),
    retry: false,
  });

// Webradio (concept 16, stage 1): the administration's radio page. Its queries share the key
// `['admin', 'radio']`, so a change invalidates them together.
export type AdminRadioStation = components['schemas']['AdminRadioStation'];
type AdminRadioStationPage = components['schemas']['AdminRadioStationPage'];
export type AdminRadioStationPatch = components['schemas']['AdminRadioStationPatch'];
export type RadioConfig = components['schemas']['RadioConfig'];
export type RadioConfigUpdate = components['schemas']['RadioConfigUpdate'];
export type RadioCatalogState = components['schemas']['RadioCatalogState'];
export type RadioRelays = components['schemas']['RadioRelays'];
export type RadioProbe = components['schemas']['RadioProbe'];
export type AdminRadioStationCreate = components['schemas']['AdminRadioStationCreate'];

/** The relays and their listeners are asked again every 5 s while «Übersicht» is open. */
export const RADIO_RELAYS_POLL_MS = 5000;

export const radioAdminKeys = { all: ['admin', 'radio'] as const };

export const radioConfigQuery = queryOptions({
  queryKey: [...radioAdminKeys.all, 'config'],
  queryFn: ({ signal }): Promise<RadioConfig> =>
    request(api.GET('/admin/radio/config', { signal })),
});

export const updateRadioConfig = (body: RadioConfigUpdate): Promise<RadioConfig> =>
  request(api.PATCH('/admin/radio/config', { body }));

export const radioCatalogQuery = queryOptions({
  queryKey: [...radioAdminKeys.all, 'catalog'],
  queryFn: ({ signal }): Promise<RadioCatalogState> =>
    request(api.GET('/admin/radio/catalog', { signal })),
});

export const syncRadioCatalog = () => request(api.POST('/admin/radio/catalog/sync'));

export const radioRelaysQuery = queryOptions({
  queryKey: [...radioAdminKeys.all, 'relays'],
  queryFn: ({ signal }): Promise<RadioRelays> =>
    request(api.GET('/admin/radio/relays', { signal })),
});

export const stopRadioListener = (listenId: string) =>
  request(api.DELETE('/admin/radio/listeners/{listenId}', { params: { path: { listenId } } }));

/**
 * The station filters of the tab «Sender» (contract 1.28.0): `offered` lists the stations of the
 * offer (the server's default), `false` the whole catalog.
 */
export interface AdminRadioStationFilter {
  q?: string;
  country?: string;
  provider?: string;
  state?: 'reachable' | 'broken' | 'noLogo';
  /** Catalog or own stations (contract 1.19.0, tab «Eigene Sender»). */
  source?: 'catalog' | 'own';
  offered: boolean;
  sort?: 'popularity' | 'name';
}

/** The stations of a filter, read page by page as the table scrolls (concept 18 C-3). */
export const adminRadioStationsSource = (
  filter: AdminRadioStationFilter,
): PagedSource<AdminRadioStationPage, AdminRadioStation> => ({
  key: [...radioAdminKeys.all, 'stations', filter],
  fetch: (offset, limit, signal) =>
    request(
      api.GET('/admin/radio/stations', {
        params: { query: { ...filter, offset, limit } },
        signal,
      }),
    ),
  items: (page) => page.items,
  total: (page) => page.total,
  id: (station) => station.id,
});

/** One page of a station list of the administration. */
export const ADMIN_RADIO_PAGE_SIZE = 50;

export const adminRadioStationQuery = (stationId: number) =>
  queryOptions({
    queryKey: [...radioAdminKeys.all, 'station', stationId],
    queryFn: ({ signal }): Promise<AdminRadioStation> =>
      request(
        api.GET('/admin/radio/stations/{stationId}', { params: { path: { stationId } }, signal }),
      ),
  });

export const updateAdminRadioStation = (
  stationId: number,
  body: AdminRadioStationPatch,
): Promise<AdminRadioStation> =>
  request(
    api.PATCH('/admin/radio/stations/{stationId}', { params: { path: { stationId } }, body }),
  );

export const probeRadioStation = (stationId: number, streamId?: number): Promise<RadioProbe> =>
  request(
    api.POST('/admin/radio/stations/{stationId}/probe', {
      params: { path: { stationId } },
      body: streamId === undefined ? {} : { streamId },
    }),
  );

export const uploadRadioLogo = (stationId: number, file: File): Promise<AdminRadioStation> => {
  // openapi-fetch sends a FormData as it is (with the CSRF header of every call); its type
  // knows the part only as a string.
  const form = new FormData();
  form.append('file', file);
  return request(
    api.PUT('/admin/radio/stations/{stationId}/logo', {
      params: { path: { stationId } },
      body: form as unknown as components['schemas']['ItemImageUpload'],
    }),
  );
};

export const deleteRadioLogo = (stationId: number) =>
  request(
    api.DELETE('/admin/radio/stations/{stationId}/logo', { params: { path: { stationId } } }),
  );

/** Adds an own station (contract 1.19.0, concept 16, 5.6). */
export const createRadioStation = (body: AdminRadioStationCreate): Promise<AdminRadioStation> =>
  request(api.POST('/admin/radio/stations', { body }));

/** Deletes an own station; a catalog station can only leave the offer. */
export const deleteRadioStation = (stationId: number) =>
  request(api.DELETE('/admin/radio/stations/{stationId}', { params: { path: { stationId } } }));

// The radio offer (contract 1.28.0, concept 19 part B, B4): the stored offer, the preview of a
// draft of steps and the apply with the version the draft is based on. The server computes every
// number and every tick; the client keeps only the list of steps (rule 12).
export type RadioOffer = components['schemas']['RadioOffer'];
export type RadioOfferCountry = components['schemas']['RadioOfferCountry'];
export type RadioOfferStep = components['schemas']['RadioOfferStep'];
export type RadioOfferPreview = components['schemas']['RadioOfferPreview'];
type RadioOfferPreviewRequest = components['schemas']['RadioOfferPreviewRequest'];
export type RadioOfferApplied = components['schemas']['RadioOfferApplied'];
export type RadioOfferStation = components['schemas']['RadioOfferStation'];
export type RadioContinent = components['schemas']['RadioContinent'];

/** The continents in the order of the tab «Angebot» (B9.2). */
export const RADIO_CONTINENTS: readonly RadioContinent[] = [
  'europe',
  'northAmerica',
  'southAmerica',
  'asia',
  'africa',
  'oceania',
];

export const radioOfferQuery = queryOptions({
  queryKey: [...radioAdminKeys.all, 'offer'],
  queryFn: ({ signal }): Promise<RadioOffer> => request(api.GET('/admin/radio/offer', { signal })),
});

/** The steps on the stored offer, computed and not stored; with `list` a page of a country. */
export const previewRadioOffer = (
  body: RadioOfferPreviewRequest,
  signal?: AbortSignal,
): Promise<RadioOfferPreview> => request(api.POST('/admin/radio/offer/preview', { body, signal }));

/** Stores the steps on the offer of `version`; 409 `offer_conflict` when it changed since. */
export const applyRadioOffer = (
  version: number,
  steps: RadioOfferStep[],
): Promise<RadioOfferApplied> =>
  request(api.POST('/admin/radio/offer/apply', { body: { version, steps } }));

/** The most ids one step `mark` or `demark` takes (`RadioOfferStep.stationIds`). */
export const OFFER_STEP_IDS_MAX = 5000;

/** The steps `mark` for `ids`, at most OFFER_STEP_IDS_MAX ids each (the undo of B6). */
export function markSteps(ids: readonly number[]): RadioOfferStep[] {
  const steps: RadioOfferStep[] = [];
  for (let start = 0; start < ids.length; start += OFFER_STEP_IDS_MAX) {
    steps.push({ op: 'mark', stationIds: ids.slice(start, start + OFFER_STEP_IDS_MAX) });
  }
  return steps;
}

/**
 * Stores steps at once on the current offer (the tab «Sender»: one station in or out, «Nicht
 * erreichbare entfernen», its undo): reads the version, applies, and on 409 reads it once more
 * and applies again; the steps are relative, so nothing of the other change is lost (B4.3).
 */
export async function applyRadioOfferNow(steps: RadioOfferStep[]): Promise<RadioOfferApplied> {
  const offer = await request(api.GET('/admin/radio/offer'));
  try {
    return await applyRadioOffer(offer.version, steps);
  } catch (error) {
    if (!isApiError(error, 409)) throw error;
    const again = await request(api.GET('/admin/radio/offer'));
    return applyRadioOffer(again.version, steps);
  }
}
