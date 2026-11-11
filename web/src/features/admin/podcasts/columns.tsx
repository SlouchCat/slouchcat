import type { AdminPodcast } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { apiUrl } from '@/api/client';
import { codeMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatBytes, formatDate } from '@/lib/format';
import { ButtonLink, Chip, ProgressBar, SquareCover } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import styles from './Podcasts.module.css';

/**
 * The last fetch error of a podcast: `feed_unreachable: HTTP 404` is the code in words and the
 * server's text as the detail; an unknown code shows the whole text.
 */
export function feedError(t: TFunction, lastError: string): { message: string; detail: string } {
  const [code = '', ...rest] = lastError.split(':');
  const message = codeMessage(t, code.trim());
  return message ? { message, detail: rest.join(':').trim() } : { message: lastError, detail: '' };
}

/** The state of a podcast in its row and in the filter «Lädt» / «Fehler» (12.1). */
export type PodcastRowState = 'fetching' | 'loading' | 'feedError' | 'notSavable' | 'current';

/**
 * What the row of a podcast says about it, from what the server sent: a fetch the page asked for,
 * downloads that wait or run, a feed error, «Speichern» that applies to a folder that fails the
 * check, else «Aktuell».
 */
export function podcastRowState(podcast: AdminPodcast, fetching: boolean): PodcastRowState {
  if (fetching) return 'fetching';
  if (podcast.downloads.queued + podcast.downloads.running > 0) return 'loading';
  if (podcast.lastError) return 'feedError';
  if (podcast.effectiveMode === 'save' && !podcast.savable) return 'notSavable';
  return 'current';
}

const STATE_TONE = {
  fetching: 'info',
  loading: 'info',
  feedError: 'danger',
  notSavable: 'warning',
  current: 'outline',
} as const;

/**
 * The columns of the list of podcasts (concept 19 part A, 12.2, variant A «Liste mit Covern»):
 * cover and title with author, «Speichert» or «Nur Link» (marked when the podcast has an own
 * setting), «Explizit» and «aus Ordner» -- the title folds the row out; the episodes with the
 * stored share as a bar, the stored bytes, the newest episode, the state, and the link to the
 * page of the podcast. The whole error, the feed address and the setting are in the fold-out.
 */
export function podcastColumns(
  t: TFunction,
  language: string,
  number: Intl.NumberFormat,
  isFetching: (podcast: AdminPodcast) => boolean,
): DataColumn<AdminPodcast>[] {
  return [
    {
      key: 'title',
      header: t('admin.podcasts.columns.title'),
      rowHeader: true,
      cell: (podcast) => (
        <span className={styles.podcast}>
          <SquareCover
            title={podcast.title}
            src={podcast.coverUrl ? apiUrl(podcast.coverUrl) : undefined}
          />
          <span className={styles.stack}>
            {/* The spaces between the parts keep the button's name readable («Titel Autor …»). */}
            <span className={styles.name}>{podcast.title}</span>{' '}
            <span className={styles.title}>
              {podcast.author && <span className={styles.muted}>{podcast.author} </span>}
              {podcast.effectiveMode && (
                <Chip tone={podcast.effectiveMode === 'save' ? 'success' : 'outline'}>
                  {t(`admin.podcasts.stored.${podcast.effectiveMode}`)}
                </Chip>
              )}
              {podcast.mode !== undefined && podcast.mode !== 'inherit' && (
                <span className={styles.muted}> {t('admin.podcasts.stored.own')}</span>
              )}
              {podcast.feedOrigin === 'file' && (
                <>
                  {' '}
                  <Chip tone="outline" icon="folder">
                    {t('admin.podcasts.fromFolder')}
                  </Chip>
                </>
              )}
              {podcast.explicit && (
                <>
                  {' '}
                  <Chip tone="warning">{t('admin.podcasts.explicit')}</Chip>
                </>
              )}
            </span>
          </span>
        </span>
      ),
    },
    {
      key: 'episodes',
      header: t('admin.podcasts.columns.episodes'),
      cell: (podcast) => {
        const missing = Math.max(0, podcast.episodeCount - podcast.localEpisodeCount);
        const share =
          podcast.episodeCount > 0 ? (100 * podcast.localEpisodeCount) / podcast.episodeCount : 0;
        const stored =
          podcast.localEpisodeCount === 0
            ? t('admin.podcasts.share.none')
            : missing === 0
              ? t('admin.podcasts.share.all')
              : t(
                  podcast.feedUrl ? 'admin.podcasts.share.some' : 'admin.podcasts.share.someNoFeed',
                  {
                    stored: number.format(podcast.localEpisodeCount),
                    missing: number.format(missing),
                  },
                );
        return (
          <span className={`${styles.stack} ${styles.episodes}`}>
            <span className={styles.numbers}>{number.format(podcast.episodeCount)}</span>
            {podcast.episodeCount > 0 && (
              <ProgressBar
                value={share}
                label={t('admin.podcasts.share.label', {
                  stored: number.format(podcast.localEpisodeCount),
                  total: number.format(podcast.episodeCount),
                })}
              />
            )}
            <span className={`${styles.muted} ${styles.small}`}>{stored}</span>
          </span>
        );
      },
    },
    {
      key: 'stored',
      header: t('admin.podcasts.columns.stored'),
      align: 'end',
      cell: (podcast) => (
        <span className={styles.numbers}>
          {podcast.storedBytes > 0
            ? formatBytes(podcast.storedBytes, language)
            : t('admin.podcasts.noError')}
        </span>
      ),
    },
    {
      key: 'newest',
      header: t('admin.podcasts.columns.newest'),
      cell: (podcast) =>
        podcast.latestEpisodeAt ? (
          <time className={styles.nowrap} dateTime={podcast.latestEpisodeAt}>
            {formatDate(podcast.latestEpisodeAt, language)}
          </time>
        ) : (
          t('admin.podcasts.noError')
        ),
    },
    {
      key: 'state',
      header: t('admin.podcasts.columns.state'),
      cell: (podcast) => {
        const state = podcastRowState(podcast, isFetching(podcast));
        const loading = podcast.downloads.queued + podcast.downloads.running;
        return (
          <span className={styles.stack}>
            <Chip tone={STATE_TONE[state]}>
              {state === 'loading'
                ? t('admin.podcasts.state.loading', { number: number.format(loading) })
                : t(`admin.podcasts.state.${state}`)}
            </Chip>
            {podcast.downloads.failed > 0 && (
              <span className={styles.problem}>
                {t('admin.podcasts.stored.failed', {
                  number: number.format(podcast.downloads.failed),
                })}
              </span>
            )}
          </span>
        );
      },
    },
    {
      key: 'open',
      header: t('admin.podcasts.columns.open'),
      hideHeader: true,
      align: 'end',
      cell: (podcast) => (
        <ButtonLink
          href={paths.adminPodcast(podcast.id)}
          variant="ghost"
          size="sm"
          icon="chevronRight"
          aria-label={t('admin.podcasts.openPage.label', { title: podcast.title })}
        >
          {t('admin.podcasts.openPage.action')}
        </ButtonLink>
      ),
    },
  ];
}
