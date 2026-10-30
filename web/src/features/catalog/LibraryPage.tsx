import type { components, Rating } from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { cardKey, ItemGrid, MoreButton } from '@/features/discover/ItemGrid';
import { YearField } from '@/features/discover/YearField';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  Button,
  ButtonLink,
  ChoiceChips,
  FilterChips,
  LetterBar,
  Notice,
  SelectField,
  setSpatialFocus,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  VisuallyHidden,
} from '@/ui';
import {
  defaultOrder,
  filterCount,
  LETTERS,
  libraryRequest,
  SORTS,
  STATUSES,
  statusesOf,
  type LibraryParams,
  type LibrarySort,
} from './libraryParams';
import { libraryItemsQuery, visibleLibrariesQuery } from './queries';
import { useBackFocus } from './useBackFocus';
import styles from './LibraryPage.module.css';

type LibraryNav = components['schemas']['LibraryNav'];

const route = getRouteApi('/_authed/_active/_chrome/libraries/$libraryId');

const GRID_KEY = 'library-item';
const SORT_KEY = 'library-sort';
const ORDER_KEY = 'library-order';
const FILTER_KEY = 'library-filter';

/** The orders a library offers: the author only for audiobooks, the age rating only for video. */
const sortsOf = (kind: LibraryNav['kind']): LibrarySort[] =>
  SORTS.filter(
    (sort) => (sort !== 'author' || kind === 'audiobooks') && (sort !== 'age' || kind === 'video'),
  );

/** A library (concept 08): every title the active profile may see, sorted and filtered. */
export function LibraryPage() {
  const { libraryId } = route.useParams();
  const id = Number(libraryId);
  // A new library starts with the filter panel closed.
  return <LibraryView key={libraryId} id={Number.isSafeInteger(id) && id > 0 ? id : null} />;
}

function LibraryView({ id }: { id: number | null }) {
  const { t } = useTranslation();
  const params = route.useSearch();
  const navigate = route.useNavigate();
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const panelId = useId();
  const panel = useRef<HTMLDivElement>(null);
  const [filtersOpen, setFiltersOpen] = useState(false);

  const libraries = useQuery(visibleLibrariesQuery);
  const library = libraries.data?.find((entry) => entry.id === id);
  const request = useMemo(() => libraryRequest(params), [params]);
  const query = useInfiniteQuery({
    ...libraryItemsQuery(id ?? 0, request),
    enabled: id !== null,
  });
  const notFound = id === null || (libraries.isSuccess && !library) || isApiError(query.error, 404);
  useDocumentTitle(library?.name ?? (notFound ? t('library.notFound.title') : undefined));

  const items = useMemo(() => query.data?.pages.flatMap((page) => page.items) ?? [], [query.data]);
  const cardKeys = useMemo(() => items.map((item) => cardKey(GRID_KEY, item)), [items]);
  useBackFocus(GRID_KEY, cardKeys);
  const first = query.data?.pages[0];

  const update = (change: Partial<LibraryParams>) =>
    void navigate({
      search: (previous) => {
        const next: LibraryParams = { ...previous, ...change };
        for (const key of Object.keys(next) as (keyof LibraryParams)[]) {
          if (next[key] === undefined || next[key] === '') delete next[key];
        }
        return next;
      },
      replace: true,
    });
  const clearFilters = () =>
    update({
      genre: undefined,
      status: undefined,
      from: undefined,
      to: undefined,
      rating: undefined,
    });
  const closeFilters = () => {
    setFiltersOpen(false);
    setSpatialFocus(FILTER_KEY);
  };

  // Escape in the open panel closes it and hands the focus back to "Filter". Captured, so the
  // page's back key (useBackKeys) does not leave the page as well.
  useEffect(() => {
    if (!filtersOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || !(event.target instanceof Node)) return;
      if (!panel.current?.contains(event.target)) return;
      event.preventDefault();
      setFiltersOpen(false);
      setSpatialFocus(FILTER_KEY);
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    return () => window.removeEventListener('keydown', onKeyDown, { capture: true });
  }, [filtersOpen]);

  if (notFound) {
    return (
      <main className={styles.page}>
        <Notice
          title={t('library.notFound.title')}
          actions={
            <ButtonLink href={paths.home} variant="primary">
              {t('item.notFound.action')}
            </ButtonLink>
          }
        >
          {t('library.notFound.body')}
        </Notice>
      </main>
    );
  }

  const failed = (libraries.isError && !libraries.data) || (query.isError && !query.data);
  if (failed) {
    return (
      <main className={styles.page}>
        <Notice
          tone="danger"
          title={problemMessage(t, libraries.error ?? query.error)}
          actions={
            <Button
              icon="refresh"
              onPress={() => {
                void libraries.refetch();
                void query.refetch();
              }}
            >
              {t('common.retry')}
            </Button>
          }
        />
      </main>
    );
  }

  if (!library || !first) {
    return (
      <main className={styles.page} aria-busy="true">
        <Skeleton shape="title" />
        <Skeleton shape="block" />
      </main>
    );
  }

  const { facets, total } = first;
  const audio = library.kind !== 'video';
  const sort = params.sort ?? 'title';
  const order = params.order ?? defaultOrder(sort);
  const filters = filterCount(params);
  const statuses = statusesOf(params);

  const noun = t(`library.page.noun.${library.kind}`, { count: total });
  const counted = params.letter
    ? t('library.page.countLetter', {
        count: total,
        noun,
        letter: params.letter === '#' ? t('library.letters.other') : params.letter,
      })
    : t('library.page.count', { count: total, noun });

  const genreOptions = [
    { value: '', label: t('library.filters.allGenres') },
    ...facets.genres.map((genre) => ({ value: genre.name, label: genre.name })),
  ];
  // A chosen genre stays choosable while the facets of the new page are on their way.
  if (params.genre && !genreOptions.some((option) => option.value === params.genre)) {
    genreOptions.push({ value: params.genre, label: params.genre });
  }
  const ratings: Rating[] = [...facets.ratings];
  if (params.rating !== undefined && !ratings.includes(params.rating)) ratings.push(params.rating);

  // The letter bar filters a list sorted by title; podcasts have none (F-12).
  const showLetters = sort === 'title' && library.kind !== 'podcasts';

  const firstItem = items[0];
  return (
    <SpatialPage defaultFocusKey={firstItem ? cardKey(GRID_KEY, firstItem) : SORT_KEY}>
      <main className={styles.page}>
        <div className={styles.head}>
          <h1 className={styles.title}>{library.name}</h1>
          <p className={styles.count} role="status">
            {filters > 0 ? t('library.page.filtered', { text: counted }) : counted}
          </p>
        </div>

        <SpatialGroup focusKey="library-tools" className={styles.tools}>
          {library.contents.length > 1 && (
            <ChoiceChips
              label={t('library.content.title')}
              focusKey="library-content"
              value={params.content ?? ''}
              onChange={(content) =>
                update({
                  content: content === '' ? undefined : (content as LibraryParams['content']),
                  letter: undefined,
                })
              }
              options={[
                { value: '', label: t('library.content.all') },
                ...library.contents.map((content) => ({
                  value: content,
                  label: t(`library.content.${content}`),
                })),
              ]}
            />
          )}
          <span className={styles.grow} />
          <SelectField
            label={t('library.page.sort')}
            focusKey={SORT_KEY}
            value={sort}
            options={sortsOf(library.kind).map((value) => ({
              value,
              label: t(`library.sort.${value}`),
            }))}
            // A new order starts in its own direction and without a letter (title only).
            onChange={(next) =>
              update({
                sort: next === 'title' ? undefined : next,
                order: undefined,
                letter: undefined,
              })
            }
          />
          <SpatialButton
            focusKey={ORDER_KEY}
            variant="secondary"
            onPress={() => {
              const next = order === 'asc' ? 'desc' : 'asc';
              update({ order: next === defaultOrder(sort) ? undefined : next });
            }}
          >
            <VisuallyHidden>{t('library.page.direction')} </VisuallyHidden>
            {t(order === 'asc' ? 'library.page.ascending' : 'library.page.descending')}
          </SpatialButton>
          <SpatialButton
            focusKey={FILTER_KEY}
            variant={filters > 0 ? 'primary' : 'secondary'}
            aria-expanded={filtersOpen}
            aria-controls={panelId}
            onPress={() => setFiltersOpen((open) => !open)}
          >
            {filters > 0
              ? t('library.page.filterCount', { count: filters })
              : t('library.page.filter')}
          </SpatialButton>
        </SpatialGroup>

        {filtersOpen && (
          <div
            ref={panel}
            id={panelId}
            role="region"
            aria-label={t('library.page.filter')}
            className={styles.panel}
          >
            {facets.genres.length > 0 && (
              <div className={styles.filterGroup}>
                <span className={styles.filterLabel} aria-hidden="true">
                  {t('library.filters.genre')}
                </span>
                <ChoiceChips
                  label={t('library.filters.genre')}
                  focusKey="library-genre"
                  value={params.genre ?? ''}
                  onChange={(genre) => update({ genre: genre || undefined })}
                  options={genreOptions}
                />
              </div>
            )}
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('library.filters.status')}
              </span>
              <FilterChips
                label={t('library.filters.status')}
                focusKey="library-status"
                values={statuses}
                onChange={(values) => update({ status: values.join(',') || undefined })}
                options={STATUSES.map((status) => ({
                  value: status,
                  label: t(
                    audio && status === 'unwatched'
                      ? 'library.status.unheard'
                      : audio && status === 'watched'
                        ? 'library.status.heard'
                        : `library.status.${status}`,
                  ),
                }))}
              />
            </div>
            <div className={styles.filterGroup} role="group" aria-label={t('library.filters.year')}>
              <SpatialGroup focusKey="library-years" className={styles.years}>
                <YearField
                  label={t('library.filters.yearFrom')}
                  value={params.from}
                  onChange={(from) => update({ from })}
                  focusKey="library-from"
                />
                <YearField
                  label={t('library.filters.yearTo')}
                  value={params.to}
                  onChange={(to) => update({ to })}
                  focusKey="library-to"
                />
              </SpatialGroup>
            </div>
            {ratings.length > 0 && (
              <div className={styles.filterGroup}>
                <span className={styles.filterLabel} aria-hidden="true">
                  {t('library.filters.rating')}
                </span>
                <ChoiceChips
                  label={t('library.filters.rating')}
                  focusKey="library-rating"
                  value={params.rating === undefined ? '' : String(params.rating)}
                  onChange={(rating) =>
                    update({ rating: rating === '' ? undefined : (Number(rating) as Rating) })
                  }
                  options={[
                    { value: '', label: t('library.filters.anyRating') },
                    ...ratings.map((age) => ({
                      value: String(age),
                      label: t('library.filters.upTo', { age }),
                    })),
                  ]}
                />
              </div>
            )}
            <SpatialGroup className={styles.panelActions}>
              <SpatialButton focusKey="library-clear" variant="ghost" onPress={clearFilters}>
                {t('library.page.clear')}
              </SpatialButton>
              <SpatialButton focusKey="library-apply" variant="primary" onPress={closeFilters}>
                {t('library.page.apply')}
              </SpatialButton>
            </SpatialGroup>
          </div>
        )}

        <div className={styles.body}>
          {showLetters && (
            <LetterBar
              label={t('library.letters.label')}
              letters={LETTERS}
              available={facets.letters}
              value={params.letter}
              onChange={(letter) => update({ letter })}
              allMark={t('library.letters.allMark')}
              allLabel={t('library.letters.all')}
              letterLabels={{ '#': t('library.letters.other') }}
              focusKey="library-letter"
              className={styles.letters}
            />
          )}

          {/* While a changed view loads, the previous grid stays: busy, not gone. */}
          <div className={styles.results} aria-busy={query.isFetching || undefined}>
            {items.length === 0 ? (
              <Notice
                title={t('library.page.emptyFilters')}
                actions={
                  filters > 0 && (
                    <SpatialGroup>
                      <SpatialButton
                        focusKey="library-empty-clear"
                        variant="primary"
                        onPress={clearFilters}
                      >
                        {t('library.page.clear')}
                      </SpatialButton>
                    </SpatialGroup>
                  )
                }
              />
            ) : (
              <>
                <ItemGrid
                  items={items}
                  focusKey={GRID_KEY}
                  label={library.name}
                  className={styles.grid}
                  showWatched
                  showRating={library.kind === 'video'}
                />
                <MoreButton
                  shown={items.length}
                  keys={cardKeys}
                  hasMore={query.hasNextPage}
                  loading={query.isFetchingNextPage}
                  failed={query.isFetchNextPageError}
                  label={t('library.page.more', { count: Math.max(0, total - items.length) })}
                  focusKey="library-more"
                  onMore={() => void query.fetchNextPage()}
                />
              </>
            )}
          </div>
        </div>
      </main>
    </SpatialPage>
  );
}
