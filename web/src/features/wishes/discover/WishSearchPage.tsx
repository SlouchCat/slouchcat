import { useInfiniteQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useBackFocus } from '@/features/catalog/useBackFocus';
import { PageRetry } from '@/features/discover/ItemGrid';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Button, Notice, SearchField, SpatialGroup, SpatialPage, useNearEnd } from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import { DiscoverCard, GridSkeleton, useWishRequest } from './DiscoverCard';
import { DiscoverProblem } from './DiscoverProblem';
import { entriesOf, entryId, wishSearchQuery } from './queries';
import { WishesNav } from './WishesNav';

const route = getRouteApi('/_authed/_active/_chrome/discover/search');

const FIELD_KEY = 'wish-search-field';
const PREFIX = 'wish-search';
/**
 * Typing waits this long before the URL (and with it the request) follows: longer than the
 * library search (200 ms), as the server allows a profile 30 Seerr searches a minute.
 */
const DEBOUNCE_MS = 300;

/**
 * «Suche» of the wishes area (concept 25, 5.4, M-34): searches everything TMDB knows through
 * Seerr (searchWishTitles), also by id with `tmdb:`, `tvdb:` or `imdb:` (the server reads the
 * prefix), never persons. The text lives in the URL; the hits are tiles like on «Entdecken»,
 * further pages follow as the grid scrolls.
 */
export function WishSearchPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('wishes.area.search'));
  const goBack = useGoBack(paths.discover);
  useBackKeys(goBack);
  const params = route.useSearch();
  const navigate = route.useNavigate();

  const [text, setText] = useState(params.q ?? '');
  // The URL changed from elsewhere (a link, history): the field follows it. A value this page
  // wrote itself is not taken back, so a key typed while the URL catches up stays in the field.
  const [seenQ, setSeenQ] = useState(params.q);
  const [sentQ, setSentQ] = useState(params.q);
  if (params.q !== seenQ) {
    setSeenQ(params.q);
    if (params.q !== sentQ) {
      setSentQ(params.q);
      setText(params.q ?? '');
    }
  }
  useEffect(() => {
    if (text === (params.q ?? '')) return;
    const timer = window.setTimeout(() => {
      const q = text.trim() === '' ? undefined : text;
      setSentQ(q);
      void navigate({ search: () => (q ? { q } : {}), replace: true });
    }, DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text, params.q, navigate]);

  const q = (params.q ?? '').trim();
  // useInfiniteQuery with useNearEnd, not PagedSource with usePagedList: Seerr's pages carry no
  // total of titles and may hold fewer entries than asked, so the list cannot reserve its places.
  const query = useInfiniteQuery({ ...wishSearchQuery(q), enabled: q.length >= 2 });
  const requests = useWishRequest(query.data);
  const entries = entriesOf(query.data?.pages);
  const keyOf = (id: string) => `${PREFIX}-${id}`;
  useBackFocus(
    PREFIX,
    entries.map((entry) => keyOf(entryId(entry))),
  );

  const [end, near] = useNearEnd();
  const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = query;
  const pages = query.data?.pages.length ?? 0;
  useEffect(() => {
    if (near && hasNextPage && !isFetchingNextPage && !isFetchNextPageError) {
      void fetchNextPage();
    }
  }, [near, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, pages]);

  const typed = text.trim();
  let status = '';
  if (typed.length === 0) status = t('wishes.search.start');
  else if (typed.length === 1) status = t('wishes.search.tooShort');
  const asked = q.length >= 2;
  const failed = asked && query.isError && !query.data;

  return (
    <SpatialPage defaultFocusKey={FIELD_KEY}>
      <main className={discover.page}>
        <WishesNav current="search" limit />
        <div className={discover.head}>
          <h1 className={discover.title}>{t('wishes.area.search')}</h1>
          <p className={discover.overview}>{t('wishes.search.intro')}</p>
        </div>
        <div className={discover.searchBox}>
          <SpatialGroup focusKey="wish-search-box">
            <SearchField
              label={t('wishes.discover.searchLabel')}
              placeholder={t('wishes.search.placeholder')}
              value={text}
              onChange={setText}
              focusKey={FIELD_KEY}
              focusOnMount
            />
          </SpatialGroup>
        </div>
        <p className={discover.muted} role="status">
          {status}
        </p>

        <div className={discover.results} aria-busy={(asked && query.isFetching) || undefined}>
          {requests.element}
          {asked && query.isRefetchError && (
            <Notice
              tone="warning"
              title={t('wishes.discover.refreshFailed')}
              actions={
                <Button icon="refresh" onPress={() => void query.refetch()}>
                  {t('common.retry')}
                </Button>
              }
            />
          )}
          {asked && query.isPending && <GridSkeleton />}
          {failed && <DiscoverProblem error={query.error} onRetry={() => void query.refetch()} />}
          {asked && query.isSuccess && entries.length === 0 && !hasNextPage && (
            <Notice title={t('wishes.search.empty.title', { q })}>
              {t('wishes.search.empty.body')}
            </Notice>
          )}
          {asked && entries.length > 0 && (
            <SpatialGroup
              as="ul"
              focusKey={PREFIX}
              className={discover.grid}
              aria-label={t('wishes.search.hits')}
            >
              {entries.map((entry) => (
                <li key={entryId(entry)} className={discover.cell}>
                  <DiscoverCard
                    entry={entry}
                    focusKey={keyOf(entryId(entry))}
                    onRequest={requests.request}
                  />
                </li>
              ))}
            </SpatialGroup>
          )}
          {asked && isFetchNextPageError && <PageRetry onRetry={() => void fetchNextPage()} />}
          {asked && hasNextPage && !isFetchNextPageError && <div ref={end} aria-hidden="true" />}
        </div>
      </main>
    </SpatialPage>
  );
}
