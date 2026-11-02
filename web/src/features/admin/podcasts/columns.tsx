import type { AdminPodcast } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { codeMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { Chip } from '@/ui';
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
 * «Explizit» folds the row out; episodes with the local ones, the last fetch (or «wird
 * abgerufen» after «Jetzt abrufen») and the error, one line long; the whole error, the author,
 * the feed address and the library are in the fold-out.
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
            <Chip tone="danger" icon="warning">
              <span className={styles.clip}>{message}</span>
            </Chip>
          </span>
        );
      },
    },
  ];
}
