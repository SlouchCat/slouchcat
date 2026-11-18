import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { eventText, failText, seasonsText, WishStateChip } from '@/features/wishes/state';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { ButtonLink, FactList, Notice, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  afterDialogsClosed,
  afterRender,
  AriaButton,
  ConfirmDialog,
  DataTable,
  focusLost,
  StatusRegion,
  ToggleToolbar,
  useSearchSelection,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminWishesQuery,
  approveWish,
  dashboardQuery,
  declineWish,
  retryWish,
  seerrKeys,
  seerrSettingsQuery,
  seerrStatusQuery,
  wishKeys,
  type AdminWish,
  type AdminWishFilter,
  type AdminWishPage,
} from '../queries';
import { arrErrorText, isStalled, progressText, serviceName, TagChip } from './progress';
import { useSyncWishes } from './useSyncWishes';
import styles from './Wishes.module.css';

/**
 * The filters: the first four are the server's (`state`); «Download verzögert» and «Tag nicht
 * gesetzt» are not, so they filter the loaded pages of the open wishes here and say so while not
 * every open wish is loaded. Both concern wishes in procurement, which are always open.
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

/** The server reads the progress once a minute (N-85): the list follows while a row shows one. */
const PROGRESS_POLL_MS = 60_000;
const showsProgress = (pages: readonly AdminWishPage[] | undefined) =>
  pages?.some((page) => page.items.some((wish) => wish.progress?.stage !== undefined)) ?? false;

/** The wishes of the loaded pages, each once (the list may move between two pages). */
function wishesOf(pages: readonly AdminWishPage[] | undefined): AdminWish[] {
  const seen = new Set<number>();
  return (pages ?? [])
    .flatMap((page) => page.items)
    .filter((wish) => !seen.has(wish.id) && Boolean(seen.add(wish.id)));
}

/**
 * Seerr's page of the title («In Seerr öffnen»): the saved address of the connection, which only
 * the administration sets, and Seerr's path of a film or series. Nothing unless the address is
 * an `http:` or `https:` one.
 */
export function seerrTitleHref(
  address: string | undefined,
  title: AdminWish['title'],
): string | undefined {
  if (!address) return undefined;
  let base: URL;
  try {
    base = new URL(address);
  } catch {
    return undefined;
  }
  if (base.protocol !== 'http:' && base.protocol !== 'https:') return undefined;
  // A user name and password in the stored address stay out of the page.
  base.username = '';
  base.password = '';
  const root = base.href.endsWith('/') ? base.href : `${base.href}/`;
  return new URL(`${title.media === 'show' ? 'tv' : 'movie'}/${title.tmdbId}`, root).href;
}

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

/** What the administration does to a wish; each wish has its own, so two rows do not mix. */
type Action = 'approve' | 'decline' | 'retry';

const ACTION_DONE = {
  approve: 'admin.wishes.requests.approved',
  decline: 'admin.wishes.requests.declined',
  retry: 'admin.wishes.requests.retried',
} as const satisfies Record<Action, string>;

function without<T>(map: ReadonlyMap<number, T>, id: number): ReadonlyMap<number, T> {
  if (!map.has(id)) return map;
  const next = new Map(map);
  next.delete(id);
  return next;
}

/** A request of the wish waits for an approval in Seerr: the film's, or one of a season. */
const hasPending = (wish: AdminWish) =>
  wish.state === 'pending' || wish.seasons.some((season) => season.state === 'pending');

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

/**
 * «Freigeben» and «Ablehnen» of a wish that waits for an approval in Seerr (F-16). «Ablehnen»
 * asks first; its question lives in the page, so a fresh list cannot take it away.
 */
function ReviewActions({
  wish,
  acting,
  onApprove,
  onDecline,
}: {
  wish: AdminWish;
  /** What runs for this wish right now. */
  acting: Action | undefined;
  onApprove: (wish: AdminWish) => void;
  onDecline: (wish: AdminWish) => void;
}) {
  const { t } = useTranslation();
  // A series in procurement can still have a season that waits for an approval.
  if (!hasPending(wish)) return null;
  const title = wish.title.title;
  // The pressed button shows that it works and keeps the focus; the other one waits.
  return (
    <span className={styles.rowActions}>
      <AriaButton
        variant="primary"
        size="sm"
        icon="check"
        isPending={acting === 'approve'}
        isDisabled={acting !== undefined && acting !== 'approve'}
        aria-label={t('admin.wishes.requests.approveLabel', { title })}
        onPress={() => onApprove(wish)}
      >
        {t('admin.wishes.requests.approve')}
      </AriaButton>
      <AriaButton
        variant="dangerQuiet"
        size="sm"
        icon="close"
        isPending={acting === 'decline'}
        isDisabled={acting !== undefined && acting !== 'decline'}
        aria-label={t('admin.wishes.requests.declineLabel', { title })}
        onPress={() => onDecline(wish)}
      >
        {t('admin.wishes.requests.decline')}
      </AriaButton>
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
  seerrHref,
}: {
  wish: AdminWish;
  onRetry: (wish: AdminWish) => void;
  retrying: boolean;
  /** Seerr's page of the title, when the saved address allows one. */
  seerrHref: string | undefined;
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
      {(canRetry || seerrHref || wish.title.itemId) && (
        <div className={styles.actions}>
          {canRetry && (
            <AriaButton
              icon="refresh"
              isPending={retrying}
              aria-label={t('admin.wishes.requests.retryLabel', { title: wish.title.title })}
              onPress={() => onRetry(wish)}
            >
              {t('admin.wishes.requests.retry')}
            </AriaButton>
          )}
          {wish.title.itemId && (
            <ButtonLink
              href={paths.adminItem(wish.title.itemId)}
              aria-label={t('admin.wishes.requests.toItemLabel', { title: wish.title.title })}
            >
              {t('admin.wishes.requests.toItem')}
            </ButtonLink>
          )}
          {seerrHref && (
            <ButtonLink
              href={seerrHref}
              icon="external"
              target="_blank"
              rel="noopener noreferrer"
              aria-label={t('admin.wishes.requests.inSeerrLabel', { title: wish.title.title })}
            >
              {t('admin.wishes.requests.inSeerr')}
            </ButtonLink>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Medien › Wünsche › Anfragen (concept 25, M-23): every wish of the household, newest change
 * first, page by page («Weitere laden»), with its requesters (account · profile) and their profile
 * tags, the Seerr user and requests, the state with the download progress (read again every
 * minute while a row shows one) and the attempts of the outbox; «Freigeben» and «Ablehnen» for a
 * wish that waits in Seerr (passed on to Seerr, F-16; «Ablehnen» asks first). A row folds out into
 * its history, progress, tags, «Erneut versuchen», «Zum Titel» and «In Seerr öffnen»; a refused
 * action opens its row with the reason and reads the list again. The filter counts every wish
 * whatever the filter; «Jetzt abgleichen» queues the sync with Seerr. The filter is `?item=`, so
 * the overview's hints (`stalled`, `untagged`) set it, also while the tab is open.
 */
export function RequestsTab() {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const [selected, selectFilter] = useSearchSelection('item', FILTERS, 'all');
  const filter = isFilter(selected) ? selected : 'all';
  const list = useInfiniteQuery({
    ...adminWishesQuery(serverFilter(filter)),
    refetchInterval: (query) => (showsProgress(query.state.data?.pages) ? PROGRESS_POLL_MS : false),
  });
  // The open wishes, for the counts of the two filters of the client.
  const open = useInfiniteQuery(adminWishesQuery('open'));
  const status = useQuery(seerrStatusQuery);
  const seerr = useQuery(seerrSettingsQuery);
  const [message, say] = useStatusMessage();
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const [acting, setActing] = useState<ReadonlyMap<number, Action>>(() => new Map());
  const [failures, setFailures] = useState<ReadonlyMap<number, unknown>>(() => new Map());
  // The wish whose «Ablehnen» asks, while the question is open.
  const [declining, setDeclining] = useState<AdminWish | null>(null);
  const body = useRef<HTMLDivElement>(null);
  const shownRef = useRef<HTMLParagraphElement>(null);
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      // Seerr's counters, the queue of the line below the filter among them.
      queryClient.invalidateQueries({ queryKey: seerrKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
  const openRow = (id: number) => setExpanded((current) => new Set(current).add(String(id)));
  const sync = useSyncWishes({
    onMutate: () => say(null),
    onQueued: () => say(t('admin.wishes.requests.syncQueued')),
  });
  const { mutateAsync } = useMutation({
    mutationFn: ({ wish, action }: { wish: AdminWish; action: Action }) =>
      action === 'retry'
        ? retryWish(wish.id)
        : action === 'approve'
          ? approveWish(wish.id)
          : declineWish(wish.id),
  });
  const act = async (wish: AdminWish, action: Action) => {
    say(null);
    sync.reset();
    setActing((current) => new Map(current).set(wish.id, action));
    setFailures((current) => without(current, wish.id));
    try {
      await mutateAsync({ wish, action });
      say(t(ACTION_DONE[action], { title: wish.title.title }));
      await refresh();
      // The buttons went with the wish's former state: the focus goes to the row's toggle.
      await afterDialogsClosed();
      await afterRender();
      if (focusLost()) {
        body.current
          ?.querySelector<HTMLElement>(`tr[data-row-key="${wish.id}"] [aria-expanded]`)
          ?.focus();
      }
    } catch (error) {
      // The reason stands at the row: its fold-out opens with it (design rule 1.3).
      setFailures((current) => new Map(current).set(wish.id, error));
      openRow(wish.id);
      // The wish moved on meanwhile (409 `wish_not_pending`, `wish_not_retryable`): the row shows
      // what it is now, and the buttons that no longer apply go.
      if (isApiError(error, 409)) void refresh();
    } finally {
      setActing((current) => without(current, wish.id));
    }
  };
  const columns = useMemo(
    () =>
      requestColumns(t, language, (wish) => (
        <ReviewActions
          wish={wish}
          acting={acting.get(wish.id)}
          onApprove={(entry) => void act(entry, 'approve')}
          onDecline={setDeclining}
        />
      )),
    // `act` reads only setters and stable helpers; the row's state comes through `acting`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [t, language, acting],
  );
  const counts = list.data?.pages[0]?.counts;
  const openWishes = wishesOf(open.data?.pages);
  const openTotal = open.data?.pages.at(-1)?.total;
  // The two filters of the client see only the open wishes loaded so far.
  const partly = openTotal !== undefined && openWishes.length < openTotal;
  const countOf = (value: Filter) => {
    const only = CLIENT_FILTERS[value];
    if (only) return open.data ? openWishes.filter(only).length : undefined;
    if (!counts) return undefined;
    if (value === 'all') return counts.open + counts.available + counts.closed;
    return counts[value as Exclude<AdminWishFilter, 'all'>];
  };
  const only = CLIENT_FILTERS[filter];
  const loaded = list.data ? wishesOf(list.data.pages) : undefined;
  const rows = loaded && only ? loaded.filter(only) : loaded;
  const total = list.data?.pages.at(-1)?.total ?? 0;
  const queued = status.data?.counts.queued ?? 0;
  const more = () => {
    const before = list.data?.pages.at(-1)?.total;
    void list.fetchNextPage().then((result) => {
      // Pages go by offset over a list that moves (poll, approvals, sync). When the new page
      // counts other wishes than the list had, a wish left or joined the filter, the rest
      // shifted and one at the edge of a page would never show: every loaded page is read again,
      // from the first. Still possible: a move that keeps the count (a wish of a later page whose
      // change puts it first) hides that wish until the list is read again (the poll, the next
      // visit, any change made here).
      if (result.data?.pages.at(-1)?.total !== before) void list.refetch();
      // The button goes with the last page; the line that counts the wishes takes the focus.
      if (!result.hasNextPage) shownRef.current?.focus();
    });
  };
  const seerrHref = (wish: AdminWish) => seerrTitleHref(seerr.data?.url, wish.title);

  return (
    <div className={styles.tabBody} ref={body}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.requests')}</VisuallyHidden>
      <ConfirmDialog
        isOpen={declining !== null}
        onOpenChange={(isOpen) => {
          if (!isOpen) setDeclining(null);
        }}
        title={t('admin.wishes.requests.declineTitle', { title: declining?.title.title ?? '' })}
        body={t('admin.wishes.requests.declineBody')}
        confirmLabel={t('admin.wishes.requests.decline')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => {
          if (declining) void act(declining, 'decline');
        }}
      />
      <div className={styles.bar}>
        <ToggleToolbar
          label={t('admin.wishes.requests.filter.label')}
          value={filter}
          onChange={(value) => selectFilter(isFilter(value) ? value : 'all')}
          options={FILTERS.map((value) => {
            const count = countOf(value);
            const loadedOnly = partly && value in CLIENT_FILTERS;
            return {
              value,
              label: t(`admin.wishes.requests.filter.${value}`),
              count: count === undefined ? '–' : number.format(count),
              ...(count === undefined
                ? {}
                : {
                    countLabel: t(
                      loadedOnly
                        ? 'admin.wishes.requests.filter.countLoaded'
                        : 'admin.wishes.requests.filter.count',
                      { count },
                    ),
                  }),
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
      {sync.error ? (
        <Notice tone="danger">{problemMessage(t, sync.error)}</Notice>
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
        <div className={styles.tableFrame} data-perf="first-work-row">
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
                  retrying={acting.get(wish.id) === 'retry'}
                  failure={failures.get(wish.id) ?? null}
                  seerrHref={seerrHref(wish)}
                  onRetry={(entry) => void act(entry, 'retry')}
                />
              ),
            }}
          />
        </div>
      )}
      {loaded && (loaded.length < total || (list.data?.pages.length ?? 0) > 1) && (
        <div className={styles.actions}>
          <p ref={shownRef} tabIndex={-1} className={styles.summary}>
            {t(only ? 'admin.wishes.requests.searchedIn' : 'admin.wishes.requests.shown', {
              shown: number.format(loaded.length),
              total: number.format(total),
            })}
          </p>
          {list.hasNextPage && (
            <AriaButton
              variant="secondary"
              icon="chevronDown"
              isPending={list.isFetchingNextPage}
              onPress={more}
            >
              {t('admin.wishes.requests.more')}
            </AriaButton>
          )}
        </div>
      )}
    </div>
  );
}
