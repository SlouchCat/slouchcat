import type { AdminPodcast, AdminPodcastEpisode } from '@slouchcat/api-client';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, apiUrl, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatBytes, formatDate, formatDateTime, formatDuration } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Notice, ProgressBar, Skeleton, SquareCover, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  DataTable,
  StatusRegion,
  StatusStrip,
  ToggleToolbar,
  useSearchSelection,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import { feedError } from './podcasts/columns';
import { downloadShare } from './podcasts/PodcastDownloads';
import { PodcastSetting } from './podcasts/PodcastSetting';
import styles from './podcasts/Podcasts.module.css';
import {
  librariesQuery,
  podcastDownloadsQuery,
  podcastEpisodePage,
  podcastEpisodesQuery,
  podcastQuery,
  type PodcastEpisodeFilter,
} from './queries';

const route = getRouteApi('/_authed/_active/_chrome/admin/podcasts/$podcastId');

/** The views of the episode list, in `?tab=` (all by default). */
const FILTERS: readonly PodcastEpisodeFilter[] = ['all', 'with_file', 'without_file'];

/** While a download of the podcast waits or runs, the page asks again this often. */
const ACTIVE_POLL_MS = 3000;
/** After «Jetzt abrufen» the page asks again this often for a while. */
const REFRESH_POLL_MS = 2000;
const REFRESH_POLL_FOR_MS = 30_000;

const STATE_TONE = {
  file: 'success',
  link: 'outline',
  queued: 'info',
  running: 'info',
  failed: 'danger',
  removed: 'warning',
  unclear: 'warning',
} as const;

/**
 * The page of one podcast (concept 19 part A, 12.2, mockup «Podcast»): its cover and counters,
 * its own setting with «Fehlende nachladen» (the component of the fold-out of the list), and its
 * episodes newest first in the views Alle, Gespeichert and Ohne Datei, each with the state the
 * server decided and «Laden», «Erneut» or «Abbrechen». Only the main profile of an admin sees it,
 * like the list.
 */
export function PodcastPage() {
  const { t } = useTranslation();
  const session = useActiveSession();
  const { podcastId } = route.useParams();
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
  return <PodcastView podcastId={podcastId} />;
}

/** The action of an episode in its state, or none (a file that is there, or nothing to load). */
type EpisodeAction = 'load' | 'retry' | 'cancel';

export function episodeAction(episode: AdminPodcastEpisode): EpisodeAction | null {
  switch (episode.state) {
    case 'queued':
    case 'running':
      return 'cancel';
    case 'failed':
      return episode.feed ? 'retry' : null;
    case 'link':
    case 'removed':
    case 'unclear':
      return episode.feed ? 'load' : null;
    default:
      return null;
  }
}

/** The line below the title of an episode: length, size and what its state means. */
export function episodeMeta(
  t: TFunction,
  language: string,
  episode: AdminPodcastEpisode,
  paused = false,
): string {
  const parts: string[] = [];
  if (episode.durationMs !== undefined) parts.push(formatDuration(t, episode.durationMs));
  if (episode.sizeBytes !== undefined) parts.push(formatBytes(episode.sizeBytes, language));
  const share = downloadShare(episode.progressBytes, episode.totalBytes);
  switch (episode.state) {
    case 'file':
      if (episode.fileName) parts.push(episode.fileName);
      break;
    case 'link':
      parts.push(t('admin.podcasts.episodes.meta.link'));
      break;
    case 'queued':
      parts.push(
        paused ? t('admin.podcasts.queue.state.paused') : t('admin.podcasts.queue.state.queued'),
      );
      break;
    case 'running':
      parts.push(
        share !== undefined
          ? t('admin.podcasts.queue.state.runningShare', { share })
          : t('admin.podcasts.queue.state.running'),
      );
      break;
    case 'failed':
      parts.push(
        t(`admin.podcasts.episodes.error.${episode.errorCode ?? 'enclosure_unavailable'}`),
        episode.retryAt
          ? t('admin.podcasts.episodes.meta.retryAt', {
              at: formatDateTime(episode.retryAt, language),
            })
          : t('admin.podcasts.episodes.meta.noRetry'),
      );
      break;
    case 'removed':
      parts.push(
        episode.fileName
          ? t('admin.podcasts.episodes.meta.removedFile', { file: episode.fileName })
          : t('admin.podcasts.episodes.meta.removed'),
      );
      break;
    case 'unclear':
      parts.push(t('admin.podcasts.episodes.meta.unclear'));
      break;
  }
  return parts.join(' · ');
}

function PodcastView({ podcastId }: { podcastId: string }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const shownRef = useRef<HTMLParagraphElement>(null);
  const [message, say] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const [pollUntil, setPollUntil] = useState(0);
  useEffect(() => {
    if (pollUntil === 0) return;
    const timer = window.setTimeout(() => setPollUntil(0), Math.max(0, pollUntil - Date.now()));
    return () => window.clearTimeout(timer);
  }, [pollUntil]);
  const [filter, selectFilter] = useSearchSelection('tab', FILTERS, 'all', { normalise: true });

  const podcastData = useQuery({ ...podcastQuery(podcastId), enabled: false }).data;
  const active =
    podcastData !== undefined && podcastData.downloads.queued + podcastData.downloads.running > 0;
  const interval = pollUntil > 0 ? REFRESH_POLL_MS : active ? ACTIVE_POLL_MS : false;
  const podcast = useQuery({ ...podcastQuery(podcastId), refetchInterval: interval });
  const libraries = useQuery(librariesQuery);
  // Whether the downloads are paused: a waiting episode says «angehalten», like the queue.
  const queue = useQuery({ ...podcastDownloadsQuery, refetchInterval: interval });
  const paused = queue.data?.paused ?? false;
  const view = filter as PodcastEpisodeFilter;
  const episodesOptions = podcastEpisodesQuery(podcastId, view);
  const episodes = useInfiniteQuery(episodesOptions);
  // While the page polls, only the first page is asked again and replaces its old copy: every
  // page costs the server the states of all episodes of the podcast, so the pages loaded by
  // «Mehr zeigen» are not fetched every few seconds. A change by a button asks for all of them.
  const episodesKey = episodesOptions.queryKey;
  useEffect(() => {
    if (interval === false) return;
    const timer = window.setInterval(() => {
      podcastEpisodePage(podcastId, view, 0)
        .then((fresh) =>
          queryClient.setQueryData(episodesKey, (old) =>
            old ? { ...old, pages: [fresh, ...old.pages.slice(1)] } : old,
          ),
        )
        .catch(() => undefined); // the next tick or a button tries again
    }, interval);
    return () => window.clearInterval(timer);
    // episodesKey follows podcastId and view.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [interval, podcastId, view, queryClient]);
  const title = podcast.data?.title ?? t('admin.podcasts.title');
  useDocumentTitle(title);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['admin', 'podcasts'] });
  const done = async (text: string) => {
    setFailure(null);
    say(text);
    await invalidate();
  };
  const failed = (error: unknown) => {
    say(null);
    setFailure(error);
  };
  const refresh = useMutation({
    mutationFn: (p: AdminPodcast): Promise<AdminPodcast> =>
      request(api.POST('/admin/podcasts/{id}/refresh', { params: { path: { id: p.id } } })),
    onSuccess: async (p) => {
      setPollUntil(Date.now() + REFRESH_POLL_FOR_MS);
      await done(t('admin.podcasts.refresh.queued', { title: p.title }));
    },
    onError: failed,
  });
  const load = useMutation({
    mutationFn: (e: AdminPodcastEpisode): Promise<{ queued: number }> =>
      request(
        api.POST('/admin/podcasts/{id}/download', {
          params: { path: { id: podcastId } },
          body: { episodeIds: [e.id] },
        }),
      ),
    onSuccess: ({ queued }, e) =>
      done(
        queued === 0
          ? t('admin.podcasts.episodes.loadNone', { title: e.title })
          : t('admin.podcasts.episodes.loadQueued', { title: e.title }),
      ),
    onError: failed,
  });
  const cancel = useMutation({
    mutationFn: (e: AdminPodcastEpisode) =>
      request(
        api.DELETE('/admin/podcasts/{id}/episodes/{episodeId}/download', {
          params: { path: { id: podcastId, episodeId: e.id } },
        }),
      ),
    onSuccess: (_data, e) => done(t('admin.podcasts.queue.cancelled', { title: e.title })),
    onError: failed,
  });

  if (podcast.isError && isApiError(podcast.error, 404)) {
    return (
      <AdminPage title={t('admin.podcasts.page.missing')} titleRef={titleRef}>
        <Notice
          actions={
            <ButtonLink href={paths.adminPodcasts} variant="secondary" icon="chevronLeft">
              {t('admin.podcasts.page.back')}
            </ButtonLink>
          }
        >
          {t('admin.podcasts.page.missingBody')}
        </Notice>
      </AdminPage>
    );
  }

  const p = podcast.data;
  const libraryName =
    p &&
    (libraries.data?.find((library) => library.id === p.libraryId)?.name ?? String(p.libraryId));
  const pages = episodes.data?.pages ?? [];
  // A page fetched again may hold an episode the next page holds too (the list moved meanwhile).
  const seen = new Set<string>();
  const rows = pages
    .flatMap((page) => page.episodes)
    .filter((e) => !seen.has(e.id) && Boolean(seen.add(e.id)));
  const first = pages[0];
  const total = pages.at(-1)?.total ?? 0;
  const busy = (e: AdminPodcastEpisode) =>
    (load.isPending && load.variables.id === e.id) ||
    (cancel.isPending && cancel.variables.id === e.id);
  const columns: DataColumn<AdminPodcastEpisode>[] = [
    {
      key: 'date',
      header: t('admin.podcasts.episodes.columns.date'),
      cell: (e) =>
        e.publishedAt ? (
          <time className={styles.nowrap} dateTime={e.publishedAt}>
            {formatDate(e.publishedAt, i18n.language)}
          </time>
        ) : (
          t('admin.podcasts.noError')
        ),
    },
    {
      key: 'title',
      header: t('admin.podcasts.episodes.columns.title'),
      rowHeader: true,
      cell: (e) => {
        const share = downloadShare(e.progressBytes, e.totalBytes);
        const meta = episodeMeta(t, i18n.language, e, paused);
        return (
          <span className={styles.stack}>
            <span>{e.title}</span>
            <span className={`${styles.muted} ${styles.small}`}>{meta}</span>
            {e.state === 'running' && <ProgressBar value={share ?? 0} label={meta} />}
          </span>
        );
      },
    },
    {
      key: 'state',
      header: t('admin.podcasts.episodes.columns.state'),
      cell: (e) => (
        <Chip tone={STATE_TONE[e.state]}>{t(`admin.podcasts.episodes.state.${e.state}`)}</Chip>
      ),
    },
    {
      key: 'action',
      header: t('admin.podcasts.episodes.columns.action'),
      hideHeader: true,
      align: 'end',
      cell: (e) => {
        const action = episodeAction(e);
        if (!action) return null;
        // One button per row whose label follows the state, so the focus stays on it when
        // «Laden» turns into «Abbrechen».
        return (
          <AriaButton
            variant={action === 'cancel' ? 'ghost' : 'secondary'}
            icon={action === 'cancel' ? 'close' : action === 'retry' ? 'refresh' : 'download'}
            aria-label={t(`admin.podcasts.episodes.action.${action}Label`, { title: e.title })}
            isDisabled={action !== 'cancel' && p !== undefined && !p.savable}
            isPending={busy(e)}
            onPress={() => (action === 'cancel' ? cancel.mutate(e) : load.mutate(e))}
          >
            {t(`admin.podcasts.episodes.action.${action}`)}
          </AriaButton>
        );
      },
    },
  ];

  const more = () => {
    void episodes.fetchNextPage().then((result) => {
      // The button goes with the last page; the line that counts the episodes takes the focus.
      if (!result.hasNextPage) shownRef.current?.focus();
    });
  };
  const missing = p ? Math.max(0, p.episodeCount - p.localEpisodeCount) : 0;
  const status = p && (
    <StatusStrip
      label={t('admin.podcasts.page.counters')}
      tiles={[
        {
          key: 'episodes',
          label: t('admin.podcasts.page.episodes'),
          value: number.format(p.episodeCount),
        },
        {
          key: 'stored',
          label: t('admin.podcasts.page.stored'),
          value: number.format(p.localEpisodeCount),
          note:
            missing > 0
              ? t('admin.podcasts.page.missingCount', { number: number.format(missing) })
              : undefined,
        },
        {
          key: 'bytes',
          label: t('admin.podcasts.page.bytes'),
          value: formatBytes(p.storedBytes, i18n.language),
        },
        {
          key: 'newest',
          label: t('admin.podcasts.page.newest'),
          value: p.latestEpisodeAt
            ? formatDate(p.latestEpisodeAt, i18n.language)
            : t('admin.podcasts.noError'),
        },
      ]}
    />
  );
  const error = p?.lastError ? feedError(t, p.lastError) : null;

  return (
    <AdminPage
      title={title}
      titleRef={titleRef}
      liveSeconds={interval === false ? undefined : interval / 1000}
      status={status}
      actions={
        <>
          <ButtonLink href={paths.adminPodcasts} variant="ghost" icon="chevronLeft">
            {t('admin.podcasts.page.back')}
          </ButtonLink>
          {p?.feedUrl && (
            <AriaButton
              variant="secondary"
              icon="refresh"
              aria-label={t('admin.podcasts.refresh.label', { title: p.title })}
              isPending={refresh.isPending}
              onPress={() => refresh.mutate(p)}
            >
              {t('admin.podcasts.refresh.action')}
            </AriaButton>
          )}
        </>
      }
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {podcast.isPending && <Skeleton shape="block" />}
      {podcast.isError && !isApiError(podcast.error, 404) && (
        <Notice
          tone="danger"
          title={problemMessage(t, podcast.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void podcast.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {p && (
        <>
          <div className={styles.head}>
            <SquareCover
              title={p.title}
              src={p.coverUrl ? apiUrl(p.coverUrl) : undefined}
              size="lg"
            />
            <div className={styles.headText}>
              <span className={styles.muted}>
                {[
                  p.author,
                  t('admin.podcasts.page.library', { library: libraryName }),
                  p.feedOrigin === 'file' || (!p.feedUrl && p.rootId !== undefined)
                    ? t('admin.podcasts.fromFolder')
                    : undefined,
                ]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
              <span className={styles.title}>
                {p.effectiveMode && (
                  <Chip tone={p.effectiveMode === 'save' ? 'success' : 'outline'}>
                    {t(`admin.podcasts.stored.${p.effectiveMode}`)}
                  </Chip>
                )}
                {p.explicit && <Chip tone="warning">{t('admin.podcasts.explicit')}</Chip>}
              </span>
              {error && (
                <div className={styles.error}>
                  <p>{error.message}</p>
                  {error.detail && <p className={styles.url}>{error.detail}</p>}
                </div>
              )}
            </div>
          </div>
          <div className={styles.columns}>
            <div className={styles.side}>
              <BandPanel title={t('admin.podcasts.page.setting')} icon="sliders">
                <div className={styles.setting}>
                  {p.effectiveMode ? (
                    <PodcastSetting podcast={p} />
                  ) : (
                    <Text tone="secondary">{t('admin.podcasts.detail.noFeedFolder')}</Text>
                  )}
                  <dl className={styles.facts}>
                    <div>
                      <dt>{t('admin.podcasts.detail.feedUrl')}</dt>
                      <dd>
                        {p.feedUrl ? (
                          <span className={styles.url}>{p.feedUrl}</span>
                        ) : (
                          t('admin.podcasts.detail.noFeed')
                        )}
                      </dd>
                    </div>
                    {p.feedUrl && (
                      <div>
                        <dt>{t('admin.podcasts.columns.lastFetch')}</dt>
                        <dd>
                          {p.lastCheckedAt
                            ? formatDateTime(p.lastCheckedAt, i18n.language)
                            : t('admin.podcasts.never')}
                        </dd>
                      </div>
                    )}
                  </dl>
                </div>
              </BandPanel>
            </div>
            <div className={styles.main}>
              <BandPanel title={t('admin.podcasts.episodes.title')} icon="list">
                <div className={styles.listBody}>
                  <ToggleToolbar
                    label={t('admin.podcasts.episodes.filter.label')}
                    value={filter}
                    onChange={selectFilter}
                    options={FILTERS.map((value) => {
                      const count =
                        first === undefined
                          ? undefined
                          : value === 'all'
                            ? first.counts.all
                            : value === 'with_file'
                              ? first.counts.withFile
                              : first.counts.withoutFile;
                      return {
                        value,
                        label: t(`admin.podcasts.episodes.filter.${value}`),
                        count: count === undefined ? '–' : number.format(count),
                      };
                    })}
                  />
                  {episodes.isPending && <Skeleton shape="block" />}
                  {episodes.isError && (
                    <Notice
                      tone="danger"
                      title={problemMessage(t, episodes.error)}
                      actions={
                        <AriaButton icon="refresh" onPress={() => void episodes.refetch()}>
                          {t('common.retry')}
                        </AriaButton>
                      }
                    />
                  )}
                  {episodes.isSuccess &&
                    (rows.length === 0 ? (
                      <Text tone="secondary">{t('admin.podcasts.episodes.empty')}</Text>
                    ) : (
                      <DataTable
                        caption={t('admin.podcasts.episodes.caption', { title: p.title })}
                        columns={columns}
                        rows={rows}
                        rowKey={(e) => e.id}
                      />
                    ))}
                  {episodes.isSuccess && rows.length > 0 && (
                    <div className={styles.actions}>
                      <p ref={shownRef} tabIndex={-1} className={styles.summary}>
                        {t('admin.podcasts.episodes.shown', {
                          shown: number.format(rows.length),
                          total: number.format(total),
                        })}
                      </p>
                      {episodes.hasNextPage && (
                        <AriaButton
                          variant="secondary"
                          icon="chevronDown"
                          isPending={episodes.isFetchingNextPage}
                          onPress={more}
                        >
                          {t('admin.podcasts.episodes.more')}
                        </AriaButton>
                      )}
                    </div>
                  )}
                </div>
              </BandPanel>
            </div>
          </div>
        </>
      )}
    </AdminPage>
  );
}
