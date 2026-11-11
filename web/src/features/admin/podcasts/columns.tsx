import type { AdminPodcast } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { codeMessage } from '@/api/errors';
import { formatBytes, formatDateTime } from '@/lib/format';
import { Chip, Icon } from '@/ui';
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

/**
 * The columns of the podcast table (POD-2, mockup Pod-C): the title with «aus Ordner» and
 * «Explizit» folds the row out; episodes with the local ones, «gespeichert» (concept 19 part A,
 * stage 1: what applies, marked when the podcast has an own setting, the stored bytes and the
 * downloads), the last fetch (or «wird abgerufen» after «Jetzt abrufen») and the error, one line
 * long; the whole error, the author, the feed address and the library are in the fold-out.
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
        <span className={styles.title}>
          <span>{podcast.title}</span>
          {podcast.feedOrigin === 'file' && (
            <Chip tone="outline" icon="folder">
              {t('admin.podcasts.fromFolder')}
            </Chip>
          )}
          {podcast.explicit && <Chip tone="warning">{t('admin.podcasts.explicit')}</Chip>}
        </span>
      ),
    },
    {
      key: 'episodes',
      header: t('admin.podcasts.columns.episodes'),
      align: 'end',
      cell: (podcast) => (
        <span className={styles.stack}>
          <span className={styles.numbers}>{number.format(podcast.episodeCount)}</span>
          {podcast.localEpisodeCount > 0 && (
            <span className={`${styles.muted} ${styles.numbers}`}>
              {t('admin.podcasts.localEpisodes', {
                count: podcast.localEpisodeCount,
                number: number.format(podcast.localEpisodeCount),
              })}
            </span>
          )}
        </span>
      ),
    },
    {
      key: 'stored',
      header: t('admin.podcasts.columns.stored'),
      cell: (podcast) => {
        if (!podcast.effectiveMode) return t('admin.podcasts.noError');
        const loading = podcast.downloads.queued + podcast.downloads.running;
        return (
          <span className={styles.stack}>
            <span className={styles.title}>
              <Chip tone={podcast.effectiveMode === 'save' ? 'success' : 'outline'}>
                {t(`admin.podcasts.stored.${podcast.effectiveMode}`)}
              </Chip>
              {podcast.mode !== undefined && podcast.mode !== 'inherit' && (
                <span className={styles.muted}>{t('admin.podcasts.stored.own')}</span>
              )}
            </span>
            {podcast.storedBytes > 0 && (
              <span className={`${styles.muted} ${styles.numbers}`}>
                {formatBytes(podcast.storedBytes, language)}
              </span>
            )}
            {loading > 0 && (
              <span className={styles.muted}>
                {t('admin.podcasts.stored.loading', { number: number.format(loading) })}
              </span>
            )}
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
      key: 'lastFetch',
      header: t('admin.podcasts.columns.lastFetch'),
      cell: (podcast) => {
        if (isFetching(podcast)) {
          return <Chip tone="info">{t('admin.podcasts.refresh.running')}</Chip>;
        }
        return podcast.lastCheckedAt ? (
          <time className={styles.nowrap} dateTime={podcast.lastCheckedAt}>
            {formatDateTime(podcast.lastCheckedAt, language)}
          </time>
        ) : (
          t('admin.podcasts.never')
        );
      },
    },
    {
      key: 'error',
      header: t('admin.podcasts.columns.error'),
      cell: (podcast) => {
        if (!podcast.lastError) return t('admin.podcasts.noError');
        const { message } = feedError(t, podcast.lastError);
        // One line here (S-5); the whole text and the server's detail are in the fold-out.
        return (
          <span className={styles.errorCell} title={message}>
            <Icon name="warning" size="sm" />
            <span className={styles.clip}>{message}</span>
          </span>
        );
      },
    },
  ];
}
