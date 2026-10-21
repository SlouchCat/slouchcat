import type { Notification } from '@slouchcat/api-client';
import { useInfiniteQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { episodeCode } from '@/features/catalog/labels';
import { formatDateTime } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  InboxEntry,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
} from '@/ui';
import { MoreButton } from './ItemGrid';
import { notificationsPageQuery, useMarkAllReadMutation, useMarkReadMutation } from './queries';
import styles from './Discover.module.css';

const BACK_KEY = 'notifications-back';
const READ_ALL_KEY = 'notifications-read-all';
const entryKey = (entry: Notification) => `notification-${entry.id}`;

/** "S01E03 · Pilot" for an episode, the title for a podcast episode. */
function entryTitle(t: TFunction, entry: Notification): string {
  const code = episodeCode(t, entry.item);
  return code ? t('item.nextEpisodeLabel', { code, title: entry.item.title }) : entry.item.title;
}

function entryContext(t: TFunction, entry: Notification): string {
  const parent = entry.parent?.title ?? entry.item.seriesTitle;
  return parent ? t('notifications.newEpisode', { parent }) : t('notifications.newEpisodeAlone');
}

/**
 * The in-app inbox (E-8): new episodes of favourite series and podcasts. Opening an entry marks
 * it read; "Alle als gelesen markieren" clears the badge in the header.
 */
export function NotificationsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('notifications.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const inbox = useInfiniteQuery(notificationsPageQuery);
  const markRead = useMarkReadMutation();
  const markAll = useMarkAllReadMutation();
  const entries = useMemo(
    () => inbox.data?.pages.flatMap((page) => page.notifications) ?? [],
    [inbox.data],
  );
  const unread = inbox.data?.pages[0]?.unread ?? 0;
  const first = entries[0];

  return (
    <SpatialPage defaultFocusKey={first ? entryKey(first) : BACK_KEY}>
      <main className={styles.page} aria-busy={inbox.isPending || undefined}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('notifications.title')}</h1>
          {inbox.isSuccess && (
            <p className={styles.muted}>{t('notifications.summary', { count: unread })}</p>
          )}
          <SpatialGroup className={styles.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
            {unread > 0 && (
              <SpatialButton
                focusKey={READ_ALL_KEY}
                variant="secondary"
                icon="check"
                aria-disabled={markAll.isPending || undefined}
                onPress={() => {
                  // The button goes away with the last unread entry: the focus moves back first.
                  if (!markAll.isPending) {
                    markAll.mutate(undefined, { onSuccess: () => setSpatialFocus(BACK_KEY) });
                  }
                }}
              >
                {t('notifications.markAllRead')}
              </SpatialButton>
            )}
          </SpatialGroup>
        </div>

        {inbox.isPending && <Skeleton shape="block" />}
        {inbox.isError && !inbox.data && (
          <Notice
            tone="danger"
            title={problemMessage(t, inbox.error)}
            actions={
              <Button icon="refresh" onPress={() => void inbox.refetch()}>
                {t('common.retry')}
              </Button>
            }
          />
        )}
        {markAll.isError && <Notice tone="danger" title={problemMessage(t, markAll.error)} />}

        {inbox.isSuccess && entries.length === 0 && (
          <Notice title={t('notifications.empty.title')}>{t('notifications.empty.body')}</Notice>
        )}

        {entries.length > 0 && (
          <section className={styles.section}>
            <SpatialGroup as="ul" className={styles.inbox} aria-label={t('notifications.title')}>
              {entries.map((entry) => (
                <li key={entry.id}>
                  <InboxEntry
                    focusKey={entryKey(entry)}
                    href={paths.item(entry.item.id)}
                    title={entryTitle(t, entry)}
                    context={entryContext(t, entry)}
                    time={formatDateTime(entry.createdAt, i18n.language)}
                    unread={!entry.readAt}
                    unreadLabel={t('notifications.unread')}
                    onOpen={() => {
                      if (!entry.readAt) markRead.mutate(entry.id);
                    }}
                  />
                </li>
              ))}
            </SpatialGroup>
            <MoreButton
              shown={entries.length}
              keys={entries.map(entryKey)}
              hasMore={inbox.hasNextPage}
              loading={inbox.isFetchingNextPage}
              failed={inbox.isFetchNextPageError}
              label={t('notifications.more')}
              focusKey="notifications-more"
              onMore={() => void inbox.fetchNextPage()}
            />
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
