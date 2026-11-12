import type { ItemDetail, ItemRef } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { offlineSupported } from '@/features/offline/support';
import { formatRuntime } from '@/lib/format';
import { Button, EpisodeListItem, Notice, PosterCard, RowList, Skeleton, SpatialGroup } from '@/ui';
import { CoverPen, EditMetadata } from './EditMetadata';
import { episodeCode } from './labels';
import { itemQuery } from './queries';
import { usePlayStart } from './usePlayStart';
import styles from './ItemPage.module.css';

// The season's "Herunterladen" reads the offline database: a chunk of its own (M5).
const DownloadAction = lazy(() =>
  import('@/features/offline/DownloadAction').then((module) => ({
    default: module.DownloadAction,
  })),
);

type NextUp = NonNullable<ItemDetail['nextUp']>;

const seasonKey = (season: ItemRef) => `season-${season.id}`;

/** "2004 · 24 Folgen · 3 gesehen": the facts of a season (E-13, E-15). */
export function seasonFacts(t: TFunction, season: ItemRef): string[] {
  const total = season.childCount ?? 0;
  const watched = season.watchedCount ?? 0;
  return [
    season.year !== undefined ? String(season.year) : undefined,
    season.childCount !== undefined ? t('item.season.episodes', { count: total }) : undefined,
    watched > 0 ? t('item.season.watched', { count: watched }) : undefined,
  ].filter((part): part is string => part !== undefined);
}

/**
 * The episodes of a season as a list (E-14): the `nextUp` episode marked with "Als Nächstes" or
 * "Weiterschauen" when it is in this list. A click on an episode starts it, a started one asks
 * first whether to resume (concept 20 P-4); «Details» opens its page.
 */
export function EpisodeList({
  episodes,
  nextUp,
}: {
  episodes: readonly ItemRef[];
  nextUp?: NextUp;
}) {
  const { t } = useTranslation();
  const playStart = usePlayStart();
  if (episodes.length === 0) return <p className={styles.muted}>{t('item.episodes.empty')}</p>;
  const marked = nextUp && nextUp.reason !== 'rewatch' ? nextUp.episode.id : undefined;
  return (
    <>
      <SpatialGroup
        as="ul"
        className={styles.episodeRows}
        aria-label={t('item.episodes.title')}
        preferredChildFocusKey={marked ? `episode-${marked}` : undefined}
      >
        {episodes.map((episode) => {
          const progress = episode.progress;
          const code = episodeCode(t, episode);
          const named = code
            ? t('item.nextUp.episode', { code, title: episode.title })
            : episode.title;
          return (
            <li key={episode.id}>
              <EpisodeListItem
                focusKey={`episode-${episode.id}`}
                playHref={paths.play(episode.id)}
                playLabel={t('item.episodes.play')}
                onPlay={(event) => playStart.ask({ ...episode, title: named }, event)}
                detailsHref={paths.item(episode.id)}
                detailsLabel={t('item.details')}
                detailsName={t('item.episodes.details', { title: named })}
                code={code}
                title={episode.title}
                overview={episode.overview}
                caption={
                  episode.runtimeMinutes ? formatRuntime(t, episode.runtimeMinutes) : undefined
                }
                stillUrl={imageUrl(episode.images.backdropSmall ?? episode.images.backdrop)}
                accent={episode.accent}
                progress={
                  progress && !progress.completed && progress.positionMs > 0
                    ? {
                        percent: progress.percent,
                        label: t('item.watched', { percent: Math.round(progress.percent) }),
                      }
                    : undefined
                }
                watchedLabel={progress?.completed ? t('library.status.watched') : undefined}
                highlight={
                  episode.id === marked
                    ? t(
                        nextUp?.reason === 'resume'
                          ? 'item.nextUp.markResume'
                          : 'item.nextUp.markNext',
                      )
                    : undefined
                }
              />
            </li>
          );
        })}
      </SpatialGroup>
      {playStart.dialog}
    </>
  );
}

/** The chosen season under the shelf: its facts, its text and its episodes (season detail). */
function ChosenSeason({ season, nextUp }: { season: ItemRef; nextUp?: NextUp }) {
  const { t } = useTranslation();
  const detail = useQuery(itemQuery(season.id));
  if (detail.isPending) {
    return (
      <div className={styles.episodeRows} aria-busy="true">
        <Skeleton shape="block" />
      </div>
    );
  }
  if (detail.isError) {
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, detail.error)}
        actions={
          <Button icon="refresh" onPress={() => void detail.refetch()}>
            {t('common.retry')}
          </Button>
        }
      />
    );
  }
  const facts = seasonFacts(t, detail.data);
  return (
    <>
      <div className={styles.seasonHead}>
        <h3 className={styles.seasonTitle}>{detail.data.title}</h3>
        {facts.length > 0 && <p className={styles.muted}>{facts.join(t('format.separator'))}</p>}
        {detail.data.overview && <p className={styles.seasonText}>{detail.data.overview}</p>}
      </div>
      <EpisodeList
        episodes={detail.data.episodes ?? []}
        // Marked only in the season the episode belongs to (m-3).
        nextUp={nextUp?.episode.parentId === season.id ? nextUp : undefined}
      />
    </>
  );
}

/**
 * Scrolls the row sideways so the chosen card stands wholly in view, without moving the page (the
 * shelf may still be below the fold) and without taking the focus.
 */
function revealChosen(scroller: HTMLDivElement | null) {
  const card = scroller?.querySelector('[aria-current="true"]');
  if (!scroller || !card) return;
  const inset = parseFloat(getComputedStyle(scroller).scrollPaddingInlineStart) || 0;
  const box = scroller.getBoundingClientRect();
  const place = card.getBoundingClientRect();
  if (place.left < box.left + inset) scroller.scrollLeft -= box.left + inset - place.left;
  else if (place.right > box.right - inset)
    scroller.scrollLeft += place.right - (box.right - inset);
}

/**
 * The seasons of a series as a shelf of covers (concept 17 D-1, variant A; E-13): each with
 * "2004 · 24 Folgen", a bar of the watched episodes and a check when all are watched. The chosen
 * season (first the one of the `nextUp` episode) shows its text and episodes underneath. Fixed
 * arrows page the shelf, a mouse drags it, and the chosen season moves into view by itself when
 * the page opens (the season of the next episode may be the ninth) and when it changes (concept 20
 * P-1).
 */
export function SeasonShelf({ seasons, nextUp }: { seasons: readonly ItemRef[]; nextUp?: NextUp }) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const queryClient = useQueryClient();
  // The pen on a season's cover (concept 20 P-3), as on every other cover.
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  const [editing, setEditing] = useState<ItemRef | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const current =
    seasons.find((season) => season.id === chosen) ??
    seasons.find((season) => season.id === nextUp?.episode.parentId) ??
    seasons[0];
  const currentId = current?.id;
  useLayoutEffect(() => revealChosen(scroller.current), [currentId]);
  if (!current) return null;
  return (
    // One group for shelf and episodes, as wide as the page: ↓ from the actions in the head enters
    // it at the shelf (by geometry the wider episode list would win over the narrow shelf).
    <SpatialGroup
      as="section"
      className={styles.shelf}
      aria-labelledby="item-seasons-heading"
      preferredChildFocusKey="detail-seasons"
      rememberLastChild={false}
    >
      <h2 id="item-seasons-heading" className={`${styles.sectionTitle} ${styles.gutter}`}>
        {t('item.seasons')}
      </h2>
      <RowList
        focusKey="detail-seasons"
        aria-label={t('item.seasons')}
        arrows="always"
        scrollerRef={scroller}
        preferredChildFocusKey={seasonKey(current)}
      >
        {seasons.map((season) => {
          const active = season.id === current.id;
          const total = season.childCount ?? 0;
          const watched = Math.min(season.watchedCount ?? 0, total);
          return (
            <PosterCard
              key={season.id}
              focusKey={seasonKey(season)}
              title={season.title}
              caption={seasonFacts(t, season).join(t('format.separator'))}
              posterUrl={imageUrl(season.images.poster)}
              posterSmallUrl={imageUrl(season.images.posterSmall)}
              accent={season.accent}
              onPress={() => setChosen(season.id)}
              active={active}
              dimmed={!active}
              aria-current={active ? 'true' : undefined}
              tabIndex={active ? 0 : -1}
              watchedLabel={
                total > 0 && watched === total ? t('library.status.watched') : undefined
              }
              // Only the chosen season is a Tab stop, then its pen (roving tabindex).
              tool={
                canManage ? (
                  <CoverPen
                    item={season}
                    onEdit={() => setEditing(season)}
                    excludeFromTabOrder={!active}
                  />
                ) : undefined
              }
              progress={
                watched > 0 && watched < total
                  ? {
                      percent: (watched / total) * 100,
                      label: t('item.season.progress', { count: watched, total }),
                    }
                  : undefined
              }
            />
          );
        })}
      </RowList>
      <div className={`${styles.season} ${styles.gutter}`}>
        <ChosenSeason key={current.id} season={current} nextUp={nextUp} />
        {/* Below the episodes: the arrows from the shelf still land on the episodes. */}
        {offlineSupported() && (
          <Suspense fallback={null}>
            <SpatialGroup className={styles.seasonActions}>
              <DownloadAction
                key={current.id}
                target={current}
                focusKey="detail-season-download"
                size="md"
              />
            </SpatialGroup>
          </Suspense>
        )}
      </div>
      {editing && (
        <EditMetadata
          itemId={editing.id}
          onClose={() => {
            setEditing(null);
            // The shelf's covers come with the series.
            if (editing.parentId) {
              void queryClient.invalidateQueries({ queryKey: ['item', editing.parentId] });
            }
          }}
        />
      )}
    </SpatialGroup>
  );
}
