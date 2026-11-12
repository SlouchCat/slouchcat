import type { ItemDetail, ItemRef, MediaFileInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { lazy, Suspense, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import {
  useRecommendMutation,
  useWatchlistMutation,
  useWatchlistPending,
} from '@/features/discover/queries';
import { isDownloadable, offlineSupported } from '@/features/offline/support';
import { formatDate, formatDuration, formatPlayerTime, formatRuntime } from '@/lib/format';
import { useOnline } from '@/lib/useOnline';
import { Button, Notice, RoundAction, SpatialButtonLink, SpatialGroup } from '@/ui';
import { EDITABLE_KINDS } from './EditMetadata';
import { episodeCode, isAudioKind } from './labels';
import styles from './ItemPage.module.css';

// "Herunterladen" and the download state (M5) read the offline database: a chunk of their own,
// loaded only where the browser can keep downloads.
const DownloadAction = lazy(() =>
  import('@/features/offline/DownloadAction').then((module) => ({
    default: module.DownloadAction,
  })),
);

export const PLAY_KEY = 'detail-play';
export const TRAILER_KEY = 'detail-trailer';
const RESTART_KEY = 'detail-restart';
const DOWNLOAD_KEY = 'detail-download';
const WATCHLIST_KEY = 'detail-watchlist';
export const RECOMMEND_KEY = 'detail-recommend';

type NextUp = NonNullable<ItemDetail['nextUp']>;

/**
 * Whether the title's own file plays: a podcast episode streams from its feed through the server;
 * an audiobook plays all its parts; any other title needs the chosen version's file. Podcasts play
 * only through their episodes. Series and seasons do not come here: they play their next episode
 * (nextUpTarget).
 */
function canPlay(item: ItemDetail, file: MediaFileInfo | undefined): boolean {
  switch (item.kind) {
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

/** What the big button says and starts (concept 17 3.2, 3.3; E-10). */
export interface PlayTarget {
  label: string;
  /** The second line: the episode, the position and the time left, the runtime. */
  detail?: string;
  href: string;
  /** Progress of the title (or episode) the button resumes, 0 … 100. */
  percent?: number;
  /** Where the button starts from the beginning ("Von vorne"); only while resuming. */
  restartHref?: string;
  available: boolean;
}

const started = (progress: ItemRef['progress']) =>
  progress && !progress.completed && progress.positionMs > 0 ? progress : undefined;

const timeLeft = (t: TFunction, progress: NonNullable<ItemRef['progress']>) =>
  formatDuration(t, Math.max(0, progress.durationMs - progress.positionMs));

function nextUpTarget(t: TFunction, nextUp: NextUp): PlayTarget {
  const { episode, reason } = nextUp;
  const code = episodeCode(t, episode);
  const line = code ? t('item.nextUp.episode', { code, title: episode.title }) : episode.title;
  // The next episode may be started already: then it resumes like "Weiterschauen".
  const resume = reason === 'resume' || reason === 'next' ? started(episode.progress) : undefined;
  let detail = line;
  if (resume) detail = t('item.nextUp.left', { episode: line, left: timeLeft(t, resume) });
  if (reason === 'rewatch')
    detail = t('item.nextUp.rewatchFrom', { episode: code ?? episode.title });
  return {
    label: t(`item.nextUp.${reason}`),
    detail,
    href: reason === 'rewatch' ? paths.playAt(episode.id, 0) : paths.play(episode.id),
    percent: resume?.percent,
    restartHref: resume ? paths.playAt(episode.id, 0) : undefined,
    available: true,
  };
}

/**
 * The big button of a title: series and seasons by `nextUp`, every other title by its own
 * progress (Abspielen, Fortsetzen, Nochmals ansehen). None for a podcast (its episodes play) and for
 * a series or season without a visible episode.
 */
export function playTarget(
  t: TFunction,
  language: string,
  item: ItemDetail,
  file: MediaFileInfo | undefined,
): PlayTarget | undefined {
  if (item.kind === 'podcast') return undefined;
  if (item.kind === 'series' || item.kind === 'season') {
    return item.nextUp ? nextUpTarget(t, item.nextUp) : undefined;
  }
  const audio = isAudioKind(item.kind);
  // Only one version: the server picks it; with several the chosen one goes along. An audiobook's
  // files are its parts, not versions.
  const version = !audio && item.mediaFiles.length > 1 ? file?.id : undefined;
  const available = canPlay(item, file);
  const resume = started(item.progress);
  if (resume) {
    return {
      label: t(audio ? 'item.action.resumeListening' : 'item.action.resume'),
      detail: t('item.action.resumeAt', {
        time: formatPlayerTime(resume.positionMs, resume.durationMs),
        left: timeLeft(t, resume),
      }),
      href: paths.play(item.id, version),
      percent: resume.percent,
      restartHref: paths.playAt(item.id, 0, version),
      available,
    };
  }
  if (item.progress?.completed) {
    return {
      label: t(audio ? 'item.action.againListening' : 'item.action.again'),
      detail: t(audio ? 'item.action.listenedOn' : 'item.action.watchedOn', {
        date: formatDate(item.progress.updatedAt, language),
      }),
      href: paths.playAt(item.id, 0, version),
      available,
    };
  }
  const length = item.runtimeMinutes
    ? formatRuntime(t, item.runtimeMinutes)
    : item.durationMs
      ? formatDuration(t, item.durationMs)
      : undefined;
  return { label: t('item.play'), detail: length, href: paths.play(item.id, version), available };
}

function PlayContent({ target }: { target: PlayTarget }) {
  return (
    <span className={styles.playText}>
      <span className={styles.playLabel}>{target.label}</span>{' '}
      {target.detail && <span className={styles.playDetail}>{target.detail}</span>}
      {target.percent !== undefined && (
        <span className={styles.playStroke} aria-hidden="true">
          <span style={{ width: `${target.percent}%` }} />
        </span>
      )}
    </span>
  );
}

interface DetailActionsProps {
  item: ItemDetail;
  /** The version the play link starts (B-5); undefined for titles without files. */
  file: MediaFileInfo | undefined;
  target: PlayTarget | undefined;
  onTrailer: () => void;
}

/**
 * The actions of a detail page (concept 17 D-2, variant A; E-10, E-11): the big button with its
 * second line and progress stroke, under it the round actions in this order: Trailer, Von vorne,
 * Merken, Empfehlen, Download, Verwalten. Offline Merken and Empfehlen are disabled (they live on
 * the server), like the cover's watchlist button. «Empfehlen» (concept 18 part B) stands only
 * where the server says it applies (`canRecommend`: the owner's account, no season or extra); a press puts the title into the user picks or takes the own pick back, and a
 * hint under the row says so.
 */
export function DetailActions({ item, file, target, onTrailer }: DetailActionsProps) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const online = useOnline();
  const audio = isAudioKind(item.kind);
  const version = !audio && item.mediaFiles.length > 1 ? file?.id : undefined;
  // A shortcut for the one who manages the library, not a visibility rule: the server answers the
  // administration page only for that account and profile anyway.
  const canManage = session.account.role === 'admin' && session.profile.isPrimary;
  // Every kind but seasons and extras goes on the watchlist (E-5); a series or podcast there
  // notifies about new episodes (concept 20 P-2, in place of the favourite of E-8).
  const listable = item.kind !== 'season' && item.kind !== 'extra';
  const watchlist = useWatchlistMutation();
  const watchlistPending = useWatchlistPending(item.id);
  const recommend = useRecommendMutation();
  const [pickHint, setPickHint] = useState<string | null>(null);
  const failed = watchlist.error ?? recommend.error;

  return (
    <>
      {target && (
        <SpatialGroup className={styles.play}>
          {target.available ? (
            <SpatialButtonLink
              focusKey={PLAY_KEY}
              href={target.href}
              variant="primary"
              size="lg"
              icon="play"
            >
              <PlayContent target={target} />
            </SpatialButtonLink>
          ) : (
            <Button variant="primary" size="lg" icon="play" isDisabled>
              <PlayContent target={target} />
            </Button>
          )}
        </SpatialGroup>
      )}
      {/* One row in its order for arrows and Tab (ordered): on a phone the round actions wrap,
          and → must still walk them in this order. */}
      <SpatialGroup ordered className={styles.roundActions}>
        {(item.trailerCount ?? 0) > 0 && online && (
          <RoundAction
            focusKey={TRAILER_KEY}
            icon="play"
            label={t('item.trailer')}
            onPress={onTrailer}
          />
        )}
        {target?.restartHref && target.available && (
          <RoundAction
            focusKey={RESTART_KEY}
            icon="replay"
            label={t('item.action.restart')}
            href={target.restartHref}
          />
        )}
        {listable && (
          <RoundAction
            focusKey={WATCHLIST_KEY}
            icon="plus"
            label={item.inWatchlist ? t('item.watchlist.added') : t('item.watchlist.add')}
            pressed={item.inWatchlist}
            isDisabled={!online}
            busy={watchlistPending}
            onPress={() => watchlist.mutate({ id: item.id, on: !item.inWatchlist })}
          />
        )}
        {item.canRecommend && (
          <RoundAction
            focusKey={RECOMMEND_KEY}
            icon="star"
            label={item.recommendedByMe ? t('item.recommend.done') : t('item.recommend.add')}
            pressed={item.recommendedByMe}
            description={t('item.recommend.hint')}
            isDisabled={!online}
            busy={recommend.isPending}
            onPress={() => {
              setPickHint(null);
              recommend.mutate(
                { id: item.id, on: !item.recommendedByMe },
                {
                  onSuccess: (pick) =>
                    setPickHint(
                      pick
                        ? t('item.recommend.added', { count: pick.retentionDays })
                        : t('item.recommend.removed'),
                    ),
                },
              );
            }}
          />
        )}
        {offlineSupported() && isDownloadable(item.kind) && (
          <Suspense fallback={null}>
            <DownloadAction
              target={item}
              mediaFileId={version}
              available={item.kind === 'season' || target?.available === true}
              focusKey={DOWNLOAD_KEY}
              appearance="round"
            />
          </Suspense>
        )}
        {/* Titles the metadata editor does not edit keep the link to their administration page;
            the others have the pen on the cover (PosterPen). */}
        {canManage && !EDITABLE_KINDS.has(item.kind) && (
          <RoundAction icon="info" label={t('item.manage')} href={paths.adminItem(item.id)} />
        )}
      </SpatialGroup>
      {pickHint && !failed && <Notice title={pickHint} />}
      {failed && <Notice tone="danger" title={problemMessage(t, failed)} />}
    </>
  );
}
