import type { HomeRow, ItemRef } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useEffect, useState } from 'react';
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
import { isAudioKind } from './labels';
import { homeQuery } from './queries';
import styles from './HomePage.module.css';

const authed = getRouteApi('/_authed/_active');

const rowTitleKey = {
  spotlight: 'home.rows.spotlight',
  continue: 'home.rows.continue',
  recent: 'home.rows.recent',
  released: 'home.rows.released',
  picks: 'home.picks',
  watchlist: 'home.rows.watchlist',
  collections: 'home.rows.collections',
  genres: 'home.rows.genres',
} as const;

// Posters loaded eagerly: the cards of the first row that fit a wide screen (P-10); the rest are lazy.
const EAGER_POSTERS = 6;

const cardKey = (row: HomeRow, item: ItemRef) => `${row.key}-${item.id}`;

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

function cardCaption(t: TFunction, row: HomeRow, item: ItemRef): string | undefined {
  // A curated title carries the main user's note (picks, E-2); it says more than the year.
  if (item.note) return item.note;
  if (row.key === 'continue' && item.progress) {
    const minutes = Math.max(
      1,
      Math.round((item.progress.durationMs - item.progress.positionMs) / 60_000),
    );
    return t('home.remaining', { time: formatRuntime(t, minutes) });
  }
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
  // Only a stage at the top is the page's first image: the server paints it into the shell, and the
  // posters wait for it. A stage further down has no prepaint and holds no poster back.
  const leadRow = home.data?.rows.find(hasEntries);
  const firstHero = leadRow && leadRow.key === home.data?.heroRow ? leadRow.items[0] : undefined;
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

  const slides = (row: HomeRow): HeroSlide[] =>
    row.items.map((item) => ({
      id: item.id,
      title: item.title,
      backdropUrl: imageUrl(item.images.backdrop),
      backdropSmallUrl: imageUrl(item.images.backdropSmall),
      posterUrl: imageUrl(item.images.poster),
      posterSmallUrl: imageUrl(item.images.posterSmall),
      meta: <ItemMeta item={item} />,
      note: item.note,
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
    return (
      <Row key={row.key} title={title} focusKey={focusKey}>
        {row.items.map((item, itemIndex) => (
          <PosterCard
            key={item.id}
            focusKey={cardKey(row, item)}
            href={paths.item(item.id)}
            title={item.title}
            caption={cardCaption(t, row, item)}
            posterUrl={imageUrl(item.images.poster)}
            posterSmallUrl={imageUrl(item.images.posterSmall)}
            accent={item.accent}
            eager={eagerRow && itemIndex < EAGER_POSTERS}
            deferImage={heroHasArt && !heroShown}
            shape={isAudioKind(item.kind) ? 'square' : 'poster'}
            progress={
              row.key === 'continue' && item.progress
                ? {
                    percent: item.progress.percent,
                    label: t(isAudioKind(item.kind) ? 'item.listened' : 'item.watched', {
                      percent: Math.round(item.progress.percent),
                    }),
                  }
                : undefined
            }
          />
        ))}
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
            {session.account.role === 'owner' && (
              <div>
                <ButtonLink href={paths.adminLibraries} variant="secondary" icon="folder">
                  {t('home.empty.action')}
                </ButtonLink>
              </div>
            )}
          </div>
        )}
      </main>
    </SpatialPage>
  );
}
