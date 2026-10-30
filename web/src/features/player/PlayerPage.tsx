import type { ItemRef, PlaybackInfo, PlaybackRequest, Quality } from '@slouchcat/api-client';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemExtension, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { itemQuery } from '@/features/catalog/queries';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Notice } from '@/ui';
import { AriaButton } from '@/ui/aria';
import {
  browserPlatform,
  deviceCapabilities,
  hlsEngine,
  type MediaPlatform,
  type PlaybackEngine,
} from './capabilities';
import { AudioPlayerPage } from './AudioPlayerPage';
import { useStopPlaybackOnLeave } from './usePlaybackSession';
import { VideoPlayer, type PlaybackChange } from './VideoPlayer';
import styles from './Player.module.css';

const route = getRouteApi('/_authed/_active/play/$itemId');

const containerMime: Record<string, string> = {
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  mov: 'video/quicktime',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
  // Audiobooks and podcast episodes (M6).
  mp3: 'audio/mpeg',
  m4a: 'audio/mp4',
  m4b: 'audio/mp4',
  aac: 'audio/aac',
  flac: 'audio/flac',
  ogg: 'audio/ogg',
  opus: 'audio/ogg',
};

/**
 * MIME type for the capability check: the server's (with codecs) or one derived from the container.
 * For audio a known container comes first: an episode's type is the feed's, often non-standard
 * (`audio/x-mp3`, `audio/mpeg3`), which browsers do not recognise although they play the file.
 */
export function playbackMime(info: PlaybackInfo): string {
  const known = containerMime[info.mediaFile.container.toLowerCase()];
  if (info.mediaKind === 'audio' && known) return known;
  return (
    info.mediaFile.mimeType ??
    known ??
    `${info.mediaKind}/${info.mediaFile.container.toLowerCase()}`
  );
}

/**
 * How this browser plays the answer of startPlayback: the file directly, HLS natively (Safari) or
 * through hls.js, or not at all. This is a device capability check (W-1), not a visibility rule:
 * the server already decided that the profile may watch the title. Audio of an unknown container
 * (a feed's enclosure) is tried directly: the element's error shows a failure.
 */
export function playbackEngine(info: PlaybackInfo, platform: MediaPlatform): PlaybackEngine {
  if (info.mode === 'hls') return hlsEngine(platform);
  if (platform.canPlayType(playbackMime(info)) !== '') return 'direct';
  const unknown = !containerMime[info.mediaFile.container.toLowerCase()];
  return info.mediaKind === 'audio' && unknown ? 'direct' : 'unsupported';
}

/** Retries of the start request while the server answers 503 `transcode_busy`. */
const BUSY_RETRIES = 3;
/** Wait between those retries when the answer carries no `Retry-After`. */
const BUSY_RETRY_FALLBACK_MS = 1000;

/** How long a switch waits for the old session to end before it starts the next one. */
const STOP_WAIT_MS = 2000;

const isTranscodeBusy = (error: unknown) =>
  isApiError(error, 503) && error.code === 'transcode_busy';

/** The tracks the viewer picked, for the file that was playing then (W-4). */
interface TrackChoice {
  /** `?file` of the URL at the time; a choice does not carry over to another version. */
  file: string | null;
  audioTrack?: number;
  subtitleTrack?: number | null;
}

/** Body of startPlayback: the device profile, the version and the viewer's choices. */
export function playbackRequest(
  file: string | undefined,
  tracks: Omit<TrackChoice, 'file'>,
  quality: Quality | undefined,
  positionMs: number | undefined,
): PlaybackRequest {
  return {
    ...(file ? { mediaFileId: file } : {}),
    capabilities: deviceCapabilities(),
    ...(tracks.audioTrack !== undefined ? { audioTrack: tracks.audioTrack } : {}),
    ...(tracks.subtitleTrack !== undefined ? { subtitleTrack: tracks.subtitleTrack } : {}),
    subtitleDelivery: 'auto',
    ...(quality ? { quality } : {}),
    ...(positionMs !== undefined ? { positionMs } : {}),
  };
}

/**
 * The item an auto-next navigation is heading for; the next player marks itself with
 * `data-autonext="navigated"` (W-9 measurement) and clears it.
 */
let autoNextTarget: string | null = null;

/** Video player route (W-1 direct play, W-4 tracks, W-5 progress, W-8 quality, W-9 auto-next). */
export function PlayerPage() {
  const { itemId } = route.useParams();
  // Full screen belongs to the document so that it survives switches and auto-next (W-2); it
  // ends with the player route.
  useEffect(
    () => () => {
      if (document.fullscreenElement) void document.exitFullscreen().catch(() => undefined);
    },
    [],
  );
  // Auto-next stays on this route with another id: a fresh player, session and measurement.
  return <PlayerView key={itemId} itemId={itemId} />;
}

function PlayerView({ itemId }: { itemId: string }) {
  const { t } = useTranslation();
  // The version to play (B-5): from the link of the detail page or the version menu, kept in
  // the URL so a reload plays the chosen one. The menu also hands over the position; `at` starts
  // at a chapter of an audiobook (its link on the detail page).
  const { file, at } = route.useSearch();
  const navigate = route.useNavigate();
  const queryClient = useQueryClient();
  const [startMs, setStartMs] = useState<number | undefined>(at);
  const [switchingTo, setSwitchingTo] = useState<string | undefined>(undefined);
  const [stopping, setStopping] = useState(false);
  const [switchStartedAt, setSwitchStartedAt] = useState<number | undefined>(undefined);
  const [trackChoice, setTrackChoice] = useState<TrackChoice>({ file: null });
  const [quality, setQuality] = useState<Quality | undefined>(undefined);
  // A new session with the same choices (a second switch back) must not reuse a cached answer.
  const [generation, setGeneration] = useState(0);
  const [autoNext] = useState(() => {
    const arrived = autoNextTarget === itemId;
    if (arrived) autoNextTarget = null;
    return arrived;
  });
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // Start of the measurement "navigation → first frame playing" (data-start-ms).
  const [openedAt] = useState(() => performance.now());
  const goBack = useGoBack(paths.item(itemId));

  const tracks =
    trackChoice.file === (file ?? null)
      ? { audioTrack: trackChoice.audioTrack, subtitleTrack: trackChoice.subtitleTrack }
      : {};

  // Starting playback creates a session and may start a transcode on the server, so the request
  // runs once per mount and choice: never refetched on reconnect, focus or staleness (a second
  // session would rebuild the player mid-stream). Only a busy transcoder is retried, after the
  // server's delay.
  const playback = useQuery({
    queryKey: ['playback', itemId, file ?? null, tracks, quality ?? null, generation],
    queryFn: ({ signal }): Promise<PlaybackInfo> =>
      request(
        api.POST('/items/{id}/play', {
          params: { path: { id: itemId } },
          body: playbackRequest(file, tracks, quality, startMs),
          signal,
        }),
      ),
    staleTime: Infinity,
    gcTime: 0,
    // A switch keeps the paused player on screen until the new session answers.
    placeholderData: keepPreviousData,
    retry: (failureCount, error) => isTranscodeBusy(error) && failureCount < BUSY_RETRIES,
    retryDelay: (_failureCount, error) =>
      (isApiError(error) ? error.retryAfterMs : null) ?? BUSY_RETRY_FALLBACK_MS,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  // `at` starts this player only: once the session runs it leaves the URL, so a reload (or back
  // and forward) resumes from the saved position instead of the chapter's start.
  const started = playback.isSuccess;
  useEffect(() => {
    if (at === undefined || !started) return;
    void navigate({ search: (previous) => ({ ...previous, at: undefined }), replace: true });
  }, [at, navigate, started]);
  useDocumentTitle(playback.data?.item.title);
  const stopSession = useStopPlaybackOnLeave(playback.data?.playbackSessionId);
  // The versions of the title for the version menu; the detail page usually has them cached.
  const detail = useQuery({ ...itemQuery(itemId), enabled: playback.isSuccess });
  const versions = detail.data?.mediaFiles ?? [];

  const [infoOpen, setInfoOpen] = useState(false);

  // From the menu until the new session answered; the old player is paused and locked meanwhile.
  const switching =
    playback.isPlaceholderData || stopping || (switchingTo !== undefined && switchingTo !== file);

  /**
   * The running session (and its transcode) ends before the next one starts: with few transcode
   * slots the server would otherwise answer the start with 503, and a stream limit of one would
   * answer 409. A slow answer holds the switch up for STOP_WAIT_MS at most.
   */
  const endCurrent = async () => {
    setSwitchStartedAt(performance.now());
    setStopping(true);
    await Promise.race([
      stopSession(),
      new Promise((resolve) => window.setTimeout(resolve, STOP_WAIT_MS)),
    ]);
    return mounted.current;
  };

  const switchVersion = async (fileId: string, positionMs: number) => {
    const current = playback.data;
    if (!current || current.mediaFile.id === fileId || switching) return;
    setSwitchingTo(fileId);
    if (!(await endCurrent())) return;
    setStartMs(positionMs);
    setStopping(false);
    await navigate({ search: (previous) => ({ ...previous, file: fileId }), replace: true });
  };

  // Another track or quality (W-4, W-8): a new session of the same file at the same position.
  const switchPlayback = async (change: PlaybackChange, positionMs: number) => {
    if (!playback.data || switching) return;
    if (!(await endCurrent())) return;
    setStartMs(positionMs);
    if (change.quality !== undefined) setQuality(change.quality);
    if (change.audioTrack !== undefined || change.subtitleTrack !== undefined) {
      setTrackChoice({
        ...tracks,
        file: file ?? null,
        ...(change.audioTrack !== undefined ? { audioTrack: change.audioTrack } : {}),
        ...(change.subtitleTrack !== undefined ? { subtitleTrack: change.subtitleTrack } : {}),
      });
    }
    setGeneration((value) => value + 1);
    setStopping(false);
  };

  const playNext = (next: ItemRef) => {
    autoNextTarget = next.id;
    void navigate({ to: '/play/$itemId', params: { itemId: next.id }, search: {}, replace: true });
  };

  const engine = useMemo(
    () => (playback.data ? playbackEngine(playback.data, browserPlatform()) : null),
    [playback.data],
  );

  // Progress changed while watching: the start, library and detail pages must refetch.
  useEffect(
    () => () => {
      void queryClient.invalidateQueries({ queryKey: ['home'] });
      void queryClient.invalidateQueries({ queryKey: ['library'] });
      void queryClient.invalidateQueries({ queryKey: ['item', itemId] });
    },
    [itemId, queryClient],
  );

  if (playback.data && engine && engine !== 'unsupported' && playback.data.mediaKind === 'audio') {
    return (
      <AudioPlayerPage
        key={playback.data.playbackSessionId}
        info={playback.data}
        engine={engine}
        openedAt={openedAt}
        onBack={goBack}
        startPositionMs={startMs}
        detail={detail.data}
      />
    );
  }

  if (playback.data && engine && engine !== 'unsupported') {
    return (
      <VideoPlayer
        // A new session (another version, track or quality) gets a fresh element and reporter.
        key={playback.data.playbackSessionId}
        info={playback.data}
        engine={engine}
        openedAt={openedAt}
        onBack={goBack}
        startPositionMs={startMs}
        versions={versions}
        infoOpen={infoOpen}
        onToggleInfo={() => setInfoOpen((open) => !open)}
        switching={switching}
        onSwitchVersion={(fileId, positionMs) => void switchVersion(fileId, positionMs)}
        onSwitch={(change, positionMs) => void switchPlayback(change, positionMs)}
        switchStartedAt={switchStartedAt}
        onNextEpisode={playNext}
        autoNext={autoNext}
      />
    );
  }

  return (
    <PlayerMessage
      onBack={goBack}
      errorCode={playback.isError && isApiError(playback.error) ? playback.error.code : undefined}
    >
      {playback.isPending && !isTranscodeBusy(playback.failureReason) && (
        <p className={styles.loading}>{t('player.loading')}</p>
      )}
      {playback.isPending && isTranscodeBusy(playback.failureReason) && (
        <Notice tone="info" title={t('player.busy.title')}>
          {t('player.busy.body')}
        </Notice>
      )}
      {playback.isError && <StartError error={playback.error} />}
      {playback.data && engine === 'unsupported' && (
        <Notice tone="warning" title={t('player.unsupported.title')}>
          <p>{t('player.unsupported.body')}</p>
          <p className={styles.detailsLine}>
            {t('player.unsupported.details', {
              container: playback.data.mediaFile.container.toUpperCase(),
              video: playback.data.mediaFile.videoCodec?.toUpperCase() ?? '–',
              audio: playback.data.mediaFile.audioCodec?.toUpperCase() ?? '–',
            })}
          </p>
        </Notice>
      )}
    </PlayerMessage>
  );
}

/**
 * Why the server did not start the playback. The limits (K-5 watch time, W-8 streams) carry
 * their numbers in the problem's extensions; the client only shows them.
 */
function StartError({ error }: { error: Error }) {
  const { t } = useTranslation();
  if (isApiError(error, 403) && error.code === 'watch_limit_reached') {
    const limit = problemExtension(error, 'limitMinutes');
    const used = problemExtension(error, 'usedMinutes');
    return (
      <Notice tone="warning" title={t('player.watchLimit.title')}>
        <p>
          {limit !== undefined
            ? t('player.watchLimit.body', { count: limit })
            : t('player.watchLimit.bodyUnknown')}
        </p>
        {used !== undefined && <p>{t('player.watchLimit.used', { count: used })}</p>}
      </Notice>
    );
  }
  if (isApiError(error, 409) && error.code === 'stream_limit_reached') {
    const max = problemExtension(error, 'maxStreams');
    const active = problemExtension(error, 'activeStreams');
    return (
      <Notice tone="warning" title={t('player.streamLimit.title')}>
        <p>
          {max !== undefined
            ? t('player.streamLimit.body', { count: max })
            : t('player.streamLimit.bodyUnknown')}
        </p>
        {active !== undefined && <p>{t('player.streamLimit.active', { count: active })}</p>}
      </Notice>
    );
  }
  return (
    <Notice tone="danger" title={t('player.error.title')}>
      {problemMessage(t, error)}
    </Notice>
  );
}

function PlayerMessage({
  children,
  onBack,
  errorCode,
}: {
  children: ReactNode;
  onBack: () => void;
  errorCode?: string;
}) {
  const { t } = useTranslation();
  useBackKeys(onBack);
  return (
    <main className={styles.messagePage} data-error-code={errorCode}>
      <div className={styles.messageBox}>
        {children}
        <div>
          <AriaButton variant="secondary" icon="back" onPress={onBack} autoFocus>
            {t('common.back')}
          </AriaButton>
        </div>
      </div>
    </main>
  );
}
