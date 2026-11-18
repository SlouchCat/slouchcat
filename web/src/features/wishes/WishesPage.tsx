import type { ItemRef } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { lazy, Suspense, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { paths } from '@/app/paths';
import { PageRetry } from '@/features/discover/ItemGrid';
import { quotaText } from '@/features/shared/quota';
import { intlLocale } from '@/i18n';
import {
  PAGED_LIST_VIRTUAL_FROM,
  rangeBySize,
  usePagedList,
  type PlaceRange,
} from '@/lib/pagedList';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import {
  Button,
  ChoiceChips,
  Notice,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
  VirtualList,
} from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import {
  useWithdrawWish,
  wishesSource,
  wishQuotaQuery,
  type Wish,
  type WishStateFilter,
} from './queries';
import { wishProblemMessage } from './state';
import { WISH_FILTERS, type WishesParams } from './wishesParams';
import { WishesNav } from './discover/WishesNav';
import { WishRow, wishRowKey } from './WishRow';
import styles from './Wishes.module.css';

const route = getRouteApi('/_authed/_active/_chrome/wishes');

// Dialogs need react-aria, which the browsing pages do not load up front (P-10).
const RequestDialog = lazy(() =>
  import('./RequestDialog').then((module) => ({ default: module.RequestDialog })),
);
const WithdrawDialog = lazy(() =>
  import('./WithdrawDialog').then((module) => ({ default: module.WithdrawDialog })),
);

const BACK_KEY = 'wishes-back';
const FILTER_KEY = 'wishes-filter';
/** The count line under the title, for the list the chosen filter shows. */
const COUNT_KEYS = {
  all: 'wishes.page.countAll',
  open: 'wishes.page.count',
  available: 'wishes.page.countAvailable',
  closed: 'wishes.page.countClosed',
} as const satisfies Record<WishStateFilter | 'all', string>;
/** A row's expected height in px (poster, three lines and the actions). */
const ROW_HEIGHT = 168;

/**
 * «Meine Wünsche» (concept 25, 5.3, M-08): the profile's wishes, own and joined, newest first, with
 * the limit of the account on top and the chips «Alle», «Offen», «Verfügbar», «Abgeschlossen»
 * with their counts (state in the URL). Each row says its state and offers what the server allows;
 * «Zurückziehen» asks first, «Erneut anfordern» opens the request dialog. The list loads more as it
 * scrolls. Wishes live on the server: offline the page says so.
 */
export function WishesPage() {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  useDocumentTitle(t('wishes.page.title'));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const online = useOnline();
  const navigate = useNavigate();
  const params = route.useSearch();
  const routeNavigate = route.useNavigate();
  const state = params.state;
  const source = useMemo(() => wishesSource(state), [state]);
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(source, rangeBySize(span), { enabled: online });
  const list = paged.first;
  const quota = useQuery({ ...wishQuotaQuery, enabled: online });
  const withdraw = useWithdrawWish();
  const [asking, setAsking] = useState<Wish | null>(null);
  const [again, setAgain] = useState<Wish | null>(null);
  const [requested, setRequested] = useState(false);
  const counts = list.data?.counts;

  const select = (value: string) =>
    void routeNavigate({
      search: (): WishesParams =>
        WISH_FILTERS.some((filter) => filter === value) ? { state: value as WishStateFilter } : {},
      replace: true,
    });

  // The limit of the account: no limit read (no right, never read) says nothing.
  const limit =
    quota.data && !isApiError(quota.error)
      ? [quotaText(t, language, 'movie', quota.data), quotaText(t, language, 'tv', quota.data)]
          .filter(Boolean)
          .join(t('format.separator'))
      : '';
  const items = paged.slots.filter((wish): wish is Wish => wish !== undefined);
  const first = items[0];
  const countOf = (filter: WishStateFilter | '') => {
    if (!counts) return undefined;
    if (filter === '') return undefined;
    return counts[filter];
  };

  // A withdrawn wish may leave the filtered list: the focus goes to the neighbour first.
  const onWithdrawn = (wish: Wish) => {
    const index = items.findIndex((entry) => entry.id === wish.id);
    const neighbour = items[index + 1] ?? items[index - 1];
    setSpatialFocus(neighbour ? wishRowKey(neighbour) : FILTER_KEY);
  };

  return (
    <SpatialPage defaultFocusKey={first ? wishRowKey(first) : BACK_KEY}>
      <main className={discover.page} aria-busy={list.isPending || undefined}>
        <WishesNav current="mine" />
        <div className={discover.head}>
          <h1 className={discover.title}>{t('wishes.page.title')}</h1>
          {list.isSuccess && (
            // The count of the list the chosen filter shows, like «Gesehen» counts its list.
            <p className={discover.overview}>
              {t(COUNT_KEYS[state ?? 'all'], { count: paged.total })}
            </p>
          )}
          {limit && <p className={discover.muted}>{t('wishes.page.limit', { limit })}</p>}
          <SpatialGroup className={discover.actions}>
            <SpatialButton focusKey={BACK_KEY} variant="ghost" icon="back" onPress={goBack}>
              {t('common.back')}
            </SpatialButton>
          </SpatialGroup>
        </div>

        {!online && <Notice title={t('wishes.page.offline')} />}

        {online && (
          <section className={discover.filters} aria-label={t('wishes.page.filters.label')}>
            <div className={discover.filterGroup}>
              <span className={discover.filterLabel} aria-hidden="true">
                {t('wishes.page.filter')}
              </span>
              <ChoiceChips
                label={t('wishes.page.filter')}
                focusKey={FILTER_KEY}
                value={state ?? ''}
                onChange={select}
                options={(['', ...WISH_FILTERS] as const).map((filter) => {
                  const count = countOf(filter);
                  return {
                    value: filter,
                    label:
                      count === undefined
                        ? t(`wishes.page.filters.${filter || 'all'}`)
                        : t('wishes.page.filterCount', {
                            name: t(`wishes.page.filters.${filter || 'all'}`),
                            count,
                          }),
                  };
                })}
              />
            </div>
          </section>
        )}

        {online && list.isPending && <Skeleton shape="block" />}
        {online && list.isError && !list.data && (
          <Notice
            tone="danger"
            title={wishProblemMessage(t, list.error)}
            actions={
              <Button icon="refresh" onPress={() => void list.refetch()}>
                {t('common.retry')}
              </Button>
            }
          />
        )}
        {withdraw.isError && <Notice tone="danger" title={wishProblemMessage(t, withdraw.error)} />}
        {withdraw.isSuccess && <Notice title={t('titles.done.withdrawn')} />}
        {requested && <Notice title={t('titles.done.requested')} />}

        {online &&
          list.isSuccess &&
          paged.total === 0 &&
          (state ? (
            <Notice title={t(`wishes.page.emptyFilter.${state}`)} />
          ) : (
            <Notice title={t('wishes.page.empty.title')}>{t('wishes.page.empty.body')}</Notice>
          ))}

        {online && paged.total > 0 && (
          <section className={discover.section} aria-labelledby="wishes-list">
            <h2 id="wishes-list" className={discover.sectionTitle}>
              {t(`wishes.page.filters.${state ?? 'all'}`)}
            </h2>
            <VirtualList
              slots={paged.slots}
              virtual={paged.total > PAGED_LIST_VIRTUAL_FROM}
              onRange={setSpan}
              className={styles.rows}
              label={t('wishes.page.title')}
              estimate={ROW_HEIGHT}
              itemKey={(wish) => wish.id}
              renderPlaceholder={() => <Skeleton shape="line" />}
              renderItem={(wish) => (
                <WishRow
                  wish={wish}
                  busy={withdraw.isPending && withdraw.variables === wish.id}
                  onWithdraw={setAsking}
                  onRequestAgain={(entry) => {
                    setRequested(false);
                    withdraw.reset();
                    setAgain(entry);
                  }}
                />
              )}
            />
            {paged.failed && <PageRetry onRetry={paged.retry} />}
          </section>
        )}

        {asking && (
          <Suspense fallback={null}>
            <WithdrawDialog
              title={asking.title}
              joined={asking.role === 'joined'}
              onClose={() => setAsking(null)}
              onConfirm={() => {
                const wish = asking;
                setAsking(null);
                setRequested(false);
                withdraw.mutate(wish.id, { onSuccess: () => onWithdrawn(wish) });
              }}
            />
          </Suspense>
        )}
        {again && (
          <Suspense fallback={null}>
            <RequestDialog
              title={again}
              source="list"
              onClose={() => setAgain(null)}
              onDone={() => {
                setAgain(null);
                setRequested(true);
              }}
              onAvailable={(item: ItemRef) => void navigate({ href: paths.item(item.id) })}
            />
          </Suspense>
        )}
      </main>
    </SpatialPage>
  );
}
