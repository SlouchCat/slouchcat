import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useBackFocus } from '@/features/catalog/useBackFocus';
import { PageRetry } from '@/features/discover/ItemGrid';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ChoiceChips,
  FilterChips,
  Notice,
  SelectField,
  setSpatialFocus,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  useNearEnd,
} from '@/ui';
import discover from '@/features/discover/Discover.module.css';
import library from '@/features/catalog/LibraryPage.module.css';
import type { ExternalMedia } from '../queries';
import { DiscoverCard, GridSkeleton, useWishRequest } from './DiscoverCard';
import { DiscoverProblem } from './DiscoverProblem';
import { entriesOf, entryId, wishGenresQuery, wishTitlesQuery } from './queries';
import { TITLE_SORTS, type TitleSort, type TitlesParams } from './wishesAreaParams';
import { WishesNav } from './WishesNav';

const moviesRoute = getRouteApi('/_authed/_active/_chrome/discover/movies');
const seriesRoute = getRouteApi('/_authed/_active/_chrome/discover/series');

/** «Filme» (M-32): every film Seerr lists, sorted and filtered by the URL. */
export function WishMoviesPage() {
  const params = moviesRoute.useSearch();
  const navigate = moviesRoute.useNavigate();
  return (
    <TitlesView
      key="movie"
      media="movie"
      params={params}
      onParams={(next) => void navigate({ search: () => next, replace: true })}
    />
  );
}

/** «Serien» (M-33): the same for series. */
export function WishSeriesPage() {
  const params = seriesRoute.useSearch();
  const navigate = seriesRoute.useNavigate();
  return (
    <TitlesView
      key="show"
      media="show"
      params={params}
      onParams={(next) => void navigate({ search: () => next, replace: true })}
    />
  );
}

const HIDE = 'hide';

interface TitlesViewProps {
  media: ExternalMedia;
  params: TitlesParams;
  onParams: (params: TitlesParams) => void;
}

/**
 * «Filme» and «Serien» of the wishes area (concept 25, 5.4, M-32, M-33): a poster grid of every
 * title Seerr lists, by «Beliebtheit», «Neueste zuerst» or «Am besten bewertet» (N-117), narrowed
 * by a genre and «Vorhandene ausblenden» in a band that starts closed and says its choice on its
 * button. The view lives in the URL, so a row title or a genre tile of «Entdecken» preselects it;
 * the grid reads Seerr's next page as it scrolls (concept 18 C-3).
 */
function TitlesView({ media, params, onParams }: TitlesViewProps) {
  const { t } = useTranslation();
  const kind = media === 'movie' ? 'movies' : 'series';
  useDocumentTitle(t(`wishes.area.${kind}`));
  const goBack = useGoBack(paths.discover);
  useBackKeys(goBack);
  const prefix = `wish-${kind}`;
  const sortKey = `${prefix}-sort`;
  const filterKey = `${prefix}-filter`;
  const panelId = useId();
  const panel = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);

  const sort: TitleSort = params.sort ?? 'popularity';
  // useInfiniteQuery with useNearEnd, not PagedSource with usePagedList: Seerr's pages carry no
  // total of titles (only of pages, at most 500) and may hold fewer entries than asked or repeat a
  // title, so the list cannot reserve its places ahead.
  const query = useInfiniteQuery(
    wishTitlesQuery(media, {
      ...(params.sort ? { sort: params.sort } : {}),
      ...(params.genre ? { genre: params.genre } : {}),
      ...(params.hide ? { hideAvailable: true } : {}),
    }),
  );
  const genres = useQuery(wishGenresQuery(media));
  const requests = useWishRequest(query.data);
  const entries = entriesOf(query.data?.pages);
  const stale = query.data?.pages.some((page) => page.stale) ?? false;
  const keyOf = (id: string) => `${prefix}-${id}`;
  useBackFocus(
    prefix,
    entries.map((entry) => keyOf(entryId(entry))),
  );

  // The next page once the end of the grid comes near (a page may hold no visible title: the
  // marker then stays in view and the page after it follows).
  const [end, near] = useNearEnd();
  const { hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage } = query;
  const pages = query.data?.pages.length ?? 0;
  useEffect(() => {
    if (near && hasNextPage && !isFetchingNextPage && !isFetchNextPageError) {
      void fetchNextPage();
    }
  }, [near, hasNextPage, isFetchingNextPage, isFetchNextPageError, fetchNextPage, pages]);

  // Escape in the open band closes it and hands the focus back to its button, before the page's
  // back key (useBackKeys) would leave the page.
  useEffect(() => {
    if (!open) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !(event.target instanceof Node)) return;
      if (!panel.current?.contains(event.target)) return;
      event.preventDefault();
      setOpen(false);
      setSpatialFocus(filterKey);
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    return () => window.removeEventListener('keydown', onKeyDown, { capture: true });
  }, [open, filterKey]);

  const update = (change: Partial<TitlesParams>) => {
    const next: TitlesParams = { ...params, ...change };
    for (const key of Object.keys(next) as (keyof TitlesParams)[]) {
      if (next[key] === undefined) delete next[key];
    }
    onParams(next);
  };
  const filters = (params.genre ? 1 : 0) + (params.hide ? 1 : 0);
  const clear = () => update({ genre: undefined, hide: undefined });

  const genreList = genres.data?.items ?? [];
  const genreName =
    params.genre === undefined
      ? t('wishes.titles.allGenres')
      : (genreList.find((genre) => genre.id === params.genre)?.name ??
        t('wishes.titles.genreId', { id: params.genre }));
  // The band's choice on its button (owner's rule: a closed band says what it holds).
  const summary = [genreName, t(params.hide ? 'wishes.titles.hidden' : 'wishes.titles.shown')].join(
    t('format.separator'),
  );
  const genreOptions = [
    { value: '', label: t('wishes.titles.allGenres') },
    ...genreList.map((genre) => ({ value: String(genre.id), label: genre.name })),
  ];
  // A genre from a link that the list does not (yet) hold stays choosable.
  if (params.genre && !genreList.some((genre) => genre.id === params.genre)) {
    genreOptions.push({ value: String(params.genre), label: genreName });
  }

  const first = entries[0];
  const failed = query.isError && !query.data;
  return (
    <SpatialPage defaultFocusKey={first ? keyOf(entryId(first)) : sortKey}>
      <main className={discover.page} aria-busy={query.isPending || undefined}>
        <WishesNav current={kind} limit />
        <div className={discover.head}>
          <h1 className={discover.title}>{t(`wishes.area.${kind}`)}</h1>
          {!failed && <p className={discover.overview}>{t(`wishes.titles.${kind}Intro`)}</p>}
        </div>

        {!failed && (
          <SpatialGroup focusKey={`${prefix}-tools`} className={library.tools}>
            <SelectField
              label={t('wishes.titles.sort')}
              focusKey={sortKey}
              value={sort}
              options={TITLE_SORTS.map((value) => ({
                value,
                label: t(`wishes.titles.sorts.${value}`),
              }))}
              onChange={(next) => update({ sort: next === 'popularity' ? undefined : next })}
            />
            <SpatialButton
              focusKey={filterKey}
              icon="sliders"
              variant={filters > 0 ? 'primary' : 'secondary'}
              aria-expanded={open}
              aria-controls={panelId}
              onPress={() => setOpen((value) => !value)}
            >
              {t('wishes.titles.filter', { summary })}
            </SpatialButton>
          </SpatialGroup>
        )}

        {!failed && open && (
          <div
            ref={panel}
            id={panelId}
            role="region"
            aria-label={t('wishes.titles.filterLabel')}
            className={library.panel}
          >
            {genreList.length > 0 && (
              <div className={library.filterGroup}>
                <span className={library.filterLabel} aria-hidden="true">
                  {t('wishes.titles.genre')}
                </span>
                <ChoiceChips
                  label={t('wishes.titles.genre')}
                  focusKey={`${prefix}-genre`}
                  value={params.genre ? String(params.genre) : ''}
                  onChange={(value) => update({ genre: value ? Number(value) : undefined })}
                  options={genreOptions}
                />
              </div>
            )}
            <div className={library.filterGroup}>
              <span className={library.filterLabel} aria-hidden="true">
                {t('wishes.titles.library')}
              </span>
              <FilterChips
                label={t('wishes.titles.library')}
                focusKey={`${prefix}-hide`}
                values={params.hide ? [HIDE] : []}
                onChange={(values) => update({ hide: values.includes(HIDE) ? true : undefined })}
                options={[{ value: HIDE, label: t('wishes.titles.hide') }]}
              />
              <p className={discover.muted}>{t('wishes.titles.hideHelp')}</p>
            </div>
            <SpatialGroup className={library.panelActions}>
              <SpatialButton focusKey={`${prefix}-clear`} variant="ghost" onPress={clear}>
                {t('wishes.titles.clear')}
              </SpatialButton>
              <SpatialButton
                focusKey={`${prefix}-apply`}
                variant="primary"
                onPress={() => {
                  setOpen(false);
                  setSpatialFocus(filterKey);
                }}
              >
                {t('wishes.titles.apply')}
              </SpatialButton>
            </SpatialGroup>
          </div>
        )}

        {stale && <Notice tone="warning" title={t('wishes.discover.stale')} />}
        {query.isRefetchError && (
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
        {requests.element}

        {query.isPending && <GridSkeleton />}
        {failed && <DiscoverProblem error={query.error} onRetry={() => void query.refetch()} />}
        {query.isSuccess && entries.length === 0 && !hasNextPage && (
          <Notice
            title={t(`wishes.titles.${kind}Empty`)}
            actions={
              filters > 0 && (
                <SpatialGroup>
                  <SpatialButton
                    focusKey={`${prefix}-empty-clear`}
                    variant="primary"
                    onPress={clear}
                  >
                    {t('wishes.titles.clear')}
                  </SpatialButton>
                </SpatialGroup>
              )
            }
          />
        )}
        {entries.length > 0 && (
          <SpatialGroup
            as="ul"
            focusKey={prefix}
            className={discover.grid}
            aria-label={t(`wishes.area.${kind}`)}
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
        {isFetchNextPageError && <PageRetry onRetry={() => void fetchNextPage()} />}
        {isFetchingNextPage && <Skeleton shape="line" />}
        {hasNextPage && !isFetchNextPageError && <div ref={end} aria-hidden="true" />}
      </main>
    </SpatialPage>
  );
}
