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
import { intlLocale } from '@/i18n';
import { formatDate, formatRuntime } from '@/lib/format';
import {
  Button,
  EpisodeListItem,
  Icon,
  Notice,
  PosterCard,
  Skeleton,
  SpatialButton,
  SpatialGroup,
  setSpatialFocus,
  type IconName,
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
// The request dialog needs react-aria, which the browsing pages do not load up front (P-10).
const RequestDialog = lazy(() =>
  import('@/features/wishes/RequestDialog').then((module) => ({ default: module.RequestDialog })),
);

type NextUp = NonNullable<ItemDetail['nextUp']>;
type Catalog = NonNullable<ItemDetail['catalog']>;
type CatalogSeason = Catalog['seasons'][number];
type MissingState = Exclude<CatalogSeason['state'], 'present'>;

/** The series a wall belongs to, as far as its missing seasons need it (concept 25 M-06). */
export type ShelfSeries = Pick<
  ItemDetail,
  'id' | 'title' | 'year' | 'tmdbId' | 'catalog' | 'canRequestSeasons'
>;

/** A cover of the wall: a season of the library, or one the catalog lists as not playable. */
export type WallEntry =
  | { id: string; season: ItemRef; missing?: undefined }
  | { id: string; season?: undefined; missing: CatalogSeason & { state: MissingState } };

const seasonKey = (season: ItemRef) => `season-${season.id}`;
const missingKey = (number: number) => `season-missing-${number}`;
/** «Staffeln anfordern» under the wall. */
const REQUEST_KEY = 'detail-seasons-request';

/** The mark on the cover of a season the profile cannot play (form, not paleness alone). */
const MISSING_ICON = {
  missing: 'plus',
  requested: 'bell',
  processing: 'download',
  announced: 'info',
} as const satisfies Record<MissingState, IconName>;

const isMissing = (season: CatalogSeason): season is CatalogSeason & { state: MissingState } =>
  season.state !== 'present';

/**
 * The covers of the wall: the seasons of the library and, with a catalog (concept 25 M-06, only
 * for a profile that may request series), the seasons it lists as missing, requested, in
 * procurement or announced, in the order of their numbers. Seasons without a number (specials)
 * stay where the server put them, after the numbered ones.
 */
export function wallEntries(seasons: readonly ItemRef[], catalog?: Catalog): WallEntry[] {
  const library: WallEntry[] = seasons.map((season) => ({ id: season.id, season }));
  if (!catalog) return library;
  const numbered = (entry: WallEntry) =>
    entry.missing ? entry.missing.number : (entry.season.seasonNumber ?? 0);
  const present = new Set(seasons.map((season) => season.seasonNumber));
  const missing: WallEntry[] = catalog.seasons
    .filter(isMissing)
    .filter((season) => !present.has(season.number))
    .map((season) => ({ id: missingKey(season.number), missing: season }));
  const regular = [...library.filter((entry) => numbered(entry) > 0), ...missing];
  // A stable sort: two library seasons of one number keep their order.
  regular.sort((a, b) => numbered(a) - numbered(b));
  return [...regular, ...library.filter((entry) => numbered(entry) <= 0)];
}

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
export function foldedSeasons<T extends { id: string }>(
  seasons: readonly T[],
  chosenId: string,
  columns: number,
  unfolded: boolean,
): readonly T[] {
  const limit = columns * 2;
  if (unfolded || limit === 0 || seasons.length <= limit) return seasons;
  const index = seasons.findIndex((season) => season.id === chosenId);
  const first = seasons.slice(0, limit);
  if (index < limit) return first;
  return [...first.slice(0, limit - 1), seasons[index]!];
}

/** «2 von 4 Staffeln vorhanden · Läuft noch, nächste Folge am 4. Dez. 2026» over the wall. */
function catalogLine(t: TFunction, language: string, catalog: Catalog): string {
  const parts = [t('item.seasonWall.present', { present: catalog.present, count: catalog.total })];
  const next = catalog.nextEpisode?.airDate;
  if (next) parts.push(t('item.seasonWall.next', { date: formatDate(next, language) }));
  else if (catalog.inProduction) parts.push(t('item.seasonWall.running'));
  return parts.join(t('format.separator'));
}

/**
 * The cover of a season the profile cannot play (concept 25 M-06): no picture, a dashed outline
 * and a mark with the state «Fehlt», «Angefordert», «In Beschaffung» or «Angekündigt», so it reads
 * apart from the dimmed covers of the seasons that are there. A missing or requested season opens
 * the request dialog with it chosen, where the server allows the profile to request seasons.
 */
function MissingSeasonCard({
  season,
  onRequest,
}: {
  season: CatalogSeason & { state: MissingState };
  onRequest?: (number: number) => void;
}) {
  const { t } = useTranslation();
  const title = t('item.seasonWall.season', { number: season.number });
  const state = t(`item.seasonWall.state.${season.state}`);
  const caption = [
    season.airDate ? season.airDate.slice(0, 4) : undefined,
    season.episodeCount > 0 ? t('item.season.episodes', { count: season.episodeCount }) : undefined,
  ].filter((part): part is string => part !== undefined);
  const requestable = onRequest && (season.state === 'missing' || season.state === 'requested');
  let label = t('item.seasonWall.cardLabel', { title, state });
  if (requestable) {
    label = t(
      season.state === 'missing' ? 'item.seasonWall.cardRequest' : 'item.seasonWall.cardJoin',
      { title },
    );
  }
  return (
    <PosterCard
      focusKey={missingKey(season.number)}
      title={title}
      caption={caption.join(t('format.separator'))}
      label={label}
      dimmed
      tabIndex={-1}
      onPress={requestable ? () => onRequest(season.number) : undefined}
    >
      <span className={styles.missingSeason} data-state={season.state} aria-hidden="true">
        <Icon name={MISSING_ICON[season.state]} size="lg" />
        <span className={styles.missingState}>{state}</span>
      </span>
    </PosterCard>
  );
}

/**
 * The seasons of a series as a wall of covers over the whole width (concept 21 P-6, variant A;
 * before a shelf, concept 17 D-1): each with "2004 · 24 Folgen", a bar of the watched episodes and
 * the check mark of the season; the pen shows with the pointer or the focus. First two rows, then
 * «Alle N Staffeln anzeigen»; the chosen season (first the one of the `nextUp` episode) always
 * stands in the wall and shows its text and episodes underneath. ↓ from the wall leads to the
 * episodes (O-11), over the buttons when there are some.
 *
 * With the series' catalog (concept 25 M-06, only for a profile that may request series) the wall
 * also shows the seasons the profile cannot play, a line «n von m Staffeln vorhanden» and, where
 * the server allows it (`canRequestSeasons`), «Staffeln anfordern»; a press on a missing season
 * opens the same dialog with that season chosen. Without a catalog the wall is as before.
 */
export function SeasonShelf({
  seasons,
  nextUp,
  series,
}: {
  seasons: readonly ItemRef[];
  nextUp?: NextUp;
  series?: ShelfSeries;
}) {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const queryClient = useQueryClient();
  // The pen on a season's cover (concept 20 P-3), as on every other cover.
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  const [editing, setEditing] = useState<ItemRef | null>(null);
  const [chosen, setChosen] = useState<string | null>(null);
  const [unfolded, setUnfolded] = useState(false);
  const [columns, setColumns] = useState(0);
  // The request dialog: the seasons chosen at first (none: every missing one), its result.
  const [requesting, setRequesting] = useState<{ preselect?: readonly number[] } | null>(null);
  const [requested, setRequested] = useState<'requested' | 'joined' | null>(null);
  const wall = useRef<HTMLDivElement>(null);
  const opener = useRef<string>(REQUEST_KEY);
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
  const catalog = series?.catalog;
  const tmdbId = series?.tmdbId;
  // Only with the server's word and a TMDB id to request by (rule 12).
  const canRequest = Boolean(series?.canRequestSeasons && catalog && tmdbId);
  const entries = wallEntries(seasons, catalog);
  const shown = foldedSeasons(entries, current.id, columns, unfolded);
  const foldable = columns > 0 && entries.length > columns * 2;
  const announced = (catalog?.seasons ?? [])
    .filter((season) => season.state === 'announced')
    .map((season) => season.number);
  const request = (preselect?: readonly number[]) => {
    opener.current = preselect ? missingKey(preselect[0]!) : REQUEST_KEY;
    setRequested(null);
    setRequesting({ preselect });
  };
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
      {catalog && (
        <p className={`${styles.muted} ${styles.gutter}`}>
          {catalogLine(t, intlLocale(i18n.language), catalog)}
        </p>
      )}
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
          {shown.map((entry) => {
            if (entry.missing) {
              return (
                <li key={entry.id}>
                  <MissingSeasonCard
                    season={entry.missing}
                    onRequest={canRequest ? (number) => request([number]) : undefined}
                  />
                </li>
              );
            }
            const season = entry.season;
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
      {(foldable || canRequest) && (
        // A group as wide as the page: by geometry ↑ from the episodes reaches it, not the wall.
        <SpatialGroup className={`${styles.wallToggle} ${styles.gutter}`}>
          {foldable && (
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
                : t('item.seasonWall.showAll', { total: entries.length })}
            </SpatialButton>
          )}
          {canRequest && (
            <SpatialButton
              focusKey={REQUEST_KEY}
              variant="secondary"
              icon="plus"
              onPress={() => request()}
            >
              {t('item.seasonWall.request')}
            </SpatialButton>
          )}
        </SpatialGroup>
      )}
      {requested && (
        <div className={styles.gutter}>
          <Notice
            title={t(requested === 'joined' ? 'titles.done.joined' : 'titles.done.requested')}
          />
        </div>
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
      {requesting && series && tmdbId && (
        <Suspense fallback={null}>
          <RequestDialog
            title={{ media: 'show', tmdbId, title: series.title, year: series.year }}
            source="detail"
            preselect={requesting.preselect}
            skip={announced}
            onClose={() => setRequesting(null)}
            onDone={(made) => {
              setRequesting(null);
              setRequested(made.role === 'joined' ? 'joined' : 'requested');
              // The covers show the new states; the focus stays where the dialog came from.
              void queryClient.invalidateQueries({ queryKey: ['item', series.id] });
              setSpatialFocus(opener.current);
            }}
            onAvailable={() => {
              setRequesting(null);
              void queryClient.invalidateQueries({ queryKey: ['item', series.id] });
            }}
          />
        </Suspense>
      )}
    </SpatialGroup>
  );
}
