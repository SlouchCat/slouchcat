import type {
  Chapter,
  CollectionRef,
  ItemDetail,
  ItemRef,
  MediaFileInfo,
} from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl, imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useFavoriteMutation, useWatchlistMutation } from '@/features/discover/queries';
import { isDownloadable, offlineSupported } from '@/features/offline/support';
import {
  formatBytes,
  formatClock,
  formatDate,
  formatDuration,
  formatPlayerTime,
  formatResolution,
  formatRuntime,
} from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import {
  Backdrop,
  Button,
  ButtonLink,
  CastCard,
  Chip,
  ChoiceChips,
  CollectionCard,
  EpisodeCard,
  EpisodeRow,
  FactList,
  Notice,
  PosterCard,
  PosterFallback,
  ProgressBar,
  Row,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  itemAccentStyle,
  setSpatialFocus,
  spatialFocusKey,
} from '@/ui';
import { ItemMeta } from './ItemMeta';
import { creditCaption, episodeCode, extraKindLabel, isAudioKind } from './labels';
import { itemQuery, podcastEpisodesQuery } from './queries';
import styles from './ItemPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/items/$itemId');

// The trailer dialog needs react-aria, which the browsing pages do not load up front (P-10).
const TrailerPanel = lazy(() =>
  import('./TrailerPanel').then((module) => ({ default: module.TrailerPanel })),
);

// "Herunterladen" and the download state (M5) read the offline database: a chunk of their own,
// loaded only where the browser can keep downloads.
const DownloadAction = lazy(() =>
  import('@/features/offline/DownloadAction').then((module) => ({
    default: module.DownloadAction,
  })),
);

const PLAY_KEY = 'detail-play';
const BACK_KEY = 'detail-back';
const TRAILER_KEY = 'detail-trailer';
const DOWNLOAD_KEY = 'detail-download';
const WATCHLIST_KEY = 'detail-watchlist';
const FAVORITE_KEY = 'detail-favorite';
// Rendered poster width (ItemPage.module.css, wide layout only).
const POSTER_SIZES = 'clamp(13rem, 20vw, 18rem)';

/** "36 % gesehen", or "gehört" for audio titles. */
const progressLabel = (t: TFunction, item: ItemRef, percent: number) =>
  t(isAudioKind(item.kind) ? 'item.listened' : 'item.watched', { percent: Math.round(percent) });

const watchedProgress = (t: TFunction, item: ItemRef) =>
  item.progress && !item.progress.completed && item.progress.positionMs > 0
    ? { percent: item.progress.percent, label: progressLabel(t, item, item.progress.percent) }
    : undefined;

/**
 * Whether the play button works: series, seasons and podcasts play through their episodes; a
 * podcast episode streams from its feed through the server; an audiobook plays all its parts;
 * any other title needs the chosen version's file.
 */
function canPlay(item: ItemDetail, file: MediaFileInfo | undefined): boolean {
  switch (item.kind) {
    case 'series':
    case 'season':
    case 'podcast':
      return false;
    case 'podcast_episode':
      return true;
    case 'audiobook':
      return item.mediaFiles.some((entry) => entry.available);
    default:
      return file?.available === true;
  }
}

function FileFacts({ file, t, language }: { file: MediaFileInfo; t: TFunction; language: string }) {
  const facts: [string, string][] = [[t('item.file.container'), file.container.toUpperCase()]];
  if (file.width && file.height) {
    facts.push([t('item.file.resolution'), formatResolution(t, file.width, file.height)]);
  }
  if (file.videoCodec) facts.push([t('item.file.video'), file.videoCodec.toUpperCase()]);
  if (file.audioCodec) facts.push([t('item.file.audio'), file.audioCodec.toUpperCase()]);
  facts.push([t('item.file.size'), formatBytes(file.sizeBytes, language)]);
  if (file.versionLabel) facts.push([t('item.file.version'), file.versionLabel]);

  return (
    <div className={styles.file}>
      {!file.available && (
        <Chip tone="warning" icon="warning">
          {t('item.file.unavailable')}
        </Chip>
      )}
      <FactList facts={facts} />
    </div>
  );
}

interface ActionsProps {
  item: ItemDetail;
  /** The version the play link starts (B-5); undefined for titles without files. */
  file: MediaFileInfo | undefined;
  onBack: () => void;
  onTrailer: () => void;
}

function Actions({ item, file, onBack, onTrailer }: ActionsProps) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const online = useOnline();
  // Series, seasons and podcasts play through their episodes; any other title has a play button,
  // disabled while it cannot play.
  const playable = item.kind !== 'series' && item.kind !== 'season' && item.kind !== 'podcast';
  const available = canPlay(item, file);
  const audio = isAudioKind(item.kind);
  const resume =
    item.progress && !item.progress.completed && item.progress.positionMs > 0
      ? item.progress
      : undefined;
  const playLabel = resume
    ? t(audio ? 'item.resumeListening' : 'item.resume', { time: formatClock(resume.positionMs) })
    : t('item.play');
  // Only one version: the server picks it; with several the chosen one goes along. An audiobook's
  // files are its parts, not versions.
  const version = !audio && item.mediaFiles.length > 1 ? file?.id : undefined;
  const playHref = paths.play(item.id, version);
  // A shortcut for the one who manages the library, not a visibility rule: the server answers the
  // administration page only for that account and profile anyway.
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  // Every kind but seasons and extras goes on the watchlist (E-5); series and podcasts can be
  // favourites, which notify about new episodes (E-8).
  const listable = item.kind !== 'season' && item.kind !== 'extra';
  const favoritable = item.kind === 'series' || item.kind === 'podcast';
  const watchlist = useWatchlistMutation();
  const favorite = useFavoriteMutation();
  const failed = watchlist.error ?? favorite.error;

  return (
    <>
      <SpatialGroup className={styles.actions}>
        {playable &&
          (available ? (
            <SpatialButtonLink
              focusKey={PLAY_KEY}
              href={playHref}
              variant="primary"
              size="lg"
              icon="play"
            >
              {playLabel}
            </SpatialButtonLink>
          ) : (
            <Button variant="primary" size="lg" icon="play" isDisabled>
              {playLabel}
            </Button>
          ))}
        <SpatialButton focusKey={BACK_KEY} variant="ghost" size="lg" icon="back" onPress={onBack}>
          {t('common.back')}
        </SpatialButton>
        {(item.trailerCount ?? 0) > 0 && online && (
          <SpatialButton
            focusKey={TRAILER_KEY}
            variant="secondary"
            size="lg"
            icon="play"
            onPress={onTrailer}
          >
            {t('item.trailer')}
          </SpatialButton>
        )}
        {/* After play, back and trailer, so their order for arrows and Tab stays as it was. */}
        {offlineSupported() && isDownloadable(item.kind) && (
          <Suspense fallback={null}>
            <DownloadAction
              target={item}
              mediaFileId={version}
              available={item.kind === 'season' || available}
              focusKey={DOWNLOAD_KEY}
            />
          </Suspense>
        )}
        {canManage && (
          <SpatialButtonLink href={paths.adminItem(item.id)} variant="ghost" size="lg" icon="info">
            {t('item.manage')}
          </SpatialButtonLink>
        )}
      </SpatialGroup>
      {/* A row of its own under the playback actions: that row keeps its order for arrows and
          Tab, and ↓ from play reaches "Merken" on every kind that has it. */}
      {listable && (
        <SpatialGroup className={styles.listActions}>
          <SpatialButton
            focusKey={WATCHLIST_KEY}
            variant="secondary"
            icon={item.inWatchlist ? 'check' : 'plus'}
            aria-disabled={watchlist.isPending || undefined}
            onPress={() => {
              if (!watchlist.isPending) watchlist.mutate({ id: item.id, on: !item.inWatchlist });
            }}
          >
            {item.inWatchlist ? t('item.watchlist.added') : t('item.watchlist.add')}
          </SpatialButton>
          {favoritable && (
            <SpatialButton
              focusKey={FAVORITE_KEY}
              variant="secondary"
              icon={item.favorite ? 'check' : 'plus'}
              aria-pressed={item.favorite}
              aria-description={t('item.favorite.hint')}
              aria-disabled={favorite.isPending || undefined}
              onPress={() => {
                if (!favorite.isPending) favorite.mutate({ id: item.id, on: !item.favorite });
              }}
            >
              {t('item.favorite.label')}
            </SpatialButton>
          )}
          {canManage && (
            <SpatialButtonLink
              href={`${paths.adminPicks}?add=${encodeURIComponent(item.id)}`}
              variant="ghost"
              icon="plus"
            >
              {t('item.addToPicks')}
            </SpatialButtonLink>
          )}
        </SpatialGroup>
      )}
      {failed && <Notice tone="danger" title={problemMessage(t, failed)} />}
    </>
  );
}

/** The visible collections the title belongs to (E-6). */
function Collections({ collections }: { collections: readonly CollectionRef[] }) {
  const { t } = useTranslation();
  if (collections.length === 0) return null;
  return (
    <Row title={t('item.collections')} focusKey="detail-collections">
      {collections.map((collection) => (
        <CollectionCard
          key={collection.id}
          href={paths.collection(collection.id)}
          name={collection.name}
          caption={t('collection.count', { count: collection.count })}
          posterUrl={imageUrl(collection.cover?.images.poster)}
          posterSmallUrl={imageUrl(collection.cover?.images.posterSmall)}
          accent={collection.cover?.accent}
        />
      ))}
    </Row>
  );
}

/**
 * Links to the neighbours of an episode, season or extra: next episode, series, main title; the
 * podcast of a podcast episode.
 */
function RelatedLinks({ item }: { item: ItemDetail }) {
  const { t } = useTranslation();
  const next = item.kind === 'episode' ? item.nextEpisode : undefined;
  const series = item.kind === 'episode' || item.kind === 'season' ? item.series : undefined;
  const parentId = item.kind === 'extra' ? item.parentId : undefined;
  const podcast = item.kind === 'podcast_episode' ? item.podcast : undefined;
  if (!next && !series && !parentId && !podcast) return null;
  const nextCode = next ? episodeCode(t, next) : undefined;

  return (
    <SpatialGroup className={styles.related}>
      {next && (
        <SpatialButtonLink href={paths.item(next.id)} variant="secondary" icon="chevronRight">
          {t('item.nextEpisode')}
          <span className={styles.relatedDetail}>
            {nextCode
              ? t('item.nextEpisodeLabel', { code: nextCode, title: next.title })
              : next.title}
          </span>
        </SpatialButtonLink>
      )}
      {series && (
        <SpatialButtonLink href={paths.item(series.id)} variant="ghost" icon="folder">
          {t('item.seriesLink')}
        </SpatialButtonLink>
      )}
      {parentId && (
        <SpatialButtonLink href={paths.item(parentId)} variant="ghost" icon="folder">
          {t('item.parentLink')}
        </SpatialButtonLink>
      )}
      {podcast && (
        <SpatialButtonLink href={paths.item(podcast.id)} variant="ghost" icon="folder">
          {t('item.podcast.link')}
        </SpatialButtonLink>
      )}
    </SpatialGroup>
  );
}

function Versions({
  item,
  file,
  onChange,
}: {
  item: ItemDetail;
  file: MediaFileInfo | undefined;
  onChange: (id: string) => void;
}) {
  const { t, i18n } = useTranslation();
  // An audiobook's files are its parts and a podcast episode streams from its feed (M6).
  if (item.kind === 'series' || item.kind === 'season' || isAudioKind(item.kind)) return null;
  return (
    <section className={styles.files} aria-labelledby="item-file-heading">
      <h2 id="item-file-heading" className={styles.sectionTitle}>
        {t('item.file.title')}
      </h2>
      {item.mediaFiles.length > 1 && file && (
        <ChoiceChips
          label={t('item.versions')}
          focusKey="detail-version"
          value={file.id}
          onChange={onChange}
          options={item.mediaFiles.map((entry, index) => ({
            value: entry.id,
            label: entry.versionLabel ?? String(index + 1),
          }))}
        />
      )}
      {file ? (
        <FileFacts file={file} t={t} language={i18n.language} />
      ) : (
        <Notice tone="warning">{t('item.file.none')}</Notice>
      )}
    </section>
  );
}

function EpisodeGrid({ episodes }: { episodes: readonly ItemRef[] }) {
  const { t } = useTranslation();
  if (episodes.length === 0) return <p className={styles.muted}>{t('item.episodes.empty')}</p>;
  return (
    <SpatialGroup as="ul" className={styles.episodes} aria-label={t('item.episodes.title')}>
      {episodes.map((episode) => (
        <li key={episode.id}>
          <EpisodeCard
            focusKey={`episode-${episode.id}`}
            href={paths.item(episode.id)}
            code={episodeCode(t, episode) ?? ''}
            title={episode.title}
            caption={episode.runtimeMinutes ? formatRuntime(t, episode.runtimeMinutes) : undefined}
            stillUrl={imageUrl(episode.images.backdropSmall ?? episode.images.backdrop)}
            accent={episode.accent}
            progress={watchedProgress(t, episode)}
          />
        </li>
      ))}
    </SpatialGroup>
  );
}

/** The episodes of one season, fetched as the season's detail (ItemDetail.episodes). */
function SeasonEpisodes({ seasonId }: { seasonId: string }) {
  const { t } = useTranslation();
  const season = useQuery(itemQuery(seasonId));
  if (season.isPending) {
    return (
      <div className={styles.episodes} aria-busy="true">
        <Skeleton shape="block" />
      </div>
    );
  }
  if (season.isError) {
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, season.error)}
        actions={
          <Button icon="refresh" onPress={() => void season.refetch()}>
            {t('common.retry')}
          </Button>
        }
      />
    );
  }
  return <EpisodeGrid episodes={season.data.episodes ?? []} />;
}

function Seasons({ seasons }: { seasons: readonly ItemRef[] }) {
  const { t } = useTranslation();
  const [chosen, setChosen] = useState<string | null>(null);
  const current = seasons.find((season) => season.id === chosen) ?? seasons[0];
  if (!current) return null;
  return (
    <section className={styles.block} aria-labelledby="item-seasons-heading">
      <h2 id="item-seasons-heading" className={styles.sectionTitle}>
        {t('item.seasons')}
      </h2>
      <ChoiceChips
        label={t('item.seasonChoice')}
        focusKey="detail-season"
        value={current.id}
        onChange={setChosen}
        options={seasons.map((season) => ({ value: season.id, label: season.title }))}
      />
      <SeasonEpisodes seasonId={current.id} />
      {/* Below the episodes: the arrows from the season chips still land on the episodes. */}
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
    </section>
  );
}

/** The chapters of an audiobook (H-1); each one starts the player at its start. */
function AudiobookChapters({
  item,
  chapters,
  playable,
}: {
  item: ItemDetail;
  chapters: readonly Chapter[];
  playable: boolean;
}) {
  const { t } = useTranslation();
  if (chapters.length === 0) return null;
  const duration = item.durationMs ?? chapters.at(-1)?.endMs ?? 0;
  const name = (chapter: Chapter, index: number) =>
    chapter.title ?? t('player.chapters.untitled', { n: index + 1 });
  return (
    <section className={styles.block} aria-labelledby="item-chapters-heading">
      <h2 id="item-chapters-heading" className={styles.sectionTitle}>
        {t('item.audio.chapters')}
      </h2>
      <SpatialGroup as="ul" className={styles.chapters} aria-label={t('item.audio.chapters')}>
        {chapters.map((chapter, index) => {
          const time = formatPlayerTime(chapter.startMs, duration);
          return (
            <li key={`${chapter.startMs}-${index}`} className={styles.chapter}>
              {playable ? (
                <SpatialButtonLink
                  focusKey={`chapter-${index}`}
                  href={paths.playAt(item.id, chapter.startMs)}
                  variant="ghost"
                  icon="play"
                  aria-label={t('item.audio.chapterPlay', { title: name(chapter, index), time })}
                >
                  <span className={styles.chapterTitle}>{name(chapter, index)}</span>
                  <span className={styles.relatedDetail}>{time}</span>
                </SpatialButtonLink>
              ) : (
                <span className={styles.chapterText}>
                  <span className={styles.chapterTitle}>{name(chapter, index)}</span>
                  <span className={styles.relatedDetail}>{time}</span>
                </span>
              )}
            </li>
          );
        })}
      </SpatialGroup>
    </section>
  );
}

/**
 * The episodes of a podcast, newest first (H-2): the detail carries the newest 50, "Mehr laden"
 * pages through the rest (listPodcastEpisodes). Every episode plays on its own; the podcast has
 * no play button.
 */
function PodcastEpisodes({ item }: { item: ItemDetail }) {
  const { t, i18n } = useTranslation();
  const first = item.episodes ?? [];
  const total = item.episodeCount ?? first.length;
  const [more, setMore] = useState(false);
  const pages = useInfiniteQuery({
    ...podcastEpisodesQuery(item.id, first.length),
    enabled: more,
  });
  const episodes = useMemo(() => {
    const newest = item.episodes ?? [];
    const loaded = pages.data?.pages.flatMap((page) => page.episodes) ?? [];
    return [...newest, ...loaded.filter((entry) => !newest.some((e) => e.id === entry.id))];
  }, [item.episodes, pages.data]);
  // A failed page leaves the button as a retry; while a page loads it stays focusable.
  const hasMore =
    episodes.length < total && (!more || pages.isFetching || pages.hasNextPage || pages.isError);
  // After a page, focus moves to its first episode (the button may go away with the last one).
  const focusFrom = useRef<number | null>(null);
  useEffect(() => {
    const target = focusFrom.current === null ? undefined : episodes[focusFrom.current];
    if (!target) return;
    focusFrom.current = null;
    setSpatialFocus(`podcast-episode-${target.id}`);
  }, [episodes]);
  const caption = (episode: ItemRef) =>
    [
      episode.publishedAt ? formatDate(episode.publishedAt, i18n.language) : undefined,
      episode.durationMs ? formatDuration(t, episode.durationMs) : undefined,
    ]
      .filter(Boolean)
      .join(t('format.separator')) || undefined;

  return (
    <section className={styles.block} aria-labelledby="item-podcast-episodes-heading">
      <h2 id="item-podcast-episodes-heading" className={styles.sectionTitle}>
        {t('item.podcast.episodes')}
      </h2>
      {episodes.length === 0 ? (
        <p className={styles.muted}>{t('item.podcast.empty')}</p>
      ) : (
        <SpatialGroup
          as="ul"
          className={styles.episodeList}
          aria-label={t('item.podcast.episodes')}
        >
          {episodes.map((episode) => (
            <li key={episode.id}>
              <EpisodeRow
                focusKey={`podcast-episode-${episode.id}`}
                href={paths.item(episode.id)}
                title={episode.title}
                caption={caption(episode)}
                progress={watchedProgress(t, episode)}
                playHref={paths.play(episode.id)}
                playLabel={t('item.podcast.play', { title: episode.title })}
              />
            </li>
          ))}
        </SpatialGroup>
      )}
      {pages.isError && <Notice tone="danger">{problemMessage(t, pages.error)}</Notice>}
      {hasMore && (
        <div>
          <SpatialButton
            focusKey="podcast-more"
            variant="secondary"
            icon="plus"
            aria-disabled={pages.isFetching || undefined}
            onPress={() => {
              if (pages.isFetching) return;
              focusFrom.current = episodes.length;
              if (!more) setMore(true);
              else if (!pages.data) void pages.refetch();
              else void pages.fetchNextPage();
            }}
          >
            {pages.isError ? t('common.retry') : t('item.podcast.more')}
          </SpatialButton>
        </div>
      )}
    </section>
  );
}

function Extras({ extras }: { extras: readonly ItemRef[] }) {
  const { t } = useTranslation();
  if (extras.length === 0) return null;
  return (
    <Row title={t('item.extras')} focusKey="detail-extras">
      {extras.map((extra) => (
        <PosterCard
          key={extra.id}
          href={paths.play(extra.id)}
          title={extra.title}
          badge={extraKindLabel(t, extra.extraKind)}
          caption={extra.runtimeMinutes ? formatRuntime(t, extra.runtimeMinutes) : undefined}
          posterUrl={imageUrl(extra.images.poster)}
          posterSmallUrl={imageUrl(extra.images.posterSmall)}
          accent={extra.accent}
        />
      ))}
    </Row>
  );
}

function Cast({ item }: { item: ItemDetail }) {
  const { t } = useTranslation();
  const cast = item.cast ?? [];
  if (cast.length === 0) return null;
  return (
    <Row title={t('item.cast.title')} focusKey="detail-cast">
      {cast.map((member) => (
        <CastCard
          key={`${member.personId}-${member.role}`}
          href={paths.person(member.personId)}
          name={member.name}
          caption={creditCaption(t, member)}
        />
      ))}
    </Row>
  );
}

/** Tab title per kind: episodes and seasons carry the series name. */
function documentTitle(t: TFunction, item: ItemDetail | undefined): string | undefined {
  if (!item) return undefined;
  const series = item.seriesTitle ?? item.series?.title;
  if (item.kind === 'season' && series) {
    return t('item.docTitle.season', { title: item.title, series });
  }
  if (item.kind === 'episode' && series) {
    return t('item.docTitle.episode', {
      code: episodeCode(t, item) ?? '',
      title: item.title,
      series,
    });
  }
  if (item.kind === 'podcast_episode' && item.podcast) {
    return t('item.docTitle.podcastEpisode', { title: item.title, podcast: item.podcast.title });
  }
  return item.title;
}

/** Line above the title: "S01E02 · Series" for an episode, the series for a season. */
function kicker(t: TFunction, item: ItemDetail): string | undefined {
  const series = item.seriesTitle ?? item.series?.title;
  if (item.kind === 'episode') {
    const code = episodeCode(t, item);
    if (code && series) return t('item.episodeOf', { code, series });
    return code ?? series;
  }
  if (item.kind === 'season') return series;
  if (item.kind === 'extra') return extraKindLabel(t, item.extraKind);
  if (item.kind === 'audiobook' && item.seriesName) {
    return item.seriesIndex !== undefined
      ? t('item.audio.volume', { series: item.seriesName, volume: item.seriesIndex })
      : item.seriesName;
  }
  if (item.kind === 'podcast_episode') return item.podcast?.title;
  return undefined;
}

/** "von <Autor>" and "Gelesen von <Sprecher>" under the title of an audiobook or podcast. */
function bylines(t: TFunction, item: ItemDetail): string[] {
  if (!isAudioKind(item.kind)) return [];
  return [
    item.author ? t('item.audio.by', { author: item.author }) : undefined,
    item.narrator ? t('item.audio.narratedBy', { narrator: item.narrator }) : undefined,
  ].filter((line): line is string => line !== undefined);
}

/**
 * Detail page of a title (C-2): artwork, facts, overview, play or resume. By kind: versions and
 * extras of a film, seasons and episodes of a series, the neighbours of an episode.
 */
export function ItemPage() {
  const { itemId } = route.useParams();
  // One state (chosen version, season, dialog) per title: the next episode is the same route.
  return <ItemView key={itemId} itemId={itemId} />;
}

function ItemView({ itemId }: { itemId: string }) {
  const { t } = useTranslation();
  const query = useQuery(itemQuery(itemId));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  useDocumentTitle(documentTitle(t, query.data));
  const [chosenFile, setChosenFile] = useState<string | null>(null);
  const [trailerOpen, setTrailerOpen] = useState(false);
  const online = useOnline();
  // Offline the trailer button and its dialog go away. The dialog stays closed afterwards (it
  // must not come back by itself and autoplay on reconnect), and the focus moves to "Zurück"
  // instead of getting lost on the page body.
  if (!online && trailerOpen) setTrailerOpen(false);
  useEffect(() => {
    if (!online && spatialFocusKey() === TRAILER_KEY) setSpatialFocus(BACK_KEY);
  }, [online]);

  if (query.isPending) {
    return (
      <main className={styles.detail} aria-busy="true">
        <div className={styles.layout}>
          <div className={styles.poster}>
            <Skeleton shape="poster" />
          </div>
          <div className={styles.info}>
            <Skeleton shape="title" />
            <Skeleton shape="line" />
            <Skeleton shape="block" />
          </div>
        </div>
      </main>
    );
  }

  if (query.isError) {
    const notFound = isApiError(query.error, 404);
    return (
      <main className={`${styles.detail} ${styles.message}`}>
        <Notice
          tone={notFound ? 'info' : 'danger'}
          title={notFound ? t('item.notFound.title') : problemMessage(t, query.error)}
          actions={
            <ButtonLink href={paths.home} variant="primary">
              {t('item.notFound.action')}
            </ButtonLink>
          }
        >
          {notFound && t('item.notFound.body')}
        </Notice>
      </main>
    );
  }

  const item = query.data;
  const { images } = item;
  const episode = item.kind === 'episode';
  const still = images.backdropSmall ?? images.backdrop;
  const hasArt = Boolean(
    images.backdrop ?? images.backdropSmall ?? images.backdropLarge ?? images.poster,
  );
  const progress = item.progress && !item.progress.completed ? item.progress : undefined;
  const genres = item.genres.join(', ');
  // The server lists the primary version first (B-5).
  const file = item.mediaFiles.find((entry) => entry.id === chosenFile) ?? item.mediaFiles[0];
  const line = kicker(t, item);
  const audio = isAudioKind(item.kind);
  const playable = canPlay(item, file);

  return (
    <SpatialPage defaultFocusKey={playable ? PLAY_KEY : BACK_KEY}>
      <main className={styles.detail}>
        <Backdrop
          url={imageUrl(images.backdrop)}
          smallUrl={imageUrl(images.backdropSmall)}
          largeUrl={imageUrl(images.backdropLarge)}
          posterUrl={imageUrl(images.poster)}
          accent={item.accent}
        />

        <div className={`${styles.layout} ${hasArt ? '' : styles.noArt}`}>
          <div className={styles.poster} style={itemAccentStyle(item.accent)}>
            {episode && still ? (
              <img
                src={apiUrl(still.url)}
                alt=""
                width={still.width}
                height={still.height}
                className={styles.stillImage}
              />
            ) : images.poster ? (
              <img
                src={apiUrl(images.poster.url)}
                srcSet={
                  images.posterLarge
                    ? `${apiUrl(images.poster.url)} 400w, ${apiUrl(images.posterLarge.url)} 800w`
                    : undefined
                }
                sizes={images.posterLarge ? POSTER_SIZES : undefined}
                alt=""
                width={400}
                height={audio ? 400 : 600}
                className={audio ? styles.coverImage : styles.posterImage}
              />
            ) : (
              <span
                className={
                  episode
                    ? styles.stillFallback
                    : audio
                      ? styles.coverFallback
                      : styles.posterFallback
                }
              >
                <PosterFallback />
              </span>
            )}
          </div>

          <div className={styles.info}>
            <div className={styles.heading}>
              {line && <p className={styles.kicker}>{line}</p>}
              <h1 className={styles.title}>{item.title}</h1>
              {item.originalTitle && item.originalTitle !== item.title && (
                <p className={styles.original}>
                  {t('item.originalTitle', { title: item.originalTitle })}
                </p>
              )}
              {bylines(t, item).map((byline) => (
                <p key={byline} className={styles.byline}>
                  {byline}
                </p>
              ))}
            </div>
            <div className={styles.factLines}>
              <ItemMeta item={item} />
              {genres && <p className={styles.genres}>{genres}</p>}
            </div>
            {item.tagline && <p className={styles.tagline}>{item.tagline}</p>}

            <Actions
              item={item}
              file={file}
              onBack={goBack}
              onTrailer={() => setTrailerOpen(true)}
            />
            <RelatedLinks item={item} />

            {progress && (
              <div className={styles.progress}>
                <ProgressBar
                  value={progress.percent}
                  label={progressLabel(t, item, progress.percent)}
                />
                <span className={styles.progressText}>
                  {progressLabel(t, item, progress.percent)}
                </span>
              </div>
            )}

            {item.overview && <p className={styles.overview}>{item.overview}</p>}

            {item.kind === 'series' && <Seasons seasons={item.seasons ?? []} />}
            {item.kind === 'season' && (
              <section className={styles.block} aria-labelledby="item-episodes-heading">
                <h2 id="item-episodes-heading" className={styles.sectionTitle}>
                  {t('item.episodes.title')}
                </h2>
                <EpisodeGrid episodes={item.episodes ?? []} />
              </section>
            )}
            {item.kind === 'audiobook' && (
              <AudiobookChapters item={item} chapters={item.chapters ?? []} playable={playable} />
            )}
            {item.kind === 'podcast' && <PodcastEpisodes item={item} />}

            <Versions item={item} file={file} onChange={setChosenFile} />
          </div>
        </div>

        <div className={styles.rows}>
          <Extras extras={item.extras ?? []} />
          {/* An older server may leave the list out; there is nothing to show then. */}
          <Collections collections={item.collections ?? []} />
          <Cast item={item} />
        </div>

        {trailerOpen && (
          <Suspense fallback={null}>
            <TrailerPanel
              itemId={item.id}
              title={item.title}
              onClose={() => setTrailerOpen(false)}
            />
          </Suspense>
        )}
      </main>
    </SpatialPage>
  );
}
