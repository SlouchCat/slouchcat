import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { eventText, failText, seasonsText, WishStateChip } from '@/features/wishes/state';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { FactList, Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  DataTable,
  StatusRegion,
  ToggleToolbar,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminWishesQuery,
  dashboardQuery,
  retryWish,
  seerrStatusQuery,
  syncWishesNow,
  wishKeys,
  type AdminWish,
  type AdminWishFilter,
} from '../queries';
import styles from './Wishes.module.css';

const FILTERS = ['all', 'open', 'available', 'closed'] as const;

/** «konto@example.org · Lena», the joined ones and those who withdrew themselves marked. */
function requesterLines(t: TFunction, wish: AdminWish): string[] {
  return wish.requesters.map((requester) => {
    const line = t('admin.wishes.requests.requester', {
      account: requester.accountEmail,
      profile: requester.profileName,
    });
    if (requester.closed) return t('admin.wishes.requests.requesterClosed', { line });
    return requester.role === 'joined'
      ? t('admin.wishes.requests.requesterJoined', { line })
      : line;
  });
}

function requestColumns(t: TFunction, language: string): DataColumn<AdminWish>[] {
  return [
    {
      key: 'title',
      header: t('admin.wishes.requests.columns.title'),
      rowHeader: true,
      cell: (wish) =>
        wish.title.year
          ? t('admin.wishes.requests.titleYear', { title: wish.title.title, year: wish.title.year })
          : wish.title.title,
    },
    {
      key: 'requesters',
      header: t('admin.wishes.requests.columns.requesters'),
      cell: (wish) =>
        wish.requesters.length === 0 ? (
          <span className={styles.muted}>{t('admin.wishes.requests.noRequester')}</span>
        ) : (
          <span className={styles.factStack}>
            {requesterLines(t, wish).map((line) => (
              <span key={line} className={styles.clip}>
                {line}
              </span>
            ))}
          </span>
        ),
    },
    {
      // The state with the time of its last change below: one column fewer, so the table fits
      // beside the sidebar at desktop width.
      key: 'state',
      header: t('admin.wishes.requests.columns.state'),
      cell: (wish) => (
        <span className={styles.factStack}>
          <span>
            <WishStateChip state={wish.state} detail={wish.stateDetail} />
          </span>
          <time className={styles.secondary} dateTime={wish.updatedAt}>
            {formatDateTime(wish.updatedAt, language)}
          </time>
        </span>
      ),
    },
    {
      key: 'seasons',
      header: t('admin.wishes.requests.columns.seasons'),
      cell: (wish) =>
        wish.title.media === 'show' ? (
          seasonsText(t, wish.seasons)
        ) : (
          <span className={styles.muted}>{t('wishes.kind.movie')}</span>
        ),
    },
    {
      key: 'seerr',
      header: t('admin.wishes.requests.columns.seerr'),
      cell: (wish) =>
        wish.seerrRequestId === undefined ? (
          <span className={styles.muted}>{t('admin.wishes.requests.noRequest')}</span>
        ) : (
          <span className={styles.nowrap}>
            {t('admin.wishes.requests.requestId', { id: wish.seerrRequestId })}
          </span>
        ),
    },
    {
      key: 'attempts',
      header: t('admin.wishes.requests.columns.attempts'),
      align: 'end',
      cell: (wish) => wish.attempts,
    },
  ];
}

/**
 * The fold-out of a wish: what happened (newest first), why it failed, who sent it as which
 * Seerr user, and the actions. «Erneut versuchen» for a failed wish (not one whose account is
 * gone); approve, decline, tags and progress come in later steps beside it.
 */
function RequestDetail({
  wish,
  onRetry,
  retrying,
}: {
  wish: AdminWish;
  onRetry: (wish: AdminWish) => void;
  retrying: boolean;
}) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const facts: (readonly [string, ReactNode])[] = [
    [t('admin.wishes.requests.facts.created'), formatDateTime(wish.createdAt, language)],
    [t('admin.wishes.requests.facts.origin'), t(`admin.wishes.requests.origin.${wish.origin}`)],
    ...(wish.seerrUserId !== undefined
      ? ([[t('admin.wishes.requests.facts.seerrUser'), String(wish.seerrUserId)]] as const)
      : []),
    ...(wish.failReason
      ? ([[t('admin.wishes.requests.facts.failReason'), failText(t, wish.failReason)]] as const)
      : []),
    ...(wish.title.itemId
      ? ([
          [
            t('admin.wishes.requests.facts.library'),
            <TextLink key="item" href={paths.adminItem(wish.title.itemId)}>
              {t('admin.wishes.requests.toItem')}
            </TextLink>,
          ],
        ] as const)
      : []),
  ];
  const canRetry = wish.state === 'failed' && wish.failReason !== 'account_deleted';
  return (
    <div className={styles.detail}>
      <section className={styles.block} aria-label={t('admin.wishes.requests.facts.label')}>
        <FactList facts={facts} />
      </section>
      <section className={styles.block} aria-labelledby={`wish-${wish.id}-history`}>
        <h3 id={`wish-${wish.id}-history`} className={styles.blockTitle}>
          {t('admin.wishes.requests.history')}
        </h3>
        {wish.events.length === 0 ? (
          <Text tone="secondary">{t('admin.wishes.requests.noHistory')}</Text>
        ) : (
          <ol className={styles.events}>
            {wish.events.map((event, index) => (
              <li key={`${event.at}-${index}`} className={styles.event}>
                <time className={styles.secondary} dateTime={event.at}>
                  {formatDateTime(event.at, language)}
                </time>
                <span>{eventText(t, event)}</span>
              </li>
            ))}
          </ol>
        )}
      </section>
      {canRetry && (
        <div className={styles.actions}>
          <AriaButton
            icon="refresh"
            isPending={retrying}
            aria-label={t('admin.wishes.requests.retryLabel', { title: wish.title.title })}
            onPress={() => onRetry(wish)}
          >
            {t('admin.wishes.requests.retry')}
          </AriaButton>
        </div>
      )}
    </div>
  );
}

/**
 * Medien › Wünsche › Anfragen (concept 25, M-23): every wish of the household, newest change
 * first, with its requesters (account · profile), state, seasons, Seerr's request and the
 * attempts of the outbox; a row folds out into its history and its actions. The filter counts
 * every wish whatever the filter; «Jetzt abgleichen» queues the sync with Seerr.
 */
export function RequestsTab() {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const [filter, setFilter] = useState<AdminWishFilter>('all');
  const list = useQuery(adminWishesQuery(filter));
  const status = useQuery(seerrStatusQuery);
  const [message, say] = useStatusMessage();
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
  const sync = useMutation({
    mutationFn: syncWishesNow,
    onMutate: () => say(null),
    onSuccess: async () => {
      say(t('admin.wishes.requests.syncQueued'));
      await refresh();
    },
  });
  const retry = useMutation({
    mutationFn: (wish: AdminWish) => retryWish(wish.id),
    onMutate: () => say(null),
    onSuccess: async (_, wish) => {
      say(t('admin.wishes.requests.retried', { title: wish.title.title }));
      await refresh();
    },
  });
  const columns = useMemo(() => requestColumns(t, language), [t, language]);
  const counts = list.data?.counts;
  const countOf = (value: (typeof FILTERS)[number]) => {
    if (!counts) return undefined;
    if (value === 'all') return counts.open + counts.available + counts.closed;
    return counts[value];
  };
  const queued = status.data?.counts.queued ?? 0;
  const failure = sync.error ?? retry.error;

  return (
    <div className={styles.tabBody}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.requests')}</VisuallyHidden>
      <div className={styles.bar}>
        <ToggleToolbar
          label={t('admin.wishes.requests.filter.label')}
          value={filter}
          onChange={(value) => setFilter(value as AdminWishFilter)}
          options={FILTERS.map((value) => {
            const count = countOf(value);
            return {
              value,
              label: t(`admin.wishes.requests.filter.${value}`),
              count: count === undefined ? '–' : number.format(count),
              ...(count === undefined
                ? {}
                : { countLabel: t('admin.wishes.requests.filter.count', { count }) }),
            };
          })}
        />
        <AriaButton icon="refresh" isPending={sync.isPending} onPress={() => sync.mutate()}>
          {t('admin.wishes.requests.sync')}
        </AriaButton>
      </div>
      {queued > 0 && (
        <Text tone="secondary">{t('admin.wishes.requests.queued', { count: queued })}</Text>
      )}
      {failure ? (
        <Notice tone="danger">{problemMessage(t, failure)}</Notice>
      ) : (
        <StatusRegion message={message} />
      )}
      {list.isPending && <Skeleton shape="block" />}
      {list.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, list.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void list.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {list.data && list.data.items.length === 0 && (
        <Text tone="secondary">
          {t(
            filter === 'all' ? 'admin.wishes.requests.empty' : 'admin.wishes.requests.emptyFilter',
          )}
        </Text>
      )}
      {list.data && list.data.items.length > 0 && (
        <div data-perf="first-work-row">
          <DataTable
            caption={t('admin.wishes.requests.caption')}
            columns={columns}
            rows={list.data.items}
            rowKey={(wish) => String(wish.id)}
            expand={{
              render: (wish) => (
                <RequestDetail
                  wish={wish}
                  retrying={retry.isPending && retry.variables?.id === wish.id}
                  onRetry={(entry) => retry.mutate(entry)}
                />
              ),
            }}
          />
        </div>
      )}
    </div>
  );
}
