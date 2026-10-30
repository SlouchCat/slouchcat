import type { Rating } from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import {
  ChoiceChips,
  CollectionCard,
  FilterChips,
  Notice,
  SearchField,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
} from '@/ui';
import { cardKey, ItemGrid, MoreButton } from './ItemGrid';
import { genresQuery, searchQuery } from './queries';
import {
  AVAILABILITY,
  availabilityOf,
  hasFilter,
  MEDIA_TYPES,
  mediaTypesOf,
  RATINGS,
  searchRequest,
  type SearchParams,
} from './searchParams';
import { YearField } from './YearField';
import styles from './Discover.module.css';

const route = getRouteApi('/_authed/_active/_chrome/search');

export const SEARCH_FIELD_KEY = 'search-field';
/** Typing waits this long before the URL (and with it the request) follows (design 4.1). */
const DEBOUNCE_MS = 200;

/**
 * Search (E-4): titles, people and collections from the second character, with filters for
 * genre, year, age rating, media type and availability. The state lives in the URL; a genre chip
 * of the start page opens the page with its filter and no text.
 */
export function SearchPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('search.title'));
  const params = route.useSearch();
  const navigate = route.useNavigate();
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  const field = useRef<HTMLInputElement>(null);

  const [text, setText] = useState(params.q ?? '');
  // The URL changed from elsewhere (the "Suche" link, history): the field follows it. A value this
  // page wrote itself is not taken back, so a key typed while the URL catches up stays in the field.
  const [seenQ, setSeenQ] = useState(params.q);
  const [sentQ, setSentQ] = useState(params.q);
  if (params.q !== seenQ) {
    setSeenQ(params.q);
    if (params.q !== sentQ) {
      setSentQ(params.q);
      setText(params.q ?? '');
    }
  }
  const update = (change: Partial<SearchParams>) =>
    void navigate({
      search: (previous) => {
        const next: SearchParams = { ...previous, ...change };
        for (const key of Object.keys(next) as (keyof SearchParams)[]) {
          if (next[key] === undefined || next[key] === '') delete next[key];
        }
        return next;
      },
      replace: true,
    });

  // The URL (and the request) follows the field 200 ms after the last key.
  useEffect(() => {
    if (text === (params.q ?? '')) return;
    const timer = window.setTimeout(() => {
      setSentQ(text || undefined);
      update({ q: text || undefined });
    }, DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `update` is recreated every render
  }, [text, params.q]);

  // "/" focuses the field (as in the header elsewhere); typed into a field it stays a character.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey) return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))
      ) {
        return;
      }
      event.preventDefault();
      field.current?.focus();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const body = useMemo(() => searchRequest(params), [params]);
  const results = useInfiniteQuery({
    ...searchQuery(body ?? {}),
    enabled: body !== null,
  });
  const genres = useQuery(genresQuery);

  const typed = text.trim();
  const first = results.data?.pages[0];
  const items = useMemo(
    () => results.data?.pages.flatMap((page) => page.items) ?? [],
    [results.data],
  );
  const total = first?.total ?? 0;
  const people = first?.people ?? [];
  const collections = first?.collections ?? [];
  const found = items.length + people.length + collections.length > 0;
  const showResults = body !== null && results.isSuccess;

  const genreOptions = [
    { value: '', label: t('search.filters.allGenres') },
    ...(genres.data ?? []).map((genre) => ({ value: genre.name, label: genre.name })),
  ];
  // A genre from a link that the list does not (yet) hold stays choosable.
  if (params.genre && !genreOptions.some((option) => option.value === params.genre)) {
    genreOptions.push({ value: params.genre, label: params.genre });
  }

  let status: string;
  if (typed.length === 1) status = t('search.tooShort');
  else if (body === null) status = t('search.start');
  // Nothing found: the notice below says so, the status line stays empty (no second message).
  else if (showResults && found) status = t('search.results', { count: total });
  else status = '';

  return (
    <SpatialPage defaultFocusKey={SEARCH_FIELD_KEY}>
      <main className={styles.page}>
        <div className={styles.head}>
          <h1 className={styles.title}>{t('search.title')}</h1>
        </div>
        <div className={styles.searchBox}>
          <SpatialGroup focusKey="search-box">
            <SearchField
              ref={field}
              label={t('search.label')}
              placeholder={t('search.placeholder')}
              value={text}
              onChange={setText}
              focusKey={SEARCH_FIELD_KEY}
              focusOnMount
            />
          </SpatialGroup>
        </div>

        <section className={styles.filters} aria-label={t('search.filters.label')}>
          <div className={styles.filterGroup}>
            <span className={styles.filterLabel} aria-hidden="true">
              {t('search.filters.genre')}
            </span>
            <ChoiceChips
              label={t('search.filters.genre')}
              focusKey="filter-genre"
              value={params.genre ?? ''}
              onChange={(genre) => update({ genre: genre || undefined })}
              options={genreOptions}
            />
          </div>
          <SpatialGroup focusKey="filter-years" className={styles.filterRow}>
            <YearField
              label={t('search.filters.yearFrom')}
              value={params.from}
              onChange={(from) => update({ from })}
              focusKey="filter-from"
            />
            <YearField
              label={t('search.filters.yearTo')}
              value={params.to}
              onChange={(to) => update({ to })}
              focusKey="filter-to"
            />
          </SpatialGroup>
          <div className={styles.filterRow}>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('search.filters.rating')}
              </span>
              <ChoiceChips
                label={t('search.filters.rating')}
                focusKey="filter-rating"
                value={params.rating === undefined ? '' : String(params.rating)}
                onChange={(rating) =>
                  update({ rating: rating === '' ? undefined : (Number(rating) as Rating) })
                }
                options={[
                  { value: '', label: t('search.filters.anyRating') },
                  ...RATINGS.map((age) => ({
                    value: String(age),
                    label: t('rating.fsk', { age }),
                  })),
                ]}
              />
            </div>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('search.filters.type')}
              </span>
              <FilterChips
                label={t('search.filters.type')}
                focusKey="filter-type"
                values={mediaTypesOf(params)}
                onChange={(types) => update({ type: types.join(',') || undefined })}
                options={MEDIA_TYPES.map((type) => ({
                  value: type,
                  label: t(`search.filters.types.${type}`),
                }))}
              />
            </div>
            <div className={styles.filterGroup}>
              <span className={styles.filterLabel} aria-hidden="true">
                {t('search.filters.availability')}
              </span>
              <FilterChips
                label={t('search.filters.availability')}
                focusKey="filter-avail"
                values={availabilityOf(params)}
                onChange={(avail) => update({ avail: avail.join(',') || undefined })}
                options={AVAILABILITY.map((avail) => ({
                  value: avail,
                  label: t(`search.filters.avail.${avail}`),
                }))}
              />
            </div>
          </div>
          {hasFilter(params) && (
            <SpatialGroup>
              <SpatialButton
                focusKey="filter-reset"
                variant="ghost"
                icon="refresh"
                onPress={() =>
                  update({
                    genre: undefined,
                    from: undefined,
                    to: undefined,
                    rating: undefined,
                    type: undefined,
                    avail: undefined,
                  })
                }
              >
                {t('search.filters.reset')}
              </SpatialButton>
            </SpatialGroup>
          )}
        </section>

        <p className={styles.muted} role="status">
          {status}
        </p>

        {/* While a changed search loads, the previous results stay: busy, not gone. */}
        <div className={styles.results} aria-busy={results.isFetching || undefined}>
          {results.isError && body !== null && !results.data && (
            <Notice tone="danger" title={problemMessage(t, results.error)} />
          )}

          {showResults && !found && (
            <Notice title={t('search.empty.title')}>{t('search.empty.body')}</Notice>
          )}

          {showResults && items.length > 0 && (
            <section className={styles.section} aria-labelledby="search-titles">
              <h2 id="search-titles" className={styles.sectionTitle}>
                {t('search.sections.titles')}
              </h2>
              <ItemGrid items={items} focusKey="search-item" label={t('search.sections.titles')} />
              <MoreButton
                shown={items.length}
                keys={items.map((item) => cardKey('search-item', item))}
                hasMore={results.hasNextPage}
                loading={results.isFetchingNextPage}
                failed={results.isFetchNextPageError}
                label={t('search.more')}
                focusKey="search-more"
                onMore={() => void results.fetchNextPage()}
              />
            </section>
          )}

          {showResults && people.length > 0 && (
            <section className={styles.section} aria-labelledby="search-people">
              <h2 id="search-people" className={styles.sectionTitle}>
                {t('search.sections.people')}
              </h2>
              <SpatialGroup
                as="ul"
                className={styles.links}
                aria-label={t('search.sections.people')}
              >
                {people.map((person) => (
                  <li key={person.id}>
                    <SpatialButtonLink
                      focusKey={`search-person-${person.id}`}
                      href={paths.person(person.id)}
                      variant="secondary"
                    >
                      {person.name}
                    </SpatialButtonLink>
                  </li>
                ))}
              </SpatialGroup>
            </section>
          )}

          {showResults && collections.length > 0 && (
            <section className={styles.section} aria-labelledby="search-collections">
              <h2 id="search-collections" className={styles.sectionTitle}>
                {t('search.sections.collections')}
              </h2>
              <SpatialGroup
                as="ul"
                className={styles.grid}
                aria-label={t('search.sections.collections')}
              >
                {collections.map((collection) => (
                  <li key={collection.id} className={styles.cell}>
                    <CollectionCard
                      focusKey={`search-collection-${collection.id}`}
                      href={paths.collection(collection.id)}
                      name={collection.name}
                      caption={t('collection.count', { count: collection.count })}
                      posterUrl={imageUrl(collection.cover?.images.poster)}
                      posterSmallUrl={imageUrl(collection.cover?.images.posterSmall)}
                      accent={collection.cover?.accent}
                    />
                  </li>
                ))}
              </SpatialGroup>
            </section>
          )}
        </div>
      </main>
    </SpatialPage>
  );
}
