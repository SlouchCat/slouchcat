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
  dismissHeroPrepaint,
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
  continue: 'home.rows.continue',
  recent: 'home.rows.recent',
  picks: 'home.picks',
} as const;

// Posters loaded eagerly: the cards of the first row that fit a wide screen (P-10); the rest are lazy.
const EAGER_POSTERS = 6;

const cardKey = (row: HomeRow, item: ItemRef) => `${row.key}-${item.id}`;

function cardCaption(t: TFunction, row: HomeRow, item: ItemRef): string | undefined {
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

/** Start page (E-1): Cat's picks as hero carousel, then the rows the server returned. */
export function HomePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('home.title'));
  const { session } = authed.useRouteContext();
  const home = useQuery(homeQuery);
  // Posters wait until the hero artwork is on screen: on a slow link they would share its
  // bandwidth (P-10). The frames keep their size, so nothing shifts.
  const [heroShown, setHeroShown] = useState(false);
  const firstPick = home.data?.rows.find((row) => row.key === 'picks')?.items[0];
  const heroHasArt = Boolean(
    firstPick &&
    (firstPick.images.backdrop ?? firstPick.images.backdropSmall ?? firstPick.images.poster),
  );
  // The server painted the first hero image into the shell (P-10); it gives way once the carousel
  // shows its own image, or right away when the page has no hero art.
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
  const rows = home.data.rows.filter((row) => row.items.length > 0);
  const picks = rows.find((row) => row.key === 'picks');
  const lists = rows.filter((row) => row.key !== 'picks');
  const firstList = lists[0];
  const firstCard = firstList?.items[0];
  const defaultFocusKey = picks
    ? HERO_PLAY_KEY
    : firstList && firstCard
      ? cardKey(firstList, firstCard)
      : 'header';

  const slides: HeroSlide[] = (picks?.items ?? []).map((item) => ({
    id: item.id,
    title: item.title,
    backdropUrl: imageUrl(item.images.backdrop),
    backdropSmallUrl: imageUrl(item.images.backdropSmall),
    posterUrl: imageUrl(item.images.poster),
    meta: <ItemMeta item={item} />,
    playHref: paths.play(item.id),
    detailsHref: paths.item(item.id),
  }));

  return (
    <SpatialPage defaultFocusKey={defaultFocusKey}>
      <main className={styles.home}>
        <VisuallyHidden as="h1">{t('home.title')}</VisuallyHidden>
        {picks && (
          <HeroCarousel
            label={t('home.picks')}
            slides={slides}
            playLabel={t('item.play')}
            detailsLabel={t('item.details')}
            onArtShown={() => setHeroShown(true)}
          />
        )}
        {lists.length > 0 && (
          <div className={`${styles.rows} ${picks ? styles.underHero : styles.padded}`}>
            {lists.map((row, rowIndex) => (
              <Row key={row.key} title={t(rowTitleKey[row.key])} focusKey={`row-${row.key}`}>
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
                    eager={rowIndex === 0 && itemIndex < EAGER_POSTERS}
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
            ))}
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
