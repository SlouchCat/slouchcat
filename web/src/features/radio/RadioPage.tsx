import type { components } from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import {
  countryName,
  facetValueName,
  radioLanguageName,
  regionName,
  stationPlace,
} from '@/app/radio/labels';
import { wantsSound } from '@/app/radio/priority';
import { useRadioFavorite } from '@/app/radio/queries';
import { useRadio } from '@/app/radio/useRadio';
import { formatDate } from '@/lib/format';
import { usePagedList, type PlaceRange } from '@/lib/pagedList';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useWideScreen } from '@/lib/useWideScreen';
import {
  ActiveFilters,
  FilterChips,
  Notice,
  Row,
  RowList,
  SearchField,
  SelectField,
  SpatialButton,
  SpatialGroup,
  SpatialPage,
  Skeleton,
  StationTile,
  VirtualList,
  type ActiveFilter,
} from '@/ui';
import { FacetSelect, RankHint, RankSelect, rankTo, type FacetOption } from '@/ui/aria';
import {
  fromRadioFilter,
  hasRadioFilter,
  RADIO_FACETS,
  RADIO_SORTS,
  radioQueryParams,
  toRadioFilter,
  valuesOf,
  withValues,
  type RadioFacetKey,
  type RadioSearch,
  type RadioSort,
} from './filters';
import {
  catalogRetry,
  isCatalogLoading,
  radioFacetsQuery,
  radioFavoritesQuery,
  radioPreferencesQuery,
  radioStationsQuery,
  radioStationsSource,
  useRadioFavoriteOrder,
  useSaveRadioPreferences,
} from './queries';
import styles from './RadioPage.module.css';

type RadioStation = components['schemas']['RadioStation'];
type RadioFacets = components['schemas']['RadioFacets'];

const route = getRouteApi('/_authed/_active/_chrome/radio');

export const RADIO_SEARCH_KEY = 'radio-search';
/** Typing waits this long before the URL (and with it the request) follows. */
const DEBOUNCE_MS = 250;
/** The last filter is stored this long after the last change (per profile, F-12). */
const SAVE_DELAY_MS = 1000;
/** Missing stations leave the catalog this long after they went missing (F-18). */
const MISSING_GRACE_MS = 7 * 24 * 60 * 60 * 1000;
/** Focus key prefixes of the station tiles, one per place on the page. */
const TILE_KEYS = {
  favorite: 'radio-favorite',
  popular: 'radio-popular',
  station: 'radio-station',
} as const;
/** Tiles of the browse rows. */
const ROW_SIZE = 12;
/** A station tile's name, place and gaps under its square picture, in px (a row's estimate). */
const STATION_TEXT_HEIGHT = 64;

/** The text of one facet value in the UI language. */
function valueLabel(t: TFunction, language: string, facet: RadioFacetKey, value: string): string {
  switch (facet) {
    case 'country':
      return countryName(language, value);
    case 'region':
      return regionName(t, value);
    case 'language':
      return radioLanguageName(language, value);
    default:
      return facetValueName(t, facet, value);
  }
}

/** The values to choose for a facet, the chosen ones always among them. */
function facetOptions(
  t: TFunction,
  language: string,
  facet: RadioFacetKey,
  facets: RadioFacets | undefined,
  chosen: readonly string[],
): FacetOption[] {
  const counted = facets?.[facet] ?? [];
  const options = counted.map(({ value, count }) => ({
    value,
    label: valueLabel(t, language, facet, value),
    count,
  }));
  for (const value of chosen) {
    if (!options.some((option) => option.value === value)) {
      options.push({ value, label: valueLabel(t, language, facet, value), count: 0 });
    }
  }
  return options;
}

/**
 * The radio page (concept 16, 10.1, variant A «Katalog wie TuneIn»): search, quick filters, a
 * filter row with counts, the active filters as chips, the browse rows without a filter and the
 * grid of stations. Filter and sort live in the URL; the last filter is stored per profile and
 * comes back when the page opens without one. The client applies no visibility rule (rule 12):
 * it shows the stations the server lists.
 */
export function RadioPage() {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  useDocumentTitle(t('radio.page.title'));
  const search = route.useSearch();
  const navigate = route.useNavigate();
  const wide = useWideScreen();
  const radio = useRadio();
  const favorite = useRadioFavorite();
  const preferences = useQuery(radioPreferencesQuery);
  const savePreferences = useSaveRadioPreferences();
  const order = useRadioFavoriteOrder();
  const [ordering, setOrdering] = useState(false);

  // A change of a filter is a step of its own (Back undoes it); typing replaces the step.
  const go = (next: RadioSearch, replace = false) => void navigate({ search: next, replace });

  // The text field follows the URL when it changes from elsewhere (Back, a reset).
  const [text, setText] = useState(search.q ?? '');
  const [seenQ, setSeenQ] = useState(search.q);
  if (search.q !== seenQ) {
    setSeenQ(search.q);
    if ((search.q ?? '') !== text.trim()) setText(search.q ?? '');
  }
  useEffect(() => {
    const typed = text.trim();
    if (typed === (search.q ?? '')) return;
    const timer = window.setTimeout(() => {
      const next = { ...search };
      if (typed) next.q = typed;
      else delete next.q;
      void navigate({ search: next, replace: true });
    }, DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [navigate, search, text]);

  // Opened without a filter: the profile's last filter comes back (F-12).
  const opened = useRef(search);
  const restored = useRef(false);
  const prefs = preferences.data;
  useEffect(() => {
    if (restored.current || !prefs) return;
    restored.current = true;
    if (Object.keys(opened.current).length > 0) return;
    const last = fromRadioFilter(prefs.lastFilter, prefs.lastSort);
    if (Object.keys(last).length > 0) void navigate({ search: last, replace: true });
  }, [navigate, prefs]);

  // The filter is stored a second after its last change.
  const filter = useMemo(() => toRadioFilter(search), [search]);
  // Both sides in the same form (key order, default sort), so a stored filter is not saved again.
  const stored = JSON.stringify([
    toRadioFilter(fromRadioFilter(prefs?.lastFilter)),
    prefs?.lastSort ?? 'popular',
  ]);
  const current = JSON.stringify([filter, search.sort ?? 'popular']);
  const { mutate: save } = savePreferences;
  // The country chosen last: `prefs` has it only once the server answered, and the filter's save
  // must not put the old one back meanwhile (a slow PUT).
  const chosenCountry = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (!prefs || !restored.current || stored === current) return;
    const timer = window.setTimeout(
      () =>
        // The PUT replaces all three: always the whole object.
        save({
          defaultCountry: chosenCountry.current ?? prefs.defaultCountry,
          lastFilter: filter,
          lastSort: search.sort ?? 'popular',
        }),
      SAVE_DELAY_MS,
    );
    return () => window.clearTimeout(timer);
  }, [current, filter, prefs, save, search.sort, stored]);

  const params = useMemo(() => radioQueryParams(search), [search]);
  const filtered = hasRadioFilter(search);
  const facets = useQuery(radioFacetsQuery(params));
  const allFacets = useQuery(radioFacetsQuery(radioQueryParams({})));
  // Every station of the filter, read page by page as the grid scrolls; the grid is always
  // virtual, a country or genre has thousands (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(radioStationsSource(params, search.top === true), span, catalogRetry);
  const stations = paged.first;
  const defaultCountry = prefs?.defaultCountry;
  const favorites = useQuery({ ...radioFavoritesQuery, enabled: !filtered });
  const popularIn = useInfiniteQuery({
    ...radioStationsQuery(radioQueryParams({ country: defaultCountry }), ROW_SIZE),
    enabled: !filtered && defaultCountry !== undefined,
  });

  const loading = [stations, facets].some((query) =>
    isCatalogLoading(query.failureReason ?? query.error),
  );
  const total = paged.total;

  const note = (station: RadioStation): string | undefined => {
    const at =
      station.removalAt ??
      (station.missingSince === undefined ? undefined : station.missingSince + MISSING_GRACE_MS);
    if (station.state === 'missing' && at !== undefined) {
      return t('radio.page.missing', { date: formatDate(new Date(at).toISOString(), language) });
    }
    return station.state === 'broken' ? t('radio.page.broken') : undefined;
  };

  const tile = (station: RadioStation, focusPrefix: string) => {
    const chosen = radio.state.phase !== 'idle' && radio.state.station?.id === station.id;
    // A station that still starts counts as playing: a press pauses it (RadioProvider.toggle).
    const state = chosen ? (wantsSound(radio.state) ? 'playing' : 'paused') : undefined;
    return (
      <StationTile
        key={station.id}
        name={station.name}
        logoUrl={imageUrl(station.logo)}
        caption={stationPlace(t, language, station)}
        note={note(station)}
        state={state}
        stateLabel={state && t(`radio.page.${state}`)}
        playLabel={
          state === 'playing'
            ? t('radio.page.pause', { name: station.name })
            : state === 'paused'
              ? t('radio.page.resume', { name: station.name })
              : t('radio.page.play', { name: station.name })
        }
        onPlay={() => (chosen ? radio.toggle() : radio.play(station))}
        favorite={{
          pressed: station.favorite,
          label: t('radio.player.favorite', { name: station.name }),
          onPress: () => favorite.mutate({ stationId: station.id, favorite: !station.favorite }),
        }}
        focusKey={`${focusPrefix}-${station.id}`}
      />
    );
  };

  // Quick filters: the profile's country, its favourites, the most popular stations.
  const countries = valuesOf(search, 'country');
  const myCountryOn =
    defaultCountry !== undefined && countries.length === 1 && countries[0] === defaultCountry;
  const quickValues = [
    ...(myCountryOn ? ['country'] : []),
    ...(search.fav ? ['fav'] : []),
    ...(search.top ? ['top'] : []),
  ];
  const quickOptions = [
    ...(defaultCountry
      ? [
          {
            value: 'country',
            label: t('radio.page.quick.myCountry', {
              country: countryName(language, defaultCountry),
            }),
          },
        ]
      : []),
    { value: 'fav', label: t('radio.page.quick.favorites') },
    { value: 'top', label: t('radio.page.quick.popular') },
  ];
  const onQuick = (values: string[]) => {
    let next: RadioSearch = { ...search };
    if (defaultCountry && values.includes('country') !== myCountryOn) {
      next = withValues(next, 'country', values.includes('country') ? [defaultCountry] : []);
    }
    if (values.includes('fav')) next.fav = true;
    else delete next.fav;
    if (values.includes('top')) next.top = true;
    else delete next.top;
    go(next);
  };

  // «Mein Land»: another default country for this profile (F-12).
  const countryChoices = [
    ...new Set([...(allFacets.data?.country ?? []).map((entry) => entry.value), defaultCountry]),
  ]
    .filter((code): code is string => Boolean(code))
    .map((code) => ({ value: code, label: countryName(language, code) }))
    .sort((a, b) => a.label.localeCompare(b.label, language));
  const changeCountry = (code: string) => {
    if (!prefs || code === (chosenCountry.current ?? prefs.defaultCountry)) return;
    chosenCountry.current = code;
    save({
      defaultCountry: code,
      lastFilter: prefs.lastFilter ?? {},
      lastSort: prefs.lastSort ?? 'popular',
    });
    if (myCountryOn) go(withValues(search, 'country', [code]));
  };

  // The active filters as chips.
  const active: ActiveFilter[] = [];
  for (const facet of RADIO_FACETS) {
    for (const value of valuesOf(search, facet)) {
      const label = valueLabel(t, language, facet, value);
      active.push({
        key: `${facet}-${value}`,
        label,
        removeLabel: t('radio.page.filters.remove', { label }),
        onRemove: () =>
          go(
            withValues(
              search,
              facet,
              valuesOf(search, facet).filter((v) => v !== value),
            ),
          ),
      });
    }
  }
  const flag = (key: 'titles' | 'fav' | 'top', label: string) => {
    if (!search[key]) return;
    active.push({
      key,
      label,
      removeLabel: t('radio.page.filters.remove', { label }),
      onRemove: () => {
        const next = { ...search };
        delete next[key];
        go(next);
      },
    });
  };
  flag('titles', t('radio.page.filters.titles'));
  flag('fav', t('radio.page.quick.favorites'));
  flag('top', t('radio.page.quick.popular'));
  if (search.q) {
    const label = t('radio.page.filters.search', { q: search.q });
    active.push({
      key: 'q',
      label,
      removeLabel: t('radio.page.filters.remove', { label }),
      onRemove: () => {
        const next = { ...search };
        delete next.q;
        setText('');
        go(next);
      },
    });
  }
  const reset = () => {
    setText('');
    go(search.sort ? { sort: search.sort } : {});
  };

  const sheet = wide
    ? undefined
    : {
        closeLabel: t('radio.page.filters.close'),
        showLabel: t('radio.page.filters.show', { count: total }),
      };
  const shownFacets = RADIO_FACETS.filter((facet) => facet !== 'region' || countries.length > 0);

  const favoriteList = favorites.data ?? [];
  const genres = (allFacets.data?.genre ?? []).slice(0, ROW_SIZE);
  const moods = allFacets.data?.mood ?? [];
  const popular = popularIn.data?.pages[0]?.items ?? [];

  return (
    <SpatialPage defaultFocusKey={RADIO_SEARCH_KEY}>
      <main className={styles.page} data-testid="radio-page">
        <div className={styles.head}>
          <h1 className={styles.title}>{t('radio.page.title')}</h1>
          <p className={styles.intro}>{t('radio.page.intro')}</p>
        </div>

        <section className={styles.controls} aria-label={t('radio.page.filters.label')}>
          <SpatialGroup focusKey="radio-search-box">
            <SearchField
              label={t('radio.page.searchLabel')}
              placeholder={t('radio.page.searchPlaceholder')}
              value={text}
              onChange={setText}
              focusKey={RADIO_SEARCH_KEY}
            />
          </SpatialGroup>
          <SpatialGroup focusKey="radio-quick" className={styles.quick}>
            <FilterChips
              label={t('radio.page.quick.label')}
              focusKey="radio-quick-chip"
              options={quickOptions}
              values={quickValues}
              onChange={onQuick}
            />
            {countryChoices.length > 0 && defaultCountry && (
              <SelectField
                label={t('radio.page.myCountry')}
                value={defaultCountry}
                options={countryChoices}
                onChange={changeCountry}
                focusKey="radio-my-country"
              />
            )}
          </SpatialGroup>
          <SpatialGroup focusKey="radio-facets" className={styles.facets} ordered>
            {shownFacets.map((facet) => (
              <FacetSelect
                key={facet}
                label={t(`radio.page.filters.${facet}`)}
                options={facetOptions(t, language, facet, facets.data, valuesOf(search, facet))}
                values={valuesOf(search, facet)}
                onChange={(values) => go(withValues(search, facet, values))}
                emptyLabel={t('radio.page.filters.none')}
                clearLabel={t('radio.page.filters.clear')}
                doneLabel={t('radio.page.filters.done')}
                sheet={sheet}
                focusKey={`radio-facet-${facet}`}
              />
            ))}
            <FilterChips
              label={t('radio.page.filters.titles')}
              focusKey="radio-titles"
              options={[{ value: 'titles', label: t('radio.page.filters.titles') }]}
              values={search.titles ? ['titles'] : []}
              onChange={(values) => {
                const next = { ...search };
                if (values.length > 0) next.titles = true;
                else delete next.titles;
                go(next);
              }}
            />
          </SpatialGroup>
          <ActiveFilters
            label={t('radio.page.filters.active')}
            filters={active}
            resetLabel={t('radio.page.filters.reset')}
            onReset={reset}
            focusKey="radio-active"
          />
        </section>

        {loading && (
          <Notice title={t('radio.page.loading.title')}>{t('radio.page.loading.body')}</Notice>
        )}

        {!filtered && !loading && (
          <>
            <section className={styles.section} aria-labelledby="radio-favorites">
              <div className={styles.sectionHead}>
                <h2 id="radio-favorites" className={styles.sectionTitle}>
                  {t('radio.page.rows.favorites')}
                </h2>
                {favoriteList.length > 1 && (
                  <SpatialGroup>
                    <SpatialButton
                      variant="ghost"
                      size="sm"
                      aria-expanded={ordering}
                      onPress={() => setOrdering((value) => !value)}
                    >
                      {ordering ? t('radio.page.order.done') : t('radio.page.order.toggle')}
                    </SpatialButton>
                  </SpatialGroup>
                )}
              </div>
              {favoriteList.length === 0 ? (
                <p className={styles.hint}>{t('radio.page.rows.noFavorites')}</p>
              ) : ordering ? (
                <ol className={styles.order} aria-label={t('radio.page.order.label')}>
                  <RankHint id="radio-order-hint" />
                  {favoriteList.map((station, position) => (
                    <li key={station.id} className={styles.orderRow}>
                      <span>{station.name}</span>
                      <RankSelect
                        name={station.name}
                        position={position}
                        count={favoriteList.length}
                        describedBy="radio-order-hint"
                        onChange={(to) =>
                          order.mutate(
                            rankTo(favoriteList, position, to, 'move').map((entry) => entry.id),
                          )
                        }
                      />
                    </li>
                  ))}
                </ol>
              ) : (
                <RowList focusKey="radio-row-favorites" aria-label={t('radio.page.rows.favorites')}>
                  {favoriteList.map((station) => tile(station, TILE_KEYS.favorite))}
                </RowList>
              )}
            </section>

            {defaultCountry && popular.length > 0 && (
              <Row
                title={t('radio.page.rows.popularIn', {
                  country: countryName(language, defaultCountry),
                })}
                focusKey="radio-row-popular"
                moreHref={`${paths.radio}?country=${defaultCountry}`}
                moreLabel={t('radio.page.rows.all', {
                  country: countryName(language, defaultCountry),
                })}
              >
                {popular.map((station) => tile(station, TILE_KEYS.popular))}
              </Row>
            )}

            {genres.length > 0 && (
              <Row title={t('radio.page.rows.byGenre')} focusKey="radio-row-genres" variant="chips">
                {genres.map((entry) => (
                  <SpatialButton
                    key={entry.value}
                    variant="secondary"
                    onPress={() => go(withValues(search, 'genre', [entry.value]))}
                  >
                    {facetValueName(t, 'genre', entry.value)}
                    <span className={styles.tileCount}>
                      {t('radio.page.rows.stations', { count: entry.count })}
                    </span>
                  </SpatialButton>
                ))}
              </Row>
            )}

            {moods.length > 0 && (
              <Row title={t('radio.page.rows.byMood')} focusKey="radio-row-moods" variant="chips">
                {moods.map((entry) => (
                  <SpatialButton
                    key={entry.value}
                    variant="secondary"
                    onPress={() => go(withValues(search, 'mood', [entry.value]))}
                  >
                    {facetValueName(t, 'mood', entry.value)}
                    <span className={styles.tileCount}>
                      {t('radio.page.rows.stations', { count: entry.count })}
                    </span>
                  </SpatialButton>
                ))}
              </Row>
            )}
          </>
        )}

        <section className={styles.section} aria-labelledby="radio-results">
          <div className={styles.sectionHead}>
            <h2 id="radio-results" className={styles.sectionTitle}>
              {filtered ? t('radio.page.results') : t('radio.page.allStations')}
            </h2>
            <span className={styles.count} role="status" data-testid="radio-count">
              {stations.isSuccess ? t('radio.page.count', { count: total }) : ''}
            </span>
            <SpatialGroup focusKey="radio-sort">
              <SelectField<RadioSort>
                label={t('radio.page.sort.label')}
                value={search.sort ?? 'popular'}
                options={RADIO_SORTS.map((sort) => ({
                  value: sort,
                  label: t(`radio.page.sort.${sort}`),
                }))}
                onChange={(sort) => {
                  const next = { ...search };
                  if (sort === 'popular') delete next.sort;
                  else next.sort = sort;
                  go(next);
                }}
              />
            </SpatialGroup>
          </div>

          {stations.isError && !loading && (
            <Notice tone="danger" title={problemMessage(t, stations.error)} />
          )}

          {stations.isSuccess && total === 0 && (
            <Notice
              title={t('radio.page.empty.title')}
              actions={
                filtered && (
                  <SpatialGroup>
                    <SpatialButton variant="secondary" onPress={reset}>
                      {t('radio.page.filters.reset')}
                    </SpatialButton>
                  </SpatialGroup>
                )
              }
            >
              {t('radio.page.empty.body')}
            </Notice>
          )}

          {total > 0 && (
            <VirtualList
              slots={paged.slots}
              virtual
              layout="grid"
              onRange={setSpan}
              className={styles.grid}
              cellClassName={styles.cell}
              label={filtered ? t('radio.page.results') : t('radio.page.allStations')}
              focusKey="radio-grid"
              estimate={(width) => width + STATION_TEXT_HEIGHT}
              itemKey={(station) => station.id}
              renderPlaceholder={() => (
                <span className={styles.placeholder}>
                  <Skeleton shape="poster" />
                </span>
              )}
              renderItem={(station) => tile(station, TILE_KEYS.station)}
            />
          )}

          {paged.failed && (
            <Notice
              tone="danger"
              title={t('library.page.pageFailed')}
              actions={
                <SpatialGroup>
                  <SpatialButton variant="secondary" icon="refresh" onPress={paged.retry}>
                    {t('common.retry')}
                  </SpatialButton>
                </SpatialGroup>
              }
            />
          )}
        </section>
      </main>
    </SpatialPage>
  );
}
