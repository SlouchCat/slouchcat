import type { AdminPodcast } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { feedError } from './columns';
import styles from './Podcasts.module.css';

/**
 * The fold-out of a podcast (POD-2, POD-3): author, the whole feed address, library and the
 * whole last error; «Jetzt abrufen» for a podcast with a feed and «Abo löschen» for one no folder
 * owns. A podcast a folder still owns comes back with the next scan, so the folder decides and
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
  const { t } = useTranslation();
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
            ) : (
              t('admin.podcasts.detail.noFeed')
            )}
          </dd>
        </div>
        <div>
          <dt>{t('admin.podcasts.detail.library')}</dt>
          <dd>{libraryName}</dd>
        </div>
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
              variant="danger"
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
    </div>
  );
}
