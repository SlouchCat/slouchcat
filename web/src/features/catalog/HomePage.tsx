import type { components, HomeRow, ItemRef } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { countryName, facetValueName } from '@/app/radio/labels';
import { wantsSound } from '@/app/radio/priority';
import { useRadioFavorite } from '@/app/radio/queries';
import { useRadio, type RadioPlayer } from '@/app/radio/useRadio';
import { formatRuntime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Button,
  ButtonLink,
  CollectionCard,
  dismissHeroPrepaint,
  dismissPosterPrepaint,
  GenreChip,
  HERO_PLAY_KEY,
  HeroCarousel,
  LibraryTile,
  Notice,
  PosterCard,
  Row,
  Skeleton,
  setSpatialFocus,
  SpatialPage,
  StationTile,
  Text,
  VisuallyHidden,
  type HeroSlide,
} from '@/ui';
import { CoverWatchlist, WATCHLIST_KINDS } from '@/features/discover/CoverWatchlist';
import { CoverPen, EDITABLE_KINDS, EditMetadata } from './EditMetadata';
import { ItemMeta } from './ItemMeta';
import { homeRowTitle, isAudioKind } from './labels';
import { CoverWatched, WATCHED_KINDS } from './watched';
import { homeQuery } from './queries';
import { usePlayStart } from './usePlayStart';
import styles from './HomePage.module.css';

const authed = getRouteApi('/_authed/_active');

// The trailer dialog needs react-aria, which the start page does not load up front (P-10).
const TrailerPanel = lazy(() =>
  import('./TrailerPanel').then((module) => ({ default: module.TrailerPanel })),
);

// Rows with a page of all their titles ("Alle anzeigen ›", concept 08 O-3), and a library row
// («Neu in …», concept 15 P-7), whose library opens sorted by «Zuletzt hinzugefügt».
// The page behind each row title (concept 20 P-7, 8.2); genres and «Meine Medien» have none, each
// of their tiles is a target already.
const rowPages: Partial<Record<HomeRow['key'], string>> = {
  spotlight: paths.homeList('spotlight'),
  continue: paths.homeList('continue'),
  recent: paths.homeList('recent'),
  released: paths.homeList('released'),
  collections: paths.collections,
  picks: paths.picks,
  watchlist: paths.watchlist,
  radio: paths.radio,
};
const rowPage = (row: HomeRow): string | undefined =>
  row.key === 'library' && row.libraryId !== undefined
    ? `${paths.library(row.libraryId)}?sort=added`
    : rowPages[row.key];

// Posters loaded eagerly: the cards of the first row that fit a wide screen (P-10); the rest are lazy.
// The server paints as many ahead on a page without a stage (posterPrepaintCards, api/preload.go).
const EAGER_POSTERS = 6;
// Tiles of «Meine Medien» loaded eagerly: two cards wide each, so as wide as EAGER_POSTERS cards; the
// server paints as many ahead when the row comes first (tilePrepaintTiles, api/preload.go).
const EAGER_TILES = 3;

/**
 * A row counts when it holds titles, collections, genres or library tiles (E-9: never an empty
 * row; «Meine Medien» counts its tiles, concept 15 P-7).
 */
const hasEntries = (row: HomeRow) =>
  row.items.length > 0 ||
  (row.collections?.length ?? 0) > 0 ||
  (row.genres?.length ?? 0) > 0 ||
  (row.libraries?.length ?? 0) > 0 ||
  (row.stations?.length ?? 0) > 0;

/** Focus key of the first card of a row (the default focus when the stage does not lead). */
function firstKey(row: HomeRow): string | undefined {
  const item = row.items[0];
  if (item) return cardKey(row, item);
  const collection = row.collections?.[0];
  if (collection) return `${row.id}-${collection.id}`;
  const tile = row.libraries?.[0];
  if (tile) return `${row.id}-${tile.id}`;
  const station = row.stations?.[0];
  if (station) return `${row.id}-${station.station.id}`;
  const genre = row.genres?.[0];
  return genre ? `${row.id}-${genre.name}` : undefined;
}

type RadioHomeStation = components['schemas']['RadioHomeStation'];

/**
 * The line under a station of «Meine Sender» (concept 16, 10.2): the title its relay plays at the
 * live edge, while one runs; otherwise country and genre (D-7: nothing is kept for a station
 * nobody listens to).
 */
function stationCaption(t: TFunction, language: string, tile: RadioHomeStation) {
  const { station, now } = tile;
  if (now) {
    return now.artist && now.title
      ? t('radio.player.artistTitle', { artist: now.artist, title: now.title })
      : now.text;
  }
  const parts = [
    station.country && countryName(language, station.country),
    station.genres[0] && facetValueName(t, 'genre', station.genres[0]),
  ].filter(Boolean);
  return parts.length > 0 ? parts.join(t('format.separator')) : undefined;
}

/**
 * A station in the app's radio: `playing` (also while it still starts, a press pauses it),
 * `paused`, or undefined when another or none is chosen.
 */
function stationState(radio: RadioPlayer, stationId: number) {
  if (radio.state.phase === 'idle' || radio.state.station?.id !== stationId) return undefined;
  return wantsSound(radio.state) ? ('playing' as const) : ('paused' as const);
}

const genreHref = (name: string) => `${paths.search}?genre=${encodeURIComponent(name)}`;

/** "Noch 20 min": the time left of a started title. */
function remaining(t: TFunction, progress: NonNullable<ItemRef['progress']>): string {
  const minutes = Math.max(1, Math.round((progress.durationMs - progress.positionMs) / 60_000));
  return t('home.remaining', { time: formatRuntime(t, minutes) });
}

/**
 * The series a card of "Weiterschauen" shows instead of its episode (concept 15 P-1): its poster
 * and title. Undefined for every other card and for an episode without `seriesRef`, which show the
 * title itself; the other rows stay as they are.
 */
const seriesOf = (row: HomeRow, item: ItemRef) =>
  row.key === 'continue' && item.kind === 'episode' ? item.seriesRef : undefined;

/**
 * The title a card's pen edits (concept 15 P-6): the title of the cover it sits on. On a series
 * card of "Weiterschauen" that is the series, whose poster and title the card shows, not the
 * episode the card opens.
 */
function editedTitle(row: HomeRow, item: ItemRef): Pick<ItemRef, 'id' | 'kind' | 'title'> {
  const series = seriesOf(row, item);
  return series ? { id: series.id, kind: 'series', title: series.title } : item;
}

/**
 * Focus and React key of a card, by the row's id (several library rows share a key, concept 15
 * P-7). A series card of "Weiterschauen" keys on its series, not on the episode: after the episode
 * ends the card shows the next one and keeps its focus on return.
 */
const cardKey = (row: HomeRow, item: ItemRef) => `${row.id}-${seriesOf(row, item)?.id ?? item.id}`;

/**
 * The progress of a started title: a position past 0. A next episode with a progress row at 0
 * (opened, never played) counts as new, without "Noch …" and without a bar.
 */
const startedProgress = (item: ItemRef) =>
  item.progress && item.progress.positionMs > 0 ? item.progress : undefined;

/** "S4 · F7", or "S4 · F7–F8" for a file with several episodes; undefined without numbers. */
function shortEpisodeCode(t: TFunction, item: ItemRef): string | undefined {
  if (item.seasonNumber === undefined || item.episodeNumber === undefined) return undefined;
  const numbers = { season: item.seasonNumber, episode: item.episodeNumber };
  return item.episodeEnd !== undefined && item.episodeEnd > item.episodeNumber
    ? t('home.continue.episodeRange', { ...numbers, end: item.episodeEnd })
    : t('home.continue.episode', numbers);
}

/**
 * Title, caption and spoken name of a series card (seriesOf): "S4 · F7 · Noch 20 min" under the
 * series title, "S4 · F8 · Neu" for the next episode not started yet; the name adds the episode's
 * own title ("Die Simpsons, S4 · F7, Donuts im Stau, Noch 20 min").
 */
function seriesCardText(t: TFunction, series: { title: string }, item: ItemRef) {
  const code = shortEpisodeCode(t, item);
  const progress = startedProgress(item);
  const state = progress ? remaining(t, progress) : t('home.continue.new');
  return {
    title: series.title,
    caption: [code, state].filter(Boolean).join(t('format.separator')),
    label: [series.title, code, item.title, state].filter(Boolean).join(t('format.listSeparator')),
  };
}

function cardCaption(t: TFunction, row: HomeRow, item: ItemRef): string | undefined {
  // A curated spotlight title carries the main user's note (M11); it says more than the year.
  if (item.note) return item.note;
  const progress = startedProgress(item);
  if (row.key === 'continue' && progress) return remaining(t, progress);
  // An audiobook or a podcast is known by its author more than by its year (M6).
  if (isAudioKind(item.kind) && item.author) return item.author;
  return item.year !== undefined ? String(item.year) : undefined;
}

function HomeSkeleton() {
  return (
    <div className={styles.skeleton} aria-hidden="true">
      <Skeleton shape="hero" />
      <div className={styles.skeletonRow}>
        {Array.from({ length: 7 }, (_, index) => (
          <span key={index} className={styles.skeletonCard}>
            <Skeleton shape="poster" />
          </span>
        ))}
      </div>
    </div>
  );
}

/**
 * Start page (E-1): the rows in the order the server returned: titles, collections (E-6) and
 * genres, which open the search. The row the server names in `heroRow` is drawn as the stage (hero
 * carousel) at its place in that order; the client never decides which row that is (3.5). Without
 * `heroRow` every row is a poster row.
 */
export function HomePage() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const radio = useRadio();
  const favorite = useRadioFavorite();
  useDocumentTitle(t('home.title'));
  const { session } = authed.useRouteContext();
  const home = useQuery(homeQuery);
  // Posters wait until the hero artwork is on screen: on a slow link they would share its
  // bandwidth (P-10). The frames keep their size, so nothing shifts.
  const [heroShown, setHeroShown] = useState(false);
  // The stage slide whose trailer dialog is open (W-10).
  const [trailerSlide, setTrailerSlide] = useState<HeroSlide>();
  // The pen on the covers (concept 15 P-6), as in the library: a shortcut for the administrator's
  // primary profile, not a rule (the server answers the editor only for that session).
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  // The title whose metadata editor is open.
  const [editing, setEditing] = useState<string | null>(null);
  // «Abspielen» of a started film in the spotlight asks whether to resume (concept 20 P-4). Not for
  // a series: its card does not know whether its next episode is started.
  const playStart = usePlayStart();
  // Only a stage at the top is the page's first image: the server paints it into the shell, and the
  // posters wait for it. A stage further down has no prepaint and holds no poster back.
  const leadRow = home.data?.rows.find(hasEntries);
  // The band begins with the server's draw (heroStart, concept 08, F-2), the title the shell painted.
  const firstHero =
    leadRow && leadRow.key === home.data?.heroRow
      ? leadRow.items[Math.min(home.data.heroStart ?? 0, leadRow.items.length - 1)]
      : undefined;
  const heroHasArt = Boolean(
    firstHero &&
    (firstHero.images.backdrop ?? firstHero.images.backdropSmall ?? firstHero.images.poster),
  );
  // The server painted the first stage image into the shell (P-10); it gives way once the carousel
  // shows its own image, or right away when the page has no stage at the top or no art for it.
  const prepaintDone = home.isError || (home.isSuccess && (!heroHasArt || heroShown));
  useEffect(() => {
    if (prepaintDone) dismissHeroPrepaint();
  }, [prepaintDone]);
  // Without a stage the server painted the first cards of the first row into the shell instead
  // (P-10); they give way once the first card shows its own poster, or right away when the page
  // has a stage, starts with collections or genres, or its first title has no poster.
  const [posterShown, setPosterShown] = useState(false);
  // «Meine Medien» on top paints the collages of its first tiles; its first poster leads.
  const leadCard = home.data?.heroRow
    ? undefined
    : (leadRow?.items[0] ?? leadRow?.libraries?.[0]?.items[0]);
  // A series card shows the series poster, which the server paints ahead as well (cardPoster).
  const leadImages = leadRow && leadCard && (seriesOf(leadRow, leadCard) ?? leadCard).images;
  // «Meine Sender» on top paints the logos of its first stations (B3); the first logo leads.
  const leadStation = home.data?.heroRow ? undefined : leadRow?.stations?.[0]?.station;
  const leadCardHasArt =
    Boolean(leadImages && (leadImages.poster ?? leadImages.posterSmall)) ||
    Boolean(leadStation?.logo);
  const posterPrepaintDone = home.isError || (home.isSuccess && (!leadCardHasArt || posterShown));
  useEffect(() => {
    if (posterPrepaintDone) dismissPosterPrepaint();
  }, [posterPrepaintDone]);

  if (home.isPending) {
    return (
      <main className={styles.home} aria-busy="true">
        <VisuallyHidden as="h1">{t('home.title')}</VisuallyHidden>
        <HomeSkeleton />
      </main>
    );
  }

  if (home.isError) {
    return (
      <main className={`${styles.home} ${styles.padded}`}>
        <VisuallyHidden as="h1">{t('home.title')}</VisuallyHidden>
        <Notice
          tone="danger"
          title={problemMessage(t, home.error)}
          actions={
            <Button variant="primary" icon="refresh" onPress={() => void home.refetch()}>
              {t('common.retry')}
            </Button>
          }
        />
      </main>
    );
  }

  // The server omits empty rows (E-9); filter defensively so an empty row never renders.
  const rows = home.data.rows.filter(hasEntries);
  const isStage = (row: HomeRow | undefined) =>
    row !== undefined && row.key === home.data.heroRow && row.items.length > 0;
  const stageLeads = isStage(rows[0]);
  // The focus starts where the page starts: on the stage's play button when the stage is on top,
  // else on the first card of the first row, also when the stage stands further down the list.
  const defaultFocusKey = stageLeads ? HERO_PLAY_KEY : ((rows[0] && firstKey(rows[0])) ?? 'header');
  // The eager posters belong to the first poster row, wherever the stage stands.
  const firstPosterRow = rows.find((row) => !isStage(row));
  // Without a stage the first card is the page's first image: it is fetched with high priority,
  // like the server's preload of it (P-10). With a stage the band goes first and no card competes.
  const posterLeads = !rows.some(isStage);

  const slides = (row: HomeRow): HeroSlide[] =>
    row.items.map((item) => ({
      id: item.id,
      title: item.title,
      logoUrl: imageUrl(item.images.logo),
      backdropUrl: imageUrl(item.images.backdrop),
      backdropSmallUrl: imageUrl(item.images.backdropSmall),
      backdropLargeUrl: imageUrl(item.images.backdropLarge),
      backdropUhdUrl: imageUrl(item.images.backdropUhd),
      backdropUhdWidth: item.images.backdropUhd?.width,
      posterUrl: imageUrl(item.images.poster),
      posterSmallUrl: imageUrl(item.images.posterSmall),
      accent: item.accent,
      caption: cardCaption(t, row, item),
      meta: <ItemMeta item={item} />,
      note: item.note,
      hasTrailer: item.hasTrailer,
      playHref: paths.play(item.id),
      detailsHref: paths.item(item.id),
    }));

  // The pen of a spotlight poster: only the active poster's pen is a Tab stop, like the poster
  // itself (the row's roving tabindex). Not on the stage above, which shows one title at a time.
  const spotlightPen = (row: HomeRow, slide: HeroSlide, active: boolean) => {
    const item = row.items.find((entry) => entry.id === slide.id);
    return item && EDITABLE_KINDS.has(item.kind) ? (
      <CoverPen item={item} onEdit={setEditing} excludeFromTabOrder={!active} />
    ) : undefined;
  };

  // The watchlist button of a spotlight poster (concept 17 D-9): a Tab stop only on the active
  // poster, like its pen.
  const spotlightMark = (row: HomeRow, slide: HeroSlide, active: boolean) => {
    const item = row.items.find((entry) => entry.id === slide.id);
    return item && WATCHLIST_KINDS.has(item.kind) ? (
      <CoverWatchlist item={item} excludeFromTabOrder={!active} />
    ) : undefined;
  };

  // The check mark of a spotlight poster (concept 20 P-5): a Tab stop only on the active poster.
  const spotlightCheck = (row: HomeRow, slide: HeroSlide, active: boolean) => {
    const item = row.items.find((entry) => entry.id === slide.id);
    return item && WATCHED_KINDS.has(item.kind) ? (
      <CoverWatched item={item} excludeFromTabOrder={!active} />
    ) : undefined;
  };

  // A title taken off the watchlist from its cover in the row «Merkliste»: the focus goes to the
  // neighbouring card (or the header) before the card disappears, as on the watchlist page.
  const unmarkedFrom = (row: HomeRow, item: ItemRef) => {
    if (row.key !== 'watchlist') return undefined;
    return () => {
      const index = row.items.findIndex((entry) => entry.id === item.id);
      const neighbour = row.items[index + 1] ?? row.items[index - 1];
      setSpatialFocus(neighbour ? cardKey(row, neighbour) : 'header');
    };
  };

  const posterRow = (row: HomeRow) => {
    const title = homeRowTitle(t, row);
    const focusKey = `row-${row.id}`;
    const eagerRow = row === firstPosterRow;
    if (row.key === 'libraries') {
      // «Meine Medien» (concept 15 P-7): a tile per library, which opens the library page.
      return (
        <Row key={row.id} title={title} focusKey={focusKey} variant="tiles">
          {(row.libraries ?? []).map((tile, tileIndex) => {
            const leads = posterLeads && eagerRow && tileIndex === 0;
            return (
              <LibraryTile
                key={tile.id}
                focusKey={`${row.id}-${tile.id}`}
                href={paths.library(tile.id)}
                name={tile.name}
                caption={t('library.page.count', {
                  count: tile.itemCount,
                  noun: t(`library.page.noun.${tile.kind}`, { count: tile.itemCount }),
                })}
                covers={tile.items.map((item) => ({
                  posterUrl: imageUrl(item.images.poster),
                  posterSmallUrl: imageUrl(item.images.posterSmall),
                }))}
                accent={tile.items[0]?.accent}
                eager={eagerRow && tileIndex < EAGER_TILES}
                priority={leads}
                onImageShown={leads ? () => setPosterShown(true) : undefined}
                deferImage={heroHasArt && !heroShown}
              />
            );
          })}
        </Row>
      );
    }
    if (row.key === 'radio') {
      // «Meine Sender» (concept 16, 10.2): square station tiles that play through the app's radio.
      return (
        <Row key={row.id} title={title} focusKey={focusKey} href={rowPage(row)}>
          {(row.stations ?? []).map((tile, tileIndex) => {
            const { station } = tile;
            const state = stationState(radio, station.id);
            const leads = posterLeads && eagerRow && tileIndex === 0;
            return (
              <StationTile
                key={station.id}
                focusKey={`${row.id}-${station.id}`}
                name={station.name}
                logoUrl={imageUrl(station.logo)}
                caption={stationCaption(t, i18n.language, tile)}
                state={state}
                stateLabel={state && t(`radio.page.${state}`)}
                playLabel={
                  state === 'playing'
                    ? t('radio.page.pause', { name: station.name })
                    : state === 'paused'
                      ? t('radio.page.resume', { name: station.name })
                      : t('radio.page.play', { name: station.name })
                }
                onPlay={() => (state ? radio.toggle() : radio.play(station))}
                favorite={{
                  pressed: station.favorite,
                  label: t('radio.player.favorite', { name: station.name }),
                  onPress: () =>
                    favorite.mutate(
                      { stationId: station.id, favorite: !station.favorite },
                      // The row lists the favourites: it follows the change.
                      { onSettled: () => void queryClient.invalidateQueries(homeQuery) },
                    ),
                }}
                priority={leads}
                onImageShown={leads ? () => setPosterShown(true) : undefined}
              />
            );
          })}
        </Row>
      );
    }
    if (row.key === 'genres') {
      return (
        <Row key={row.id} title={title} focusKey={focusKey} variant="chips">
          {(row.genres ?? []).map((genre) => (
            <GenreChip
              key={genre.name}
              focusKey={`${row.id}-${genre.name}`}
              href={genreHref(genre.name)}
              name={genre.name}
              count={genre.count}
            />
          ))}
        </Row>
      );
    }
    if (row.key === 'collections') {
      return (
        <Row key={row.id} title={title} focusKey={focusKey} href={rowPage(row)}>
          {(row.collections ?? []).map((collection, itemIndex) => (
            <CollectionCard
              key={collection.id}
              focusKey={`${row.id}-${collection.id}`}
              href={paths.collection(collection.id)}
              name={collection.name}
              caption={t('collection.count', { count: collection.count })}
              imageUrl={imageUrl(collection.image)}
              posterUrl={imageUrl(collection.cover?.images.poster)}
              posterSmallUrl={imageUrl(collection.cover?.images.posterSmall)}
              accent={collection.cover?.accent}
              eager={eagerRow && itemIndex < EAGER_POSTERS}
              deferImage={heroHasArt && !heroShown}
            />
          ))}
        </Row>
      );
    }
    return (
      <Row key={row.id} title={title} focusKey={focusKey} href={rowPage(row)}>
        {row.items.map((item, itemIndex) => {
          const series = seriesOf(row, item);
          const text = series
            ? seriesCardText(t, series, item)
            : { title: item.title, caption: cardCaption(t, row, item), label: undefined };
          const images = (series ?? item).images;
          const progress = row.key === 'continue' ? startedProgress(item) : undefined;
          const edited = editedTitle(row, item);
          return (
            <PosterCard
              key={cardKey(row, item)}
              focusKey={cardKey(row, item)}
              // A series card leads to its episode too, with "Fortsetzen" in focus (F-1).
              href={paths.item(item.id)}
              title={text.title}
              caption={text.caption}
              label={text.label}
              posterUrl={imageUrl(images.poster)}
              posterSmallUrl={imageUrl(images.posterSmall)}
              accent={item.accent}
              eager={eagerRow && itemIndex < EAGER_POSTERS}
              priority={posterLeads && eagerRow && itemIndex === 0}
              onImageShown={
                posterLeads && eagerRow && itemIndex === 0 ? () => setPosterShown(true) : undefined
              }
              deferImage={heroHasArt && !heroShown}
              shape={isAudioKind(item.kind) ? 'square' : 'poster'}
              progress={
                progress
                  ? {
                      percent: progress.percent,
                      label: t(isAudioKind(item.kind) ? 'item.listened' : 'item.watched', {
                        percent: Math.round(progress.percent),
                      }),
                    }
                  : undefined
              }
              tool={
                canManage && EDITABLE_KINDS.has(edited.kind) ? (
                  <CoverPen item={edited} onEdit={setEditing} />
                ) : undefined
              }
              // The check of the title the card opens: on a series card of "Weiterschauen" the
              // episode (concept 20 P-5), which then gives way to the next one.
              check={WATCHED_KINDS.has(item.kind) ? <CoverWatched item={item} /> : undefined}
              // A series card of "Weiterschauen" shows an episode: no watchlist button (kinds).
              mark={
                WATCHLIST_KINDS.has(item.kind) ? (
                  <CoverWatchlist item={item} onRemoved={unmarkedFrom(row, item)} />
                ) : undefined
              }
            />
          );
        })}
      </Row>
    );
  };

  return (
    <SpatialPage defaultFocusKey={defaultFocusKey}>
      <main className={styles.home}>
        <VisuallyHidden as="h1">{t('home.title')}</VisuallyHidden>
        {rows.length > 0 && (
          // The rows start below the header, the framed band too when it leads (#hero-prepaint in
          // global.css paints its first image at that place).
          <div className={`${styles.rows} ${styles.padded}`}>
            {rows.map((row) => {
              if (isStage(row)) {
                return (
                  <HeroCarousel
                    key={row.id}
                    label={homeRowTitle(t, row)}
                    href={rowPage(row)}
                    slides={slides(row)}
                    playLabel={t('item.play')}
                    detailsLabel={t('item.details')}
                    trailerLabel={t('item.trailer')}
                    rowLabel={t('home.spotlight.row')}
                    pauseLabel={t('home.spotlight.pause')}
                    resumeLabel={t('home.spotlight.resume')}
                    // Read once when the band mounts (keyed by the row): a refetch never moves it.
                    start={home.data.heroStart}
                    intervalSeconds={home.data.heroIntervalSeconds}
                    onTrailerPress={setTrailerSlide}
                    onPlayPress={(slide) => {
                      const item = row.items.find((entry) => entry.id === slide.id);
                      return item?.kind === 'movie' && playStart.ask(item);
                    }}
                    paused={trailerSlide !== undefined || editing !== null || playStart.asking}
                    onArtShown={() => setHeroShown(true)}
                    tool={
                      canManage ? (slide, active) => spotlightPen(row, slide, active) : undefined
                    }
                    mark={(slide, active) => spotlightMark(row, slide, active)}
                    check={(slide, active) => spotlightCheck(row, slide, active)}
                  />
                );
              }
              return posterRow(row);
            })}
          </div>
        )}
        {rows.length === 0 && (
          <div className={`${styles.empty} ${styles.padded}`}>
            <Text as="h2" variant="title">
              {t('home.empty.title')}
            </Text>
            <Text tone="secondary">{t('home.empty.body')}</Text>
            {session.account.role === 'admin' && (
              <div>
                <ButtonLink href={paths.adminLibraries} variant="secondary" icon="folder">
                  {t('home.empty.action')}
                </ButtonLink>
              </div>
            )}
          </div>
        )}
        {editing && <EditMetadata itemId={editing} onClose={() => setEditing(null)} />}
        {playStart.dialog}
        {trailerSlide && (
          <Suspense fallback={null}>
            <TrailerPanel
              itemId={trailerSlide.id}
              title={trailerSlide.title}
              onClose={() => setTrailerSlide(undefined)}
            />
          </Suspense>
        )}
      </main>
    </SpatialPage>
  );
}
