import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { eventText, failText, seasonsText, WishStateChip } from '@/features/wishes/state';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { FactList, Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  afterDialogsClosed,
  afterRender,
  AriaButton,
  ConfirmDialog,
  DataTable,
  focusLost,
  StatusRegion,
  ToggleToolbar,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminWishesQuery,
  approveWish,
  dashboardQuery,
  declineWish,
  retryWish,
  seerrStatusQuery,
  syncWishesNow,
  wishKeys,
  type AdminWish,
  type AdminWishFilter,
} from '../queries';
import { arrErrorText, isStalled, progressText, serviceName, TagChip } from './progress';
import styles from './Wishes.module.css';

/**
 * The filters: the first four are the server's (`state`); «Download verzögert» and «Tag nicht
 * gesetzt» are not, so they filter the loaded page of the open wishes here (at most 100, the
 * contract's limit). Both concern wishes in procurement, which are always open.
 */
const FILTERS = ['all', 'open', 'available', 'closed', 'stalled', 'untagged'] as const;
type Filter = (typeof FILTERS)[number];
const CLIENT_FILTERS: Partial<Record<Filter, (wish: AdminWish) => boolean>> = {
  stalled: (wish) => isStalled(wish.progress),
  untagged: (wish) => wish.requesters.some((requester) => requester.tag?.state === 'failed'),
};
const serverFilter = (filter: Filter): AdminWishFilter =>
  filter in CLIENT_FILTERS ? 'open' : (filter as AdminWishFilter);
const isFilter = (value: unknown): value is Filter =>
  typeof value === 'string' && FILTERS.some((known) => known === value);

/** The events of the administration's own choices (approveWish, declineWish). */
const REVIEW_EVENTS = ['approved_by_admin', 'declined_by_admin'] as const;
const isReviewEvent = (kind: string): kind is (typeof REVIEW_EVENTS)[number] =>
  REVIEW_EVENTS.some((known) => known === kind);

/** «konto@example.org · Lena», the joined ones and those who withdrew themselves marked. */
function requesterLine(t: TFunction, requester: AdminWish['requesters'][number]): string {
  const line = t('admin.wishes.requests.requester', {
    account: requester.accountEmail,
    profile: requester.profileName,
  });
  if (requester.closed) return t('admin.wishes.requests.requesterClosed', { line });
  return requester.role === 'joined' ? t('admin.wishes.requests.requesterJoined', { line }) : line;
}

/** The Seerr requests of a wish: a series is requested per season, its seasons carry them. */
function requestIds(wish: AdminWish): number[] {
  const ids = [
    ...new Set(
      wish.title.media === 'show'
        ? wish.seasons.flatMap((season) =>
            season.seerrRequestId === undefined ? [] : [season.seerrRequestId],
          )
        : [],
    ),
  ];
  if (ids.length === 0 && wish.seerrRequestId !== undefined) ids.push(wish.seerrRequestId);
  return ids;
}

/** The Seerr user the wish went out as: its name, else its id. */
function seerrUserText(t: TFunction, wish: AdminWish): string | undefined {
  if (wish.seerrUserName) return wish.seerrUserName;
  return wish.seerrUserId === undefined
    ? undefined
    : t('admin.wishes.requests.seerrUserId', { id: wish.seerrUserId });
}

type Review = { wish: AdminWish; approve: boolean };

function requestColumns(
  t: TFunction,
  language: string,
  actions: (wish: AdminWish) => ReactNode,
): DataColumn<AdminWish>[] {
  return [
    {
      // The kind, or the seasons of a series, below the title: one column fewer, so the table
      // fits beside the sidebar at desktop width.
      key: 'title',
      header: t('admin.wishes.requests.columns.title'),
      rowHeader: true,
      cell: (wish) => (
        <span className={styles.factStack}>
          <span>
            {wish.title.year
              ? t('admin.wishes.requests.titleYear', {
                  title: wish.title.title,
                  year: wish.title.year,
                })
              : wish.title.title}
          </span>
          <span className={styles.subline}>
            {wish.title.media === 'show' ? seasonsText(t, wish.seasons) : t('wishes.kind.movie')}
          </span>
        </span>
      ),
    },
    {
      // Each requester with the tag of its profile in Radarr or Sonarr below (stage 4).
      key: 'requesters',
      header: t('admin.wishes.requests.columns.requesters'),
      cell: (wish) =>
        wish.requesters.length === 0 ? (
          <span className={styles.muted}>{t('admin.wishes.requests.noRequester')}</span>
        ) : (
          <span className={styles.factStack}>
            {wish.requesters.map((requester) => (
              <span key={requester.profileId} className={styles.factStack}>
                <span className={styles.clip}>{requesterLine(t, requester)}</span>
                {requester.tag && (
                  <span>
                    <TagChip tag={requester.tag} />
                  </span>
                )}
              </span>
            ))}
          </span>
        ),
    },
    {
      // The Seerr user the wish went out as, and its requests there.
      key: 'seerr',
      header: t('admin.wishes.requests.columns.seerr'),
      cell: (wish) => {
        const ids = requestIds(wish);
        const user = seerrUserText(t, wish);
        return (
          <span className={styles.factStack}>
            {user && <span className={styles.clip}>{user}</span>}
            {ids.length === 0 ? (
              <span className={styles.muted}>{t('admin.wishes.requests.noRequest')}</span>
            ) : (
              <span className={styles.secondary}>
                {t('admin.wishes.requests.requestIds', {
                  count: ids.length,
                  ids: ids.join(t('format.listSeparator')),
                })}
              </span>
            )}
          </span>
        );
      },
    },
    {
      // The state, the download progress of a wish in procurement and the time of the last change.
      key: 'state',
      header: t('admin.wishes.requests.columns.state'),
      cell: (wish) => {
        const progress = progressText(t, wish.progress, language);
        return (
          <span className={styles.factStack}>
            <span>
              <WishStateChip state={wish.state} detail={wish.stateDetail} />
            </span>
            {progress && <span>{progress}</span>}
            <time className={styles.secondary} dateTime={wish.updatedAt}>
              {formatDateTime(wish.updatedAt, language)}
            </time>
          </span>
        );
      },
    },
    {
      key: 'attempts',
      header: t('admin.wishes.requests.columns.attempts'),
      align: 'end',
      cell: (wish) => wish.attempts,
    },
    {
      key: 'actions',
      header: t('admin.wishes.requests.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: actions,
    },
  ];
}

/** «Freigeben» and «Ablehnen» of a wish that waits for an approval in Seerr (F-16). */
function ReviewActions({
  wish,
  pending,
  onReview,
}: {
  wish: AdminWish;
  pending: Review | undefined;
  onReview: (review: Review) => void;
}) {
  const { t } = useTranslation();
  if (wish.state !== 'pending') return null;
  const title = wish.title.title;
  // The pressed button shows that it works and keeps the focus; the other one waits.
  const busy = pending?.wish.id === wish.id;
  return (
    <span className={styles.rowActions}>
      <AriaButton
        variant="primary"
        size="sm"
        icon="check"
        isPending={busy && pending.approve}
        isDisabled={busy && !pending.approve}
        aria-label={t('admin.wishes.requests.approveLabel', { title })}
        onPress={() => onReview({ wish, approve: true })}
      >
        {t('admin.wishes.requests.approve')}
      </AriaButton>
      <ConfirmDialog
        trigger={
          <AriaButton
            variant="dangerQuiet"
            size="sm"
            icon="close"
            isPending={busy && !pending.approve}
            isDisabled={busy && pending.approve}
            aria-label={t('admin.wishes.requests.declineLabel', { title })}
          >
            {t('admin.wishes.requests.decline')}
          </AriaButton>
        }
        title={t('admin.wishes.requests.declineTitle', { title })}
        body={t('admin.wishes.requests.declineBody')}
        confirmLabel={t('admin.wishes.requests.decline')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => onReview({ wish, approve: false })}
      />
    </span>
  );
}

/** The progress block of the fold-out: service, stage, last read and the service's message. */
function ProgressBlock({ wish }: { wish: AdminWish }) {
  const { t, i18n } = useTranslation();
  const progress = wish.progress;
  if (!progress) return null;
  const language = intlLocale(i18n.language);
  const service = serviceName(t, progress.service);
  const facts: (readonly [string, ReactNode])[] = [
    [t('admin.wishes.requests.progressBlock.service'), service],
    [
      t('admin.wishes.requests.progressBlock.state'),
      progressText(t, progress, language) ?? t('admin.wishes.requests.progressBlock.unknown'),
    ],
    [
      t('admin.wishes.requests.progressBlock.readAt'),
      progress.readAt
        ? formatDateTime(progress.readAt, language)
        : t('admin.wishes.requests.progressBlock.never'),
    ],
    ...(progress.error === 'incomplete'
      ? ([
          [
            t('admin.wishes.requests.progressBlock.note'),
            t('admin.wishes.requests.progress.incomplete'),
          ],
        ] as const)
      : []),
    ...(progress.message
      ? ([
          [t('admin.wishes.requests.progressBlock.message', { service }), progress.message],
        ] as const)
      : []),
  ];
  return (
    <section className={styles.block} aria-labelledby={`wish-${wish.id}-progress`}>
      <h3 id={`wish-${wish.id}-progress`} className={styles.blockTitle}>
        {t('admin.wishes.requests.progressBlock.title')}
      </h3>
      <FactList facts={facts} />
    </section>
  );
}

/** The profile tags of the requesters: state, when it was set, or why not (N-27: none for Seerr's). */
function TagsBlock({ wish }: { wish: AdminWish }) {
  const { t, i18n } = useTranslation();
  const tagged = wish.requesters.filter((requester) => requester.tag);
  if (tagged.length === 0) return null;
  const language = intlLocale(i18n.language);
  const facts = tagged.flatMap((requester) => {
    const tag = requester.tag;
    if (!tag) return [];
    const service = serviceName(t, tag.service);
    const detail =
      tag.state === 'set'
        ? tag.setAt
          ? t('admin.wishes.requests.tags.setAt', {
              service,
              time: formatDateTime(tag.setAt, language),
            })
          : t('admin.wishes.requests.tags.setIn', { service })
        : tag.state === 'pending'
          ? t('admin.wishes.requests.tags.pendingHelp', { service })
          : tag.lastError
            ? arrErrorText(t, tag.lastError, tag.service)
            : t('admin.wishes.requests.tags.failedHelp', { service });
    return [
      [
        requesterLine(t, requester),
        <span key={requester.profileId} className={styles.chips}>
          <TagChip tag={tag} />
          <span>{detail}</span>
        </span>,
      ] as const,
    ];
  });
  return (
    <section className={styles.block} aria-labelledby={`wish-${wish.id}-tags`}>
      <h3 id={`wish-${wish.id}-tags`} className={styles.blockTitle}>
        {t('admin.wishes.requests.tags.title')}
      </h3>
      <FactList facts={facts} />
    </section>
  );
}

/**
 * The fold-out of a wish: what happened (newest first), why it failed, who sent it as which
 * Seerr user, the download progress with the message of Radarr or Sonarr (only here, N-83), the
 * profile tags, and the actions. «Erneut versuchen» for a failed wish (not one whose account is
 * gone); «Freigeben» and «Ablehnen» stand in the row.
 */
function RequestDetail({
  wish,
  onRetry,
  retrying,
  failure,
}: {
  wish: AdminWish;
  onRetry: (wish: AdminWish) => void;
  retrying: boolean;
  /** Why the last action on this wish was refused (409 `wish_not_retryable`, `wish_not_pending`, …). */
  failure: unknown;
}) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const user = seerrUserText(t, wish);
  const facts: (readonly [string, ReactNode])[] = [
    [t('admin.wishes.requests.facts.created'), formatDateTime(wish.createdAt, language)],
    [t('admin.wishes.requests.facts.origin'), t(`admin.wishes.requests.origin.${wish.origin}`)],
    ...(user !== undefined
      ? ([
          [
            t('admin.wishes.requests.facts.seerrUser'),
            wish.seerrUserName && wish.seerrUserId !== undefined
              ? t('admin.wishes.requests.seerrUserFull', {
                  name: wish.seerrUserName,
                  id: wish.seerrUserId,
                })
              : user,
          ],
        ] as const)
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
      {failure ? <Notice tone="danger">{problemMessage(t, failure)}</Notice> : null}
      <section className={styles.block} aria-label={t('admin.wishes.requests.facts.label')}>
        <FactList facts={facts} />
      </section>
      <ProgressBlock wish={wish} />
      <TagsBlock wish={wish} />
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
                <span>
                  {isReviewEvent(event.kind)
                    ? t(`admin.wishes.requests.event.${event.kind}`)
                    : eventText(t, event)}
                </span>
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
 * first, with its requesters (account · profile) and their profile tags, the Seerr user and
 * requests, the state with the download progress and the attempts of the outbox; «Freigeben» and
 * «Ablehnen» for a wish that waits in Seerr (passed on to Seerr, F-16; «Ablehnen» asks first). A
 * row folds out into its history, progress, tags and «Erneut versuchen»; a refused action opens
 * its row with the reason. The filter counts every wish whatever the filter; «Jetzt abgleichen»
 * queues the sync with Seerr. `?item=stalled` or `?item=untagged` (the overview's hints) start
 * with that filter.
 */
export function RequestsTab() {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const search: Record<string, unknown> = useSearch({ strict: false });
  const [filter, setFilter] = useState<Filter>(() => (isFilter(search.item) ? search.item : 'all'));
  const list = useQuery(adminWishesQuery(serverFilter(filter)));
  // The open wishes, for the counts of the two filters of the client.
  const open = useQuery(adminWishesQuery('open'));
  const status = useQuery(seerrStatusQuery);
  const [message, say] = useStatusMessage();
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const body = useRef<HTMLDivElement>(null);
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
  const openRow = (id: number) => setExpanded((current) => new Set(current).add(String(id)));
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
    onMutate: () => {
      say(null);
      sync.reset();
      review.reset();
    },
    onSuccess: async (_, wish) => {
      say(t('admin.wishes.requests.retried', { title: wish.title.title }));
      await refresh();
    },
  });
  const review = useMutation({
    mutationFn: ({ wish, approve }: Review) =>
      approve ? approveWish(wish.id) : declineWish(wish.id),
    onMutate: () => {
      say(null);
      sync.reset();
      retry.reset();
    },
    onSuccess: async (_, { wish, approve }) => {
      say(
        t(approve ? 'admin.wishes.requests.approved' : 'admin.wishes.requests.declined', {
          title: wish.title.title,
        }),
      );
      await refresh();
      // The buttons went with the pending state: the focus goes to the row's toggle.
      await afterDialogsClosed();
      await afterRender();
      if (focusLost()) {
        body.current
          ?.querySelector<HTMLElement>(`tr[data-row-key="${wish.id}"] [aria-expanded]`)
          ?.focus();
      }
    },
    // The reason stands at the row: its fold-out opens with it (design rule 1.3).
    onError: (_, { wish }) => openRow(wish.id),
  });
  const pendingReview = review.isPending ? review.variables : undefined;
  const startReview = review.mutate;
  const columns = useMemo(
    () =>
      requestColumns(t, language, (wish) => (
        <ReviewActions wish={wish} pending={pendingReview} onReview={(next) => startReview(next)} />
      )),
    [t, language, pendingReview, startReview],
  );
  const counts = list.data?.counts;
  const countOf = (value: Filter) => {
    const only = CLIENT_FILTERS[value];
    if (only) return open.data?.items.filter(only).length;
    if (!counts) return undefined;
    if (value === 'all') return counts.open + counts.available + counts.closed;
    return counts[value as Exclude<AdminWishFilter, 'all'>];
  };
  const only = CLIENT_FILTERS[filter];
  const rows = list.data ? (only ? list.data.items.filter(only) : list.data.items) : undefined;
  const queued = status.data?.counts.queued ?? 0;
  // A refused action shows at its wish's row; the page's line is the sync's.
  const failure = sync.error;
  const failureOf = (wish: AdminWish) => {
    if (review.variables?.wish.id === wish.id && review.error) return review.error;
    return retry.variables?.id === wish.id ? retry.error : null;
  };

  return (
    <div className={styles.tabBody} ref={body}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.requests')}</VisuallyHidden>
      <div className={styles.bar}>
        <ToggleToolbar
          label={t('admin.wishes.requests.filter.label')}
          value={filter}
          onChange={(value) => setFilter(isFilter(value) ? value : 'all')}
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
      {rows && rows.length === 0 && (
        <Text tone="secondary">
          {t(
            filter === 'all'
              ? 'admin.wishes.requests.empty'
              : filter === 'stalled'
                ? 'admin.wishes.requests.emptyStalled'
                : filter === 'untagged'
                  ? 'admin.wishes.requests.emptyUntagged'
                  : 'admin.wishes.requests.emptyFilter',
          )}
        </Text>
      )}
      {rows && rows.length > 0 && (
        <div data-perf="first-work-row">
          <DataTable
            caption={t('admin.wishes.requests.caption')}
            columns={columns}
            rows={rows}
            rowKey={(wish) => String(wish.id)}
            expand={{
              expandedKeys: expanded,
              onExpandedChange: (key, isOpen) =>
                setExpanded((current) => {
                  const next = new Set(current);
                  if (isOpen) next.add(key);
                  else next.delete(key);
                  return next;
                }),
              render: (wish) => (
                <RequestDetail
                  wish={wish}
                  retrying={retry.isPending && retry.variables?.id === wish.id}
                  failure={failureOf(wish)}
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
