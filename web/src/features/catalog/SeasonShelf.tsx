import type { ItemDetail, ItemRef } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useId, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { offlineSupported } from '@/features/offline/support';
import { formatRuntime } from '@/lib/format';
import {
  Button,
  EpisodeListItem,
  Notice,
  PosterCard,
  Skeleton,
  SpatialButton,
  SpatialGroup,
} from '@/ui';
import { CoverPen, EditMetadata } from './EditMetadata';
import { episodeCode } from './labels';
import { itemQuery } from './queries';
import { usePlayStart } from './usePlayStart';
import { CoverWatched } from './watched';
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

/** The number at the start of an episode's line ("2", "3–4"), or undefined without one. */
function episodeNumber(t: TFunction, episode: ItemRef): string | undefined {
  if (episode.episodeNumber === undefined) return undefined;
  if (episode.episodeEnd !== undefined && episode.episodeEnd > episode.episodeNumber) {
    return t('item.episodeNumberRange', {
      episode: episode.episodeNumber,
      end: episode.episodeEnd,
    });
  }
  return String(episode.episodeNumber);
}

/**
 * The episodes of a season as a compact list (concept 21 P-6, variant C): one line each, the
 * `nextUp` episode marked with "Als Nächstes" or "Weiterschauen" when it is in this list and open
 * when the page opens. A click on a line opens or closes its row, Enter opens it and moves to
 * «Abspielen» (O-11); one row is open at a time. A started episode asks first whether to resume
 * (concept 20 P-4); «Details» opens its page; «Herunterladen» where the device can.
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
  // The open row: first the nextUp episode, fixed as the list opens (a check set on it moves
  // nextUp, not the open row); null: every row closed.
  const [open, setOpen] = useState<string | null>(
    () => episodes.find((episode) => episode.id === nextUp?.episode.id)?.id ?? null,
  );
  if (episodes.length === 0) return <p className={styles.muted}>{t('item.episodes.empty')}</p>;
  const marked = nextUp && nextUp.reason !== 'rewatch' ? nextUp.episode.id : undefined;
  const offline = offlineSupported();
  return (
    <>
      <SpatialGroup
        as="ul"
        className={styles.episodeRows}
        aria-label={t('item.episodes.title')}
        preferredChildFocusKey={`episode-${open ?? marked ?? episodes[0]?.id}`}
      >
        {episodes.map((episode) => {
          const progress = episode.progress;
          const started = progress && !progress.completed && progress.positionMs > 0;
          const code = episodeCode(t, episode);
          const named = code
            ? t('item.nextUp.episode', { code, title: episode.title })
            : episode.title;
          const expanded = episode.id === open;
          return (
            <li key={episode.id}>
              <EpisodeListItem
                focusKey={`episode-${episode.id}`}
                number={episodeNumber(t, episode)}
                code={code}
                title={episode.title}
                overview={episode.overview}
                caption={
                  episode.runtimeMinutes ? formatRuntime(t, episode.runtimeMinutes) : undefined
                }
                highlight={
                  episode.id === marked
                    ? t(
                        nextUp?.reason === 'resume'
                          ? 'item.nextUp.markResume'
                          : 'item.nextUp.markNext',
                      )
                    : undefined
                }
                expanded={expanded}
                onToggle={(keyboard) => setOpen(expanded && !keyboard ? null : episode.id)}
                onClose={() => setOpen(null)}
                playHref={paths.play(episode.id)}
                playLabel={t(started ? 'item.nextUp.resume' : 'item.episodes.play')}
                playName={t(started ? 'item.episodes.resumeTitle' : 'item.episodes.playTitle', {
                  title: named,
                })}
                onPlay={(event) => playStart.ask({ ...episode, title: named }, event)}
                detailsHref={paths.item(episode.id)}
                detailsLabel={t('item.details')}
                detailsName={t('item.episodes.details', { title: named })}
                actions={
                  offline && expanded ? (
                    <Suspense fallback={null}>
                      <DownloadAction
                        target={episode}
                        focusKey={`episode-${episode.id}-download`}
                        size="md"
                      />
                    </Suspense>
                  ) : undefined
                }
                stillUrl={imageUrl(episode.images.backdropSmall ?? episode.images.backdrop)}
                accent={episode.accent}
                progress={
                  started
                    ? {
                        percent: progress.percent,
                        label: t('item.watched', { percent: Math.round(progress.percent) }),
                      }
                    : undefined
                }
                // Named with the code, as «Details» is ("Gesehen: S01E02 · Die lange Nacht").
                watchedControl={
                  <CoverWatched item={{ ...episode, title: named }} appearance="row" />
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

/** How many tracks the grid has now (auto-fill), or 0 where nothing is laid out (tests). */
function gridColumns(grid: Element | null | undefined): number {
  if (!grid) return 0;
  const tracks = getComputedStyle(grid).gridTemplateColumns.trim();
  return tracks === '' || tracks === 'none' ? 0 : tracks.split(/\s+/).length;
}

/**
 * The seasons a wall of `columns` columns shows folded: its first two rows, with the chosen season
 * in place of the last one when it would lie further down (concept 21 P-6). All of them unfolded,
 * or while the columns are not known.
 */
export function foldedSeasons(
  seasons: readonly ItemRef[],
  chosenId: string,
  columns: number,
  unfolded: boolean,
): readonly ItemRef[] {
  const limit = columns * 2;
  if (unfolded || limit === 0 || seasons.length <= limit) return seasons;
  const index = seasons.findIndex((season) => season.id === chosenId);
  const first = seasons.slice(0, limit);
  if (index < limit) return first;
  return [...first.slice(0, limit - 1), seasons[index]!];
}

/**
 * The seasons of a series as a wall of covers over the whole width (concept 21 P-6, variant A;
 * before a shelf, concept 17 D-1): each with "2004 · 24 Folgen", a bar of the watched episodes and
 * the check mark of the season; the pen shows with the pointer or the focus. First two rows, then
 * «Alle N Staffeln anzeigen»; the chosen season (first the one of the `nextUp` episode) always
 * stands in the wall and shows its text and episodes underneath. ↓ from the wall leads to the
 * episodes (O-11), over the button when there is one.
 */
export function SeasonShelf({ seasons, nextUp }: { seasons: readonly ItemRef[]; nextUp?: NextUp }) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const queryClient = useQueryClient();
  // The pen on a season's cover (concept 20 P-3), as on every other cover.
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  const [editing, setEditing] = useState<ItemRef | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);
  const [unfolded, setUnfolded] = useState(false);
  const [columns, setColumns] = useState(0);
  const wall = useRef<HTMLDivElement>(null);
  const wallId = useId();
  const hasSeasons = seasons.length > 0;
  // The columns follow the width of the page (auto-fill); measured before the first paint.
  useLayoutEffect(() => {
    const node = wall.current;
    if (!node) return;
    const update = () => setColumns(gridColumns(node.firstElementChild));
    update();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(node);
    return () => observer.disconnect();
    // Again once the seasons are there: without them the wall is not in the page.
  }, [hasSeasons]);
  const current =
    seasons.find((season) => season.id === chosen) ??
    seasons.find((season) => season.id === nextUp?.episode.parentId) ??
    seasons[0];
  if (!current) return null;
  const shown = foldedSeasons(seasons, current.id, columns, unfolded);
  const foldable = columns > 0 && seasons.length > columns * 2;
  return (
    // One group for the wall and the episodes, as wide as the page: ↓ from the actions in the head
    // enters it at the wall (by geometry the episode list could win over the wall).
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
      <div ref={wall} className={styles.gutter}>
        <SpatialGroup
          focusKey="detail-seasons"
          as="ul"
          id={wallId}
          className={styles.seasonWall}
          aria-label={t('item.seasons')}
          preferredChildFocusKey={seasonKey(current)}
          rememberLastChild={false}
        >
          {shown.map((season) => {
            const active = season.id === current.id;
            const total = season.childCount ?? 0;
            const watched = Math.min(season.watchedCount ?? 0, total);
            return (
              <li key={season.id}>
                <PosterCard
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
                  // Only the chosen season is a Tab stop, then its pen and its check (roving
                  // tabindex). The check sets or clears every episode of the season (concept 20
                  // P-5).
                  check={<CoverWatched item={season} excludeFromTabOrder={!active} />}
                  tool={
                    canManage ? (
                      <CoverPen
                        item={season}
                        onEdit={() => setEditing(season)}
                        excludeFromTabOrder={!active}
                      />
                    ) : undefined
                  }
                  toolOnHover
                  progress={
                    watched > 0 && watched < total
                      ? {
                          percent: (watched / total) * 100,
                          label: t('item.season.progress', { count: watched, total }),
                        }
                      : undefined
                  }
                />
              </li>
            );
          })}
        </SpatialGroup>
      </div>
      {foldable && (
        // A group as wide as the page: by geometry ↑ from the episodes reaches it, not the wall.
        <SpatialGroup className={`${styles.wallToggle} ${styles.gutter}`}>
          <SpatialButton
            focusKey="detail-seasons-fold"
            variant="secondary"
            icon={unfolded ? 'chevronUp' : 'chevronDown'}
            aria-expanded={unfolded}
            aria-controls={wallId}
            onPress={() => setUnfolded((value) => !value)}
          >
            {unfolded
              ? t('item.seasonWall.showLess')
              : t('item.seasonWall.showAll', { total: seasons.length })}
          </SpatialButton>
        </SpatialGroup>
      )}
      <div className={`${styles.season} ${styles.gutter}`}>
        <ChosenSeason key={current.id} season={current} nextUp={nextUp} />
        {/* Below the episodes: ↓ from the wall still lands on the episodes. */}
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
            // The wall's covers come with the series.
            if (editing.parentId) {
              void queryClient.invalidateQueries({ queryKey: ['item', editing.parentId] });
            }
          }}
        />
      )}
    </SpatialGroup>
  );
}
