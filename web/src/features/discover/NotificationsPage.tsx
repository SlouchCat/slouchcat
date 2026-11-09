import type { Notification } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { episodeCode } from '@/features/catalog/labels';
import { formatDateTime } from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import {
  PAGED_LIST_VIRTUAL_FROM,
  rangeBySize,
  usePagedList,
  type PlaceRange,
} from '@/lib/pagedList';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  InboxEntry,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
  VirtualList,
} from '@/ui';
import { PageRetry } from './ItemGrid';
import { notificationsSource, useMarkAllReadMutation, useMarkReadMutation } from './queries';
import styles from './Discover.module.css';

const BACK_KEY = 'notifications-back';
const READ_ALL_KEY = 'notifications-read-all';
const CHANNELS_KEY = 'notifications-channels';
const entryKey = (entry: Notification) => `notification-${entry.id}`;
/** An entry's expected height in px (InboxEntry: three lines in its padding). */
const ENTRY_HEIGHT = 88;

/** "S01E03 · Pilot" for an episode, the title for a podcast episode. */
function entryTitle(t: TFunction, entry: Notification): string {
  const code = episodeCode(t, entry.item);
  return code ? t('item.nextEpisodeLabel', { code, title: entry.item.title }) : entry.item.title;
}

/** The entry as one spoken line: state, title, context and time. */
function entryLabel(t: TFunction, entry: Notification, language: string): string {
  const values = {
    title: entryTitle(t, entry),
    source: entryContext(t, entry),
    time: formatDateTime(entry.createdAt, language),
  };
  return entry.readAt
    ? t('notifications.entryLabel', values)
    : t('notifications.entryLabelUnread', values);
}

function entryContext(t: TFunction, entry: Notification): string {
  const parent = entry.parent?.title ?? entry.item.seriesTitle;
  return parent ? t('notifications.newEpisode', { parent }) : t('notifications.newEpisodeAlone');
}

/**
 * The in-app inbox (E-8): new episodes of favourite series and podcasts. Opening an entry marks
 * it read; "Alle als gelesen markieren" clears the badge in the header. "Kanäle einstellen" leads
 * to e-mail, Web Push and Telegram (M8).
 */
export function NotificationsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('notifications.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  // Every entry, read page by page as the list asks (concept 18 C-3); a new entry on top changes
  // the total and reads the list again from page 0.
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(notificationsSource, rangeBySize(span));
  const inbox = paged.first;
  const markRead = useMarkReadMutation();
  const markAll = useMarkAllReadMutation();
  const unread = inbox.data?.unread ?? 0;
  const first = paged.slots[0];

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
            {/* E-mail, Web Push and Telegram of the account (M8, A-6); before "Alle als gelesen
                markieren", which goes away once everything is read. */}
            <SpatialButtonLink
              focusKey={CHANNELS_KEY}
              variant="ghost"
              href={paths.notificationSettings}
            >
              {t('settings.notifications.link')}
            </SpatialButtonLink>
            {unread > 0 && (
              <SpatialButton
                focusKey={READ_ALL_KEY}
                variant="secondary"
                icon="check"
                aria-disabled={markAll.isPending || undefined}
                onPress={() => {
                  // The button goes away once the list is read again, before `mutate` would call
                  // back: the focus moves to "Zurück" now, so it never falls to the page body.
                  if (!markAll.isPending) {
                    setSpatialFocus(BACK_KEY);
                    markAll.mutate();
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

        {inbox.isSuccess && paged.total === 0 && (
          <Notice title={t('notifications.empty.title')}>{t('notifications.empty.body')}</Notice>
        )}

        {paged.total > 0 && (
          <section className={styles.section}>
            <VirtualList
              slots={paged.slots}
              virtual={paged.total > PAGED_LIST_VIRTUAL_FROM}
              onRange={setSpan}
              className={styles.inbox}
              label={t('notifications.title')}
              estimate={ENTRY_HEIGHT}
              itemKey={(entry) => entry.id}
              renderPlaceholder={() => <Skeleton shape="line" />}
              renderItem={(entry) => (
                <InboxEntry
                  focusKey={entryKey(entry)}
                  href={paths.item(entry.item.id)}
                  title={entryTitle(t, entry)}
                  context={entryContext(t, entry)}
                  time={formatDateTime(entry.createdAt, i18n.language)}
                  unread={!entry.readAt}
                  label={entryLabel(t, entry, i18n.language)}
                  onOpen={() => {
                    if (!entry.readAt) markRead.mutate(entry.id);
                  }}
                />
              )}
            />
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        )}
      </main>
    </SpatialPage>
  );
}
