import type { AdminPodcast } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { formatDateTime } from '@/lib/format';
import { Text } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { feedError } from './columns';
import { PodcastSetting } from './PodcastSetting';
import styles from './Podcasts.module.css';

/**
 * The fold-out of a podcast (POD-2, POD-3): author, the whole feed address, library, the last
 * fetch and the whole last error; for a podcast with a feed its setting of stored episodes and «Fehlende
 * nachladen» (concept 19 part A, stage 1); «Jetzt abrufen» for a podcast with a feed and «Abo
 * löschen» for one no folder owns. A podcast a folder still owns comes back with the next scan, so the folder decides and
 * the fold-out says so instead.
 */
export function PodcastDetail({
  podcast,
  libraryName,
  refreshing,
  onRefresh,
  onRemove,
}: {
  podcast: AdminPodcast;
  libraryName: string;
  refreshing: boolean;
  onRefresh: (podcast: AdminPodcast) => void;
  onRemove: (podcast: AdminPodcast) => void;
}) {
  const { t, i18n } = useTranslation();
  const error = podcast.lastError ? feedError(t, podcast.lastError) : null;
  const ownedByFolder = podcast.rootId !== undefined;
  return (
    <div className={styles.detail}>
      <dl className={styles.facts}>
        {podcast.author && (
          <div>
            <dt>{t('admin.podcasts.detail.author')}</dt>
            <dd>{podcast.author}</dd>
          </div>
        )}
        <div>
          <dt>{t('admin.podcasts.detail.feedUrl')}</dt>
          <dd>
            {podcast.feedUrl ? (
              <span className={styles.url}>{podcast.feedUrl}</span>
            ) : ownedByFolder ? (
              t('admin.podcasts.detail.noFeedFolder')
            ) : (
              t('admin.podcasts.detail.noFeed')
            )}
          </dd>
        </div>
        <div>
          <dt>{t('admin.podcasts.detail.library')}</dt>
          <dd>{libraryName}</dd>
        </div>
        {podcast.feedUrl && (
          <div>
            <dt>{t('admin.podcasts.columns.lastFetch')}</dt>
            <dd>
              {podcast.lastCheckedAt
                ? formatDateTime(podcast.lastCheckedAt, i18n.language)
                : t('admin.podcasts.never')}
            </dd>
          </div>
        )}
      </dl>
      {error && (
        <div className={styles.error}>
          <p>{error.message}</p>
          {error.detail && <p className={styles.url}>{error.detail}</p>}
        </div>
      )}
      {(podcast.feedUrl || !ownedByFolder) && (
        <div className={styles.actions}>
          {/* A folder podcast without feed has nothing to fetch. */}
          {podcast.feedUrl && (
            <AriaButton
              variant="secondary"
              icon="refresh"
              aria-label={t('admin.podcasts.refresh.label', { title: podcast.title })}
              isPending={refreshing}
              onPress={() => onRefresh(podcast)}
            >
              {t('admin.podcasts.refresh.action')}
            </AriaButton>
          )}
          {/* Once the folder is gone (rootId absent), the subscription it left behind may go. */}
          {!ownedByFolder && (
            <AriaButton
              variant="dangerQuiet"
              icon="trash"
              aria-label={t('admin.podcasts.remove.label', { title: podcast.title })}
              onPress={() => onRemove(podcast)}
            >
              {t('admin.podcasts.remove.action')}
            </AriaButton>
          )}
        </div>
      )}
      {ownedByFolder && <Text tone="secondary">{t('admin.podcasts.detail.folderNote')}</Text>}
      {/* After the actions, so the buttons stay first; a folder podcast without feed has nothing
          to load (5.1). */}
      {podcast.effectiveMode && <PodcastSetting podcast={podcast} />}
    </div>
  );
}
