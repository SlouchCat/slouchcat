import type { AdminPodcast } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  afterRender,
  focusTitleIfLost,
  useSearchSelection,
  type TabItem,
} from '@/ui/aria';
import { podcastColumns } from './podcasts/columns';
import { PodcastDetail } from './podcasts/PodcastDetail';
import { SubscribeDialog } from './podcasts/SubscribeDialog';
import { librariesQuery, podcastsQuery } from './queries';
import styles from './podcasts/Podcasts.module.css';

/** After "Jetzt abrufen" the list asks again this often for a while, until the fetch shows. */
const REFRESH_POLL_MS = 2000;
const REFRESH_POLL_FOR_MS = 30_000;

/** The tab with every podcast; the others are the podcast libraries (`?tab=library-3`). */
const ALL = 'all';
/**
 * The tab of a library. Not the bare id: the router writes a text that reads as a number in
 * quotes (`?tab=%223%22`).
 */
const libraryTab = (id: number) => `library-${id}`;

/**
 * Podcasts (M6, H-2; concept 12, variant C): the subscribed feeds and the podcasts read from
 * folders (ADR 0015), filtered by tabs «Alle» and one per podcast library; each podcast is a row
 * that folds out into its author, feed address, library, error and actions (POD-1 to POD-3).
 * «Jetzt abrufen» polls the list every 2 s for 30 s, paused while a dialog is open.
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
  const [tab, selectTab] = useSearchSelection('tab', tabIds, ALL);
  // An unknown `?tab=` shows «Alle»; once the lists are there, the address says so too.
  const search: Record<string, unknown> = useSearch({ strict: false });
  const askedTab = search.tab;
  const loaded = podcasts.isSuccess && libraries.isSuccess;
  useEffect(() => {
    if (
      loaded &&
      askedTab !== undefined &&
      !(typeof askedTab === 'string' && tabIds.includes(askedTab))
    )
      selectTab(ALL);
  }, [loaded, askedTab, tabIds, selectTab]);

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
  const [message, setMessage] = useState<string | null>(null);
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
      // The row and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterRender();
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
    void afterRender().then(() => focusTitleIfLost(titleRef));
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
  const tabOf = (id: string, label: string, rows: AdminPodcast[], library?: string): TabItem => ({
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
        <DataTable
          caption={library ? t('admin.podcasts.listOf', { library }) : t('admin.podcasts.list')}
          columns={columns}
          rows={rows}
          rowKey={(podcast) => podcast.id}
          expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
        />
      ),
  });
  const noLibrary = libraries.data !== undefined && podcastLibraries.length === 0;

  return (
    <AdminPage
      title={t('admin.podcasts.title')}
      help={t('admin.podcasts.intro')}
      liveSeconds={polling ? REFRESH_POLL_MS / 1000 : undefined}
      titleRef={titleRef}
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
      <div role="status" className={styles.live}>
        {message && failure === null && <Notice live={false}>{message}</Notice>}
      </div>
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
