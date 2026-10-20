import type { PlaybackInfo } from '@slouchcat/api-client';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { itemQuery } from '@/features/catalog/queries';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Notice } from '@/ui';
import { AriaButton } from '@/ui/aria';
import {
  browserPlatform,
  hlsEngine,
  playbackCapabilities,
  type MediaPlatform,
  type PlaybackEngine,
} from './capabilities';
import { useStopPlaybackOnLeave } from './usePlaybackSession';
import { VideoPlayer } from './VideoPlayer';
import styles from './Player.module.css';

const route = getRouteApi('/_authed/_active/play/$itemId');

const containerMime: Record<string, string> = {
  mp4: 'video/mp4',
  m4v: 'video/mp4',
  mov: 'video/quicktime',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
};

/** MIME type for the capability check: the server's (with codecs) or one derived from the container. */
export function playbackMime(info: PlaybackInfo): string {
  return (
    info.mediaFile.mimeType ??
    containerMime[info.mediaFile.container.toLowerCase()] ??
    `video/${info.mediaFile.container.toLowerCase()}`
  );
}

/**
 * How this browser plays the answer of startPlayback: the file directly, HLS natively (Safari) or
 * through hls.js, or not at all. This is a device capability check (W-1), not a visibility rule:
 * the server already decided that the profile may watch the title.
 */
export function playbackEngine(info: PlaybackInfo, platform: MediaPlatform): PlaybackEngine {
  if (info.mode === 'hls') return hlsEngine(platform);
  return platform.canPlayType(playbackMime(info)) !== '' ? 'direct' : 'unsupported';
}

/** Retries of the start request while the server answers 503 `transcode_busy`. */
const BUSY_RETRIES = 3;
/** Wait between those retries when the answer carries no `Retry-After`. */
const BUSY_RETRY_FALLBACK_MS = 1000;

/** How long a version switch waits for the old session to end before it starts the next one. */
const STOP_WAIT_MS = 2000;

const isTranscodeBusy = (error: unknown) =>
  isApiError(error, 503) && error.code === 'transcode_busy';

/** Video player route (W-1 direct play, W-5 progress and resume). */
export function PlayerPage() {
  const { t } = useTranslation();
  const { itemId } = route.useParams();
  // The version to play (B-5): from the link of the detail page or the version menu, kept in
  // the URL so a reload plays the chosen one. The menu also hands over the position.
  const { file } = route.useSearch();
  const navigate = route.useNavigate();
  const queryClient = useQueryClient();
  const [startMs, setStartMs] = useState<number | undefined>(undefined);
  const [switchingTo, setSwitchingTo] = useState<string | undefined>(undefined);
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

  // Starting playback creates a session and may start a transcode on the server, so the request
  // runs once per mount: never refetched on reconnect, focus or staleness (a second session would
  // rebuild the player mid-stream). Only a busy transcoder is retried, after the server's delay.
  const playback = useQuery({
    queryKey: ['playback', itemId, file ?? null],
    queryFn: ({ signal }): Promise<PlaybackInfo> =>
      request(
        api.POST('/items/{id}/play', {
          params: { path: { id: itemId } },
          body: {
            ...(file ? { mediaFileId: file } : {}),
            capabilities: playbackCapabilities(),
          },
          signal,
        }),
      ),
    staleTime: Infinity,
    gcTime: 0,
    // A version switch keeps the paused player on screen until the new session answers.
    placeholderData: keepPreviousData,
    retry: (failureCount, error) => isTranscodeBusy(error) && failureCount < BUSY_RETRIES,
    retryDelay: (_failureCount, error) =>
      (isApiError(error) ? error.retryAfterMs : null) ?? BUSY_RETRY_FALLBACK_MS,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  });
  useDocumentTitle(playback.data?.item.title);
  const stopSession = useStopPlaybackOnLeave(playback.data?.playbackSessionId);
  // The versions of the title for the version menu; the detail page usually has them cached.
  const detail = useQuery({ ...itemQuery(itemId), enabled: playback.isSuccess });
  const versions = detail.data?.mediaFiles ?? [];

  const [infoOpen, setInfoOpen] = useState(false);

  // From the menu until the new session answered; the old player is paused and locked meanwhile.
  const switching =
    playback.isPlaceholderData || (switchingTo !== undefined && switchingTo !== file);

  const switchVersion = async (fileId: string, positionMs: number) => {
    const current = playback.data;
    if (!current || current.mediaFile.id === fileId || switching) return;
    setSwitchingTo(fileId);
    // The running session (and its transcode) ends before the next one starts: with few transcode
    // slots the server would otherwise answer the start with 503. A slow answer holds the switch
    // up for STOP_WAIT_MS at most.
    await Promise.race([
      stopSession(),
      new Promise((resolve) => window.setTimeout(resolve, STOP_WAIT_MS)),
    ]);
    if (!mounted.current) return;
    setStartMs(positionMs);
    await navigate({ search: (previous) => ({ ...previous, file: fileId }), replace: true });
  };

  const engine = useMemo(
    () => (playback.data ? playbackEngine(playback.data, browserPlatform()) : null),
    [playback.data],
  );

  // Progress changed while watching: the start page and the detail page must refetch.
  useEffect(
    () => () => {
      void queryClient.invalidateQueries({ queryKey: ['home'] });
      void queryClient.invalidateQueries({ queryKey: ['item', itemId] });
    },
    [itemId, queryClient],
  );

  if (playback.data && engine && engine !== 'unsupported') {
    return (
      <VideoPlayer
        // A new session (another version) gets a fresh element and progress reporter.
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
      />
    );
  }

  return (
    <PlayerMessage onBack={goBack}>
      {playback.isPending && !isTranscodeBusy(playback.failureReason) && (
        <p className={styles.loading}>{t('player.loading')}</p>
      )}
      {playback.isPending && isTranscodeBusy(playback.failureReason) && (
        <Notice tone="info" title={t('player.busy.title')}>
          {t('player.busy.body')}
        </Notice>
      )}
      {playback.isError && (
        <Notice tone="danger" title={t('player.error.title')}>
          {problemMessage(t, playback.error)}
        </Notice>
      )}
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

function PlayerMessage({ children, onBack }: { children: ReactNode; onBack: () => void }) {
  const { t } = useTranslation();
  useBackKeys(onBack);
  return (
    <main className={styles.messagePage}>
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
