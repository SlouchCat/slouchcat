import type { Chapter, ItemDetail, ItemRef, MediaFileInfo } from '@slouchcat/api-client';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import {
  formatBytes,
  formatDate,
  formatDuration,
  formatPlayerTime,
  formatRuntime,
  formatResolution,
} from '@/lib/format';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { useOnline } from '@/lib/useOnline';
import {
  Button,
  ButtonLink,
  CastCard,
  Chip,
  ChoiceChips,
  EpisodeRow,
  FactList,
  Notice,
  PosterCard,
  Row,
  Skeleton,
  SpatialButton,
  SpatialButtonLink,
  SpatialGroup,
  SpatialPage,
  setSpatialFocus,
  spatialFocusKey,
} from '@/ui';
import { CoverWatchlist, WATCHLIST_KINDS } from '@/features/discover/CoverWatchlist';
import { CollectionShelf } from './CollectionShelf';
import { DetailActions, PLAY_KEY, playTarget, TRAILER_KEY } from './DetailActions';
import { BACK_KEY, DetailBody, DetailHeader, DetailPage } from './DetailHeader';
import { EDITABLE_KINDS, EditMetadata } from './EditMetadata';
import { ItemMeta } from './ItemMeta';
import { creditCaption, episodeCode, extraKindLabel, isAudioKind } from './labels';
import { itemQuery, podcastEpisodesQuery } from './queries';
import { EpisodeList, SeasonShelf, seasonFacts } from './SeasonShelf';
import styles from './ItemPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/items/$itemId');

// The trailer dialog needs react-aria, which the browsing pages do not load up front (P-10).
const TrailerPanel = lazy(() =>
  import('./TrailerPanel').then((module) => ({ default: module.TrailerPanel })),
);

const watchedProgress = (t: TFunction, item: ItemRef) =>
  item.progress && !item.progress.completed && item.progress.positionMs > 0
    ? {
        percent: item.progress.percent,
        label: t(isAudioKind(item.kind) ? 'item.listened' : 'item.watched', {
          percent: Math.round(item.progress.percent),
        }),
      }
    : undefined;

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

/**
 * The pen on the poster (concept 15 7.4): it opens the metadata editor, for the one who manages
 * the library. A plain button, so Tab reaches it before the actions and the arrow keys pass it
 * by, as on the library's covers.
 */
function PosterPen({ item }: { item: ItemDetail }) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const [editing, setEditing] = useState(false);
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  if (!canManage || !EDITABLE_KINDS.has(item.kind)) return null;
  return (
    <>
      <Button
        variant="secondary"
        size="sm"
        icon="pen"
        iconOnly
        overlay
        aria-label={t('item.editMetadata')}
        title={t('item.editMetadata')}
        onPress={() => setEditing(true)}
      />
      {editing && <EditMetadata itemId={item.id} onClose={() => setEditing(false)} />}
    </>
  );
}

/**
 * Links to the neighbours of an episode or extra: next episode, series, main title; the podcast of
 * a podcast episode.
 */
function RelatedLinks({ item }: { item: ItemDetail }) {
  const { t } = useTranslation();
  const next = item.kind === 'episode' ? item.nextEpisode : undefined;
  // A season names its series as a link over its title (DetailHeader kicker, E-15).
  const series = item.kind === 'episode' ? item.series : undefined;
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

/** Line above the title: "S01E02 · Series" for an episode (a season links its series instead). */
function kicker(t: TFunction, item: ItemDetail): string | undefined {
  const series = item.seriesTitle ?? item.series?.title;
  if (item.kind === 'episode') {
    const code = episodeCode(t, item);
    if (code && series) return t('item.episodeOf', { code, series });
    return code ?? series;
  }
  if (item.kind === 'extra') return extraKindLabel(t, item.extraKind);
  if (item.kind === 'audiobook' && item.seriesName) {
    return item.seriesIndex !== undefined
      ? t('item.audio.volume', { series: item.seriesName, volume: item.seriesIndex })
      : item.seriesName;
  }
  if (item.kind === 'podcast_episode') return item.podcast?.title;
  return undefined;
}

/**
 * Lines under the title: the original title when it differs; "von <Autor>" and "Gelesen von
 * <Sprecher>" of an audiobook or podcast.
 */
function titleLines(t: TFunction, item: ItemDetail): string[] {
  const original =
    item.originalTitle && item.originalTitle !== item.title
      ? t('item.originalTitle', { title: item.originalTitle })
      : undefined;
  if (!isAudioKind(item.kind)) return original ? [original] : [];
  return [
    original,
    item.author ? t('item.audio.by', { author: item.author }) : undefined,
    item.narrator ? t('item.audio.narratedBy', { narrator: item.narrator }) : undefined,
  ].filter((line): line is string => line !== undefined);
}

/** The watchlist button on a cover (concept 17 D-9), for the kinds that go on the watchlist. */
const coverMark = (item: ItemRef) =>
  WATCHLIST_KINDS.has(item.kind) ? <CoverWatchlist item={item} /> : undefined;

/**
 * Detail page of a title (C-2, concept 17): one head for every kind (DetailHeader), then the
 * collections, the seasons and episodes of a series, the chapters or episodes of an audio title or
 * the file, the extras and the cast (E-9).
 */
export function ItemPage() {
  const { itemId } = route.useParams();
  // One state (chosen version, season, dialog) per title: the next episode is the same route.
  return <ItemView key={itemId} itemId={itemId} />;
}

function ItemView({ itemId }: { itemId: string }) {
  const { t, i18n } = useTranslation();
  const query = useQuery(itemQuery(itemId));
  const goBack = useGoBack(paths.home);
  useBackKeys(goBack);
  useDocumentTitle(documentTitle(t, query.data));
  const [chosenFile, setChosenFile] = useState<string | null>(null);
  const [trailerOpen, setTrailerOpen] = useState(false);
  const online = useOnline();
  const item = query.data;
  // The server lists the primary version first (B-5).
  const file = item?.mediaFiles.find((entry) => entry.id === chosenFile) ?? item?.mediaFiles[0];
  const target = item ? playTarget(t, i18n.language, item, file) : undefined;
  const startKey = target?.available ? PLAY_KEY : BACK_KEY;
  // Offline the trailer button and its dialog go away. The dialog stays closed afterwards (it
  // must not come back by itself and autoplay on reconnect), and the focus moves to the big button
  // or the back arrow instead of getting lost on the page body.
  if (!online && trailerOpen) setTrailerOpen(false);
  useEffect(() => {
    if (!online && spatialFocusKey() === TRAILER_KEY) setSpatialFocus(startKey);
  }, [online, startKey]);

  if (query.isPending) {
    return (
      <main className={styles.detail} aria-busy="true">
        <div className={styles.loading}>
          <Skeleton shape="title" />
          <Skeleton shape="line" />
          <Skeleton shape="block" />
        </div>
      </main>
    );
  }

  if (query.isError || !item) {
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

  const audio = isAudioKind(item.kind);
  const series = item.seriesTitle ?? item.series?.title;
  const seriesId = item.series?.id ?? item.parentId;
  const kickerLine =
    item.kind === 'season' && series && seriesId ? (
      <SpatialButtonLink href={paths.item(seriesId)} variant="ghost" icon="folder">
        {series}
      </SpatialButtonLink>
    ) : (
      kicker(t, item)
    );
  const facts =
    item.kind === 'season' ? (
      <p className={styles.muted}>{seasonFacts(t, item).join(t('format.separator'))}</p>
    ) : (
      <ItemMeta item={item} />
    );

  return (
    <SpatialPage defaultFocusKey={startKey}>
      <DetailPage>
        <DetailHeader
          images={item.images}
          accent={item.accent}
          cover={item.kind === 'episode' ? 'still' : audio ? 'square' : 'poster'}
          onBack={goBack}
          coverTool={<PosterPen item={item} />}
          coverMark={coverMark(item)}
          kicker={kickerLine}
          title={item.title}
          logoUrl={imageUrl(item.images.logo)}
          lines={titleLines(t, item)}
          facts={facts}
          genres={item.genres.join(', ')}
          tagline={item.tagline}
          actions={
            <DetailActions
              item={item}
              file={file}
              target={target}
              onTrailer={() => setTrailerOpen(true)}
            />
          }
          related={<RelatedLinks item={item} />}
          overview={item.overview}
        />

        <DetailBody>
          {/* Right after the head (F-6, E-9); an older server may leave the list out. */}
          {(item.collections ?? []).map((collection) => (
            <CollectionShelf
              key={collection.id}
              collection={collection}
              currentId={item.id}
              coverMark={coverMark}
            />
          ))}
          {item.kind === 'series' && (
            <SeasonShelf seasons={item.seasons ?? []} nextUp={item.nextUp} />
          )}
          {item.kind === 'season' && (
            <section className={styles.block} aria-labelledby="item-episodes-heading">
              <h2 id="item-episodes-heading" className={styles.sectionTitle}>
                {t('item.episodes.title')}
              </h2>
              <EpisodeList episodes={item.episodes ?? []} nextUp={item.nextUp} />
            </section>
          )}
          {item.kind === 'audiobook' && (
            <AudiobookChapters
              item={item}
              chapters={item.chapters ?? []}
              playable={target?.available === true}
            />
          )}
          {item.kind === 'podcast' && <PodcastEpisodes item={item} />}
          <Versions item={item} file={file} onChange={setChosenFile} />
          <Extras extras={item.extras ?? []} />
          <Cast item={item} />
        </DetailBody>

        {trailerOpen && (
          <Suspense fallback={null}>
            <TrailerPanel
              itemId={item.id}
              title={item.title}
              onClose={() => setTrailerOpen(false)}
            />
          </Suspense>
        )}
      </DetailPage>
    </SpatialPage>
  );
}
