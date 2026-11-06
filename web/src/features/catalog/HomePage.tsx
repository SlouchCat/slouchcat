import type { HomeRow, ItemRef } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
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
  Notice,
  PosterCard,
  Row,
  Skeleton,
  SpatialPage,
  Text,
  VisuallyHidden,
  type HeroSlide,
} from '@/ui';
import { ItemMeta } from './ItemMeta';
import { homeRowTitleKey as rowTitleKey, isAudioKind } from './labels';
import { homeQuery } from './queries';
import styles from './HomePage.module.css';

const authed = getRouteApi('/_authed/_active');

// The trailer dialog needs react-aria, which the start page does not load up front (P-10).
const TrailerPanel = lazy(() =>
  import('./TrailerPanel').then((module) => ({ default: module.TrailerPanel })),
);

// Rows with a page of all their titles ("Alle anzeigen ›", concept 08 O-3: only these two).
const rowPages: Partial<Record<HomeRow['key'], string>> = {
  picks: paths.picks,
  watchlist: paths.watchlist,
};

// Posters loaded eagerly: the cards of the first row that fit a wide screen (P-10); the rest are lazy.
// The server paints as many ahead on a page without a stage (posterPrepaintCards, api/preload.go).
const EAGER_POSTERS = 6;

/** A row counts when it holds titles, collections or genres (E-9: never an empty row). */
const hasEntries = (row: HomeRow) =>
  row.items.length > 0 || (row.collections?.length ?? 0) > 0 || (row.genres?.length ?? 0) > 0;

/** Focus key of the first card of a row (the default focus when the stage does not lead). */
function firstKey(row: HomeRow): string | undefined {
  const item = row.items[0];
  if (item) return cardKey(row, item);
  const collection = row.collections?.[0];
  if (collection) return `${row.key}-${collection.id}`;
  const genre = row.genres?.[0];
  return genre ? `${row.key}-${genre.name}` : undefined;
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
 * Focus and React key of a card. A series card of "Weiterschauen" keys on its series, not on the
 * episode: after the episode ends the card shows the next one and keeps its focus on return.
 */
const cardKey = (row: HomeRow, item: ItemRef) => `${row.key}-${seriesOf(row, item)?.id ?? item.id}`;

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
  // A curated title carries the main user's note (picks, E-2); it says more than the year.
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
  const { t } = useTranslation();
  useDocumentTitle(t('home.title'));
  const { session } = authed.useRouteContext();
  const home = useQuery(homeQuery);
  // Posters wait until the hero artwork is on screen: on a slow link they would share its
  // bandwidth (P-10). The frames keep their size, so nothing shifts.
  const [heroShown, setHeroShown] = useState(false);
  // The stage slide whose trailer dialog is open (W-10).
  const [trailerSlide, setTrailerSlide] = useState<HeroSlide>();
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
  const leadCard = home.data?.heroRow ? undefined : leadRow?.items[0];
  // A series card shows the series poster, which the server paints ahead as well (cardPoster).
  const leadImages = leadRow && leadCard && (seriesOf(leadRow, leadCard) ?? leadCard).images;
  const leadCardHasArt = Boolean(leadImages && (leadImages.poster ?? leadImages.posterSmall));
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

  const posterRow = (row: HomeRow) => {
    const title = t(rowTitleKey[row.key]);
    const focusKey = `row-${row.key}`;
    const eagerRow = row === firstPosterRow;
    if (row.key === 'genres') {
      return (
        <Row key={row.key} title={title} focusKey={focusKey} variant="chips">
          {(row.genres ?? []).map((genre) => (
            <GenreChip
              key={genre.name}
              focusKey={`${row.key}-${genre.name}`}
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
        <Row key={row.key} title={title} focusKey={focusKey}>
          {(row.collections ?? []).map((collection, itemIndex) => (
            <CollectionCard
              key={collection.id}
              focusKey={`${row.key}-${collection.id}`}
              href={paths.collection(collection.id)}
              name={collection.name}
              caption={t('collection.count', { count: collection.count })}
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
    const moreHref = rowPages[row.key];
    return (
      <Row
        key={row.key}
        title={title}
        focusKey={focusKey}
        moreHref={moreHref}
        moreLabel={moreHref ? t('home.rows.all') : undefined}
      >
        {row.items.map((item, itemIndex) => {
          const series = seriesOf(row, item);
          const text = series
            ? seriesCardText(t, series, item)
            : { title: item.title, caption: cardCaption(t, row, item), label: undefined };
          const images = (series ?? item).images;
          const progress = row.key === 'continue' ? startedProgress(item) : undefined;
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
                    key={row.key}
                    label={t(rowTitleKey[row.key])}
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
                    paused={trailerSlide !== undefined}
                    onArtShown={() => setHeroShown(true)}
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
