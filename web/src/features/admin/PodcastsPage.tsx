import type { AdminPodcast } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { formatBytes } from '@/lib/format';
import { ButtonLink, Notice, SearchField, SelectField, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  StatusRegion,
  StatusStrip,
  ToggleToolbar,
  afterDialogsClosed,
  focusTitleIfLost,
  useSearchSelection,
  useStatusMessage,
  type TabItem,
} from '@/ui/aria';
import { podcastColumns } from './podcasts/columns';
import {
  PODCAST_FILTERS,
  PODCAST_SORTS,
  matchesFilter,
  matchesSearch,
  sortPodcasts,
  type PodcastFilter,
  type PodcastSort,
} from './podcasts/listView';
import { PodcastDownloads } from './podcasts/PodcastDownloads';
import { PodcastDownloadsBand } from './podcasts/PodcastDownloadsBand';
import { PodcastDetail } from './podcasts/PodcastDetail';
import { SubscribeDialog } from './podcasts/SubscribeDialog';
import styles from './podcasts/Podcasts.module.css';
import { librariesQuery, podcastDownloadsQuery, podcastsQuery } from './queries';

/** After "Jetzt abrufen" the list asks again this often for a while, until the fetch shows. */
const REFRESH_POLL_MS = 2000;
const REFRESH_POLL_FOR_MS = 30_000;

/** While downloads wait or run, the queue is asked again this often (12.3). */
const QUEUE_POLL_MS = 3000;

/** The tab with every podcast; the others are the podcast libraries (`?tab=library-3`). */
const ALL = 'all';
/**
 * The tab of a library. Not the bare id: the router writes a text that reads as a number in
 * quotes (`?tab=%223%22`).
 */
const libraryTab = (id: number) => `library-${id}`;

/**
 * Podcasts (M6, H-2; concept 12, variant C; concept 19 part A, 12.2 variant A «Liste mit Covern»):
 * the counters, the bands «Neue Folgen», the section «Downloads» and the subscribed feeds and the
 * podcasts read from folders (ADR 0015), filtered by tabs «Alle» and one per podcast library and
 * within them by state, search and order; each podcast is a row with its cover that folds out
 * into its author, feed address, library, error, setting and actions (POD-1 to POD-3), and links
 * to its own page. «Jetzt abrufen» polls the list every 2 s for 30 s, paused while a dialog is
 * open; while downloads wait or run, the queue is polled every 3 s and the list follows it.
 */
export function PodcastsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.podcasts.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.podcasts.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <PodcastsContent />;
}

function PodcastsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const subscribeButton = useRef<HTMLButtonElement>(null);

  const [subscribing, setSubscribing] = useState(false);
  const [removing, setRemoving] = useState<AdminPodcast | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const hold = confirmOpen || subscribing;

  // «Jetzt abrufen» polls for a while; the podcasts asked for show «wird abgerufen» until their
  // last fetch changes or the polling ends.
  const [pollUntil, setPollUntil] = useState(0);
  const [fetching, setFetching] = useState<ReadonlyMap<string, string | undefined>>(
    () => new Map(),
  );
  useEffect(() => {
    if (pollUntil === 0) return;
    const timer = window.setTimeout(
      () => {
        setPollUntil(0);
        setFetching(new Map());
      },
      Math.max(0, pollUntil - Date.now()),
    );
    return () => window.clearTimeout(timer);
  }, [pollUntil]);
  const polling = pollUntil > 0 && !hold;

  const podcasts = useQuery({
    ...podcastsQuery,
    refetchInterval: polling ? REFRESH_POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  const libraries = useQuery({ ...librariesQuery, refetchOnWindowFocus: !hold });
  const queue = useQuery({
    ...podcastDownloadsQuery,
    refetchInterval: (query) =>
      (query.state.data?.total ?? 0) > 0 && !hold ? QUEUE_POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  // The counts of the rows follow the queue: when it changes, the list is asked again.
  const queueState = queue.data ? `${queue.data.total}/${queue.data.running}` : '';
  const lastQueueState = useRef(queueState);
  useEffect(() => {
    if (lastQueueState.current === queueState) return;
    const first = lastQueueState.current === '';
    lastQueueState.current = queueState;
    if (!first)
      void queryClient.invalidateQueries({ queryKey: podcastsQuery.queryKey, exact: true });
  }, [queueState, queryClient]);
  const podcastLibraries = useMemo(
    () => (libraries.data ?? []).filter((library) => library.kind === 'podcasts'),
    [libraries.data],
  );
  const libraryName = useCallback(
    (id: number) => libraries.data?.find((library) => library.id === id)?.name ?? String(id),
    [libraries.data],
  );
  // One tab per podcast library, and per other library a podcast still names (from
  // `AdminPodcast.libraryId`), in the order of the library list.
  const tabLibraries = useMemo(() => {
    const used = new Set((podcasts.data ?? []).map((podcast) => podcast.libraryId));
    return (libraries.data ?? []).filter(
      (library) => library.kind === 'podcasts' || used.has(library.id),
    );
  }, [podcasts.data, libraries.data]);
  const tabIds = useMemo(
    () => [ALL, ...tabLibraries.map((library) => libraryTab(library.id))],
    [tabLibraries],
  );
  // An unknown `?tab=` shows «Alle»; once the lists are there, the address says so too.
  const loaded = podcasts.isSuccess && libraries.isSuccess;
  const [tab, selectTab] = useSearchSelection('tab', tabIds, ALL, { normalise: loaded });

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const onExpandedChange = useCallback(
    (key: string, open: boolean) =>
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      }),
    [],
  );
  const [filter, setFilter] = useState<PodcastFilter>('all');
  const [search, setSearch] = useState('');
  const [sort, setSort] = useState<PodcastSort>('title');
  const collator = useMemo(() => new Intl.Collator(i18n.language), [i18n.language]);
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const invalidate = () => queryClient.invalidateQueries({ queryKey: podcastsQuery.queryKey });

  const refresh = useMutation({
    mutationFn: (podcast: AdminPodcast): Promise<AdminPodcast> =>
      request(api.POST('/admin/podcasts/{id}/refresh', { params: { path: { id: podcast.id } } })),
    onSuccess: async (_data, podcast) => {
      setFailure(null);
      setMessage(t('admin.podcasts.refresh.queued', { title: podcast.title }));
      setFetching((current) => new Map(current).set(podcast.id, podcast.lastCheckedAt));
      setPollUntil(Date.now() + REFRESH_POLL_FOR_MS);
      await invalidate();
    },
    onError: (error) => {
      setMessage(null);
      setFailure(error);
    },
  });
  const remove = useMutation({
    mutationFn: (podcast: AdminPodcast) =>
      request(api.DELETE('/admin/podcasts/{id}', { params: { path: { id: podcast.id } } })),
    onSuccess: async (_data, podcast) => {
      setFailure(null);
      setMessage(t('admin.podcasts.remove.done', { title: podcast.title }));
      // Its open fold-out and «wird abgerufen» go with it.
      onExpandedChange(podcast.id, false);
      setFetching((current) => {
        const next = new Map(current);
        next.delete(podcast.id);
        return next;
      });
      await invalidate();
      // The row and its buttons are gone: once the dialog has closed, the h1 takes the focus.
      await afterDialogsClosed();
      titleRef.current?.focus();
    },
    onError: (error) => {
      setMessage(null);
      setFailure(error);
    },
  });

  // The confirmation is the page's own: a row that goes while it is open (another tab, the
  // polling) cannot take the dialog with it.
  const askRemove = useCallback((podcast: AdminPodcast) => {
    setRemoving(podcast);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const isFetching = useCallback(
    (podcast: AdminPodcast) =>
      fetching.has(podcast.id) && fetching.get(podcast.id) === podcast.lastCheckedAt,
    [fetching],
  );
  const columns = useMemo(
    () => podcastColumns(t, i18n.language, number, isFetching),
    [t, i18n.language, number, isFetching],
  );
  const detail = (podcast: AdminPodcast) => (
    <PodcastDetail
      podcast={podcast}
      libraryName={libraryName(podcast.libraryId)}
      refreshing={refresh.isPending && refresh.variables.id === podcast.id}
      onRefresh={refresh.mutate}
      onRemove={askRemove}
    />
  );

  const list = podcasts.data ?? [];
  // The filters, the search and the order are the page's, so a change of tab keeps them; the
  // counts of the filters are those of the tab.
  const toolbar = (rows: AdminPodcast[]) => (
    <div className={styles.toolbar}>
      <ToggleToolbar
        label={t('admin.podcasts.filter.label')}
        value={filter}
        onChange={(value) => setFilter(value as PodcastFilter)}
        options={PODCAST_FILTERS.map((value) => ({
          value,
          label: t(`admin.podcasts.filter.${value}`),
          count: number.format(rows.filter((podcast) => matchesFilter(podcast, value)).length),
        }))}
      />
      <div className={styles.toolbarFields}>
        <SearchField
          label={t('admin.podcasts.search.label')}
          placeholder={t('admin.podcasts.search.placeholder')}
          value={search}
          onChange={setSearch}
        />
        <SelectField
          label={t('admin.podcasts.sort.label')}
          value={sort}
          onChange={setSort}
          options={PODCAST_SORTS.map((value) => ({
            value,
            label: t(`admin.podcasts.sort.${value}`),
          }))}
        />
      </div>
    </div>
  );
  const tabOf = (id: string, label: string, rows: AdminPodcast[], library?: string): TabItem => {
    const shown = sortPodcasts(
      rows.filter(
        (podcast) =>
          matchesFilter(podcast, filter) && matchesSearch(podcast, search, i18n.language),
      ),
      sort,
      collator,
    );
    return {
      id,
      label,
      count: number.format(rows.length),
      countLabel: t('admin.podcasts.tabs.count', {
        count: rows.length,
        number: number.format(rows.length),
      }),
      content:
        rows.length === 0 ? (
          <Text tone="secondary">
            {library ? t('admin.podcasts.emptyLibrary') : t('admin.podcasts.empty')}
          </Text>
        ) : (
          <div className={styles.listBody}>
            {toolbar(rows)}
            {shown.length === 0 ? (
              <Text tone="secondary">{t('admin.podcasts.noMatch')}</Text>
            ) : (
              <DataTable
                caption={
                  library ? t('admin.podcasts.listOf', { library }) : t('admin.podcasts.list')
                }
                columns={columns}
                rows={shown}
                rowKey={(podcast) => podcast.id}
                expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
              />
            )}
          </div>
        ),
    };
  };
  // The counters (12.1): what the list holds, what stores, the bytes on the folder, the queue.
  const saving = list.filter((podcast) => podcast.effectiveMode === 'save').length;
  const linking = list.filter((podcast) => podcast.effectiveMode === 'link').length;
  const storedBytes = list.reduce((sum, podcast) => sum + podcast.storedBytes, 0);
  const status = podcasts.data && (
    <StatusStrip
      label={t('admin.podcasts.kpi.label')}
      tiles={[
        {
          key: 'podcasts',
          label: t('admin.podcasts.kpi.podcasts'),
          value: number.format(list.length),
        },
        {
          key: 'mode',
          label: t('admin.podcasts.kpi.mode'),
          value: t('admin.podcasts.kpi.saving', { number: number.format(saving) }),
          note: t('admin.podcasts.kpi.linking', { number: number.format(linking) }),
        },
        {
          key: 'stored',
          label: t('admin.podcasts.kpi.stored'),
          value: formatBytes(storedBytes, i18n.language),
        },
        {
          key: 'queue',
          label: t('admin.podcasts.kpi.queue'),
          value: queue.data
            ? queue.data.total === 0
              ? t('admin.podcasts.kpi.queueEmpty')
              : t('admin.podcasts.kpi.queueRunning', { number: number.format(queue.data.running) })
            : t('admin.podcasts.noError'),
          note:
            queue.data && queue.data.total > 0
              ? queue.data.paused
                ? t('admin.podcasts.kpi.queuePaused')
                : t('admin.podcasts.kpi.queueWaiting', {
                    number: number.format(queue.data.total - queue.data.running),
                  })
              : undefined,
          tone: queue.data?.paused && queue.data.total > 0 ? 'warning' : 'neutral',
        },
      ]}
    />
  );
  const noLibrary = libraries.data !== undefined && podcastLibraries.length === 0;
  // The band «Neue Folgen» belongs to the chosen tab (12.3): its library, or every podcast
  // library in «Alle», each band then named after its library.
  const bandLibraries = podcastLibraries.flatMap((library) =>
    library.podcastDownloads && (tab === ALL || tab === libraryTab(library.id))
      ? [{ ...library, podcastDownloads: library.podcastDownloads }]
      : [],
  );

  return (
    <AdminPage
      title={t('admin.podcasts.title')}
      help={t('admin.podcasts.intro')}
      liveSeconds={polling ? REFRESH_POLL_MS / 1000 : undefined}
      titleRef={titleRef}
      status={status}
      actions={
        noLibrary ? undefined : (
          <AriaButton
            ref={subscribeButton}
            variant="primary"
            icon="plus"
            isDisabled={podcastLibraries.length === 0}
            onPress={() => setSubscribing(true)}
          >
            {t('admin.podcasts.subscribe.action')}
          </AriaButton>
        )
      }
    >
      {noLibrary && (
        <Notice
          actions={
            <ButtonLink href={paths.adminLibraries} variant="secondary" icon="folder">
              {t('admin.podcasts.librariesLink')}
            </ButtonLink>
          }
        >
          {t('admin.podcasts.noLibrary')}
        </Notice>
      )}
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {(podcasts.isPending || libraries.isPending) && <Skeleton shape="block" />}
      {podcasts.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, podcasts.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void podcasts.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {libraries.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, libraries.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void libraries.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {podcasts.data && bandLibraries.length > 0 && (
        <div className={styles.bands}>
          {bandLibraries.map((library) => (
            <PodcastDownloadsBand
              key={library.id}
              library={library}
              titled={podcastLibraries.length > 1}
            />
          ))}
        </div>
      )}
      {queue.data && (bandLibraries.length > 0 || queue.data.total > 0) && (
        <PodcastDownloads queue={queue.data} />
      )}
      {/* The tabs come from both lists: without the libraries a linked tab would flicker. */}
      {podcasts.data && libraries.data && (
        <BandPanel
          title={t('admin.podcasts.list')}
          icon="mic"
          tabs={{
            label: t('admin.podcasts.tabs.label'),
            selectedKey: tab,
            onSelectionChange: selectTab,
            tabs: [
              tabOf(ALL, t('admin.podcasts.tabs.all'), list),
              ...tabLibraries.map((library) =>
                tabOf(
                  libraryTab(library.id),
                  library.name,
                  list.filter((podcast) => podcast.libraryId === library.id),
                  library.name,
                ),
              ),
            ],
          }}
        />
      )}
      {removing && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={t('admin.podcasts.remove.title', { title: removing.title })}
          body={t('admin.podcasts.remove.body')}
          confirmLabel={t('admin.podcasts.remove.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => remove.mutate(removing)}
        />
      )}
      {subscribing && (
        <SubscribeDialog
          libraries={podcastLibraries}
          onClose={() => {
            setSubscribing(false);
            subscribeButton.current?.focus();
          }}
          onSubscribed={(podcast) => {
            setSubscribing(false);
            setFailure(null);
            setMessage(t('admin.podcasts.subscribe.done', { title: podcast.title }));
            void invalidate();
            subscribeButton.current?.focus();
          }}
        />
      )}
    </AdminPage>
  );
}
