import type {
  ItemRef,
  MediaFileInfo,
  PlaybackInfo,
  PlaybackRequest,
  Quality,
  Track,
} from '@slouchcat/api-client';
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemExtension, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { itemQuery } from '@/features/catalog/queries';
import { useBackKeys } from '@/lib/useBackKeys';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useGoBack } from '@/lib/useGoBack';
import { Notice } from '@/ui';
import { AriaButton, focusOnMount } from '@/ui/aria';
import {
  browserPlatform,
  deviceCapabilities,
  hlsEngine,
  type MediaPlatform,
  type PlaybackEngine,
} from './capabilities';
import { AudioPlayerPage } from './AudioPlayerPage';
import { playbackCapabilities } from './capabilities';
import { selectedAudio, selectedSubtitle } from './labels';
import { baseLanguage, isCommentary, isSdh } from './trackOrder';
import { useStopPlaybackOnLeave } from './usePlaybackSession';
import { readPlayerPrefs, rememberedQuality } from './usePlayerPrefs';
import { NO_CARRY, VideoPlayer, type PlaybackChange, type PlayerCarry } from './VideoPlayer';
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

/**
 * The tracks a start names, for the file that was playing then (W-4). What it leaves out the
 * server chooses (contract 1.11.0, 11.3): only the first start does so, every switch names both.
 */
export interface TrackChoice {
  /** `?file` of the URL at the time; a choice does not carry over to another version. */
  file: string | null;
  audioTrack?: number;
  /** A subtitle track id, or `off` for none. */
  subtitle?: number | 'off';
  /** The viewer chose by hand during an episode and the server keeps it for the series. */
  rememberForSeries?: boolean;
}

/**
 * Body of startPlayback: the device profile, the version and the tracks. «Aus» is `subtitleOff`,
 * never `subtitleTrack: null`, which since contract 1.11.0 lets the server choose. `quality` is a
 * choice in this player (the panel, the hint of a slow connection); without one the quality
 * remembered on this device goes along (`rememberedQuality`, V-17).
 */
export function playbackRequest(
  file: string | undefined,
  tracks: Omit<TrackChoice, 'file'>,
  quality: Quality | undefined,
  positionMs: number | undefined,
  remembered?: Quality,
): PlaybackRequest {
  const { audioTrack, subtitle, rememberForSeries } = tracks;
  return {
    ...(file ? { mediaFileId: file } : {}),
    capabilities: deviceCapabilities(),
    ...(audioTrack !== undefined ? { audioTrack } : {}),
    ...(subtitle === 'off' ? { subtitleOff: true } : {}),
    ...(typeof subtitle === 'number' ? { subtitleTrack: subtitle } : {}),
    ...(rememberForSeries ? { rememberForSeries: true } : {}),
    subtitleDelivery: 'auto',
    ...(quality ? { quality } : rememberedQuality(remembered)),
    ...(positionMs !== undefined ? { positionMs } : {}),
  };
}

/**
 * The tracks of a switch (11.3): the change, and for what it leaves out the tracks the running
 * session delivers, so a sound or quality switch keeps the subtitles (or «Aus») as they are.
 */
export function switchTracks(
  info: PlaybackInfo,
  change: PlaybackChange,
): Omit<TrackChoice, 'file'> {
  const audioTrack = change.audioTrack ?? info.selection.audioTrack;
  return {
    ...(audioTrack !== undefined ? { audioTrack } : {}),
    subtitle: change.subtitleTrack ?? info.selection.subtitleTrack ?? 'off',
    ...(change.rememberForSeries &&
    (change.audioTrack !== undefined || change.subtitleTrack !== undefined)
      ? { rememberForSeries: true }
      : {}),
  };
}

/**
 * The first matching track in the order the server prefers within a language (review step 9, m-3,
 * as trackpick's audioIn and pick): not a commentary, then one that `prefer` holds for (a sound the
 * device decodes; a subtitle as SDH as the running one), then text before bitmap, then the default
 * one, then the file's order. Only an order: the server still decides what plays (rule 12).
 */
function best(
  tracks: readonly Track[] | undefined,
  match: (track: Track) => boolean,
  prefer: (track: Track) => boolean,
) {
  return tracks
    ?.filter(match)
    .sort(
      (a, b) =>
        Number(isCommentary(a)) - Number(isCommentary(b)) ||
        Number(prefer(b)) - Number(prefer(a)) ||
        Number(b.text) - Number(a.text) ||
        Number(b.default) - Number(a.default),
    )[0];
}

/**
 * The tracks of a version switch (B-5, 11.3): ids and stream indexes belong to one file, so the
 * running tracks are looked up in the other version by language (and forced); within it a sound in
 * a codec of `decodableAudio` (the device's, playbackCapabilities) and a subtitle that is SDH
 * exactly when the running one is come first (final review stage 2, G1-m1). What it lacks the
 * server chooses. «Aus» stays off.
 */
export function versionTracks(
  info: PlaybackInfo,
  version: MediaFileInfo | undefined,
  decodableAudio: readonly string[] = playbackCapabilities().audio,
): Omit<TrackChoice, 'file'> {
  const audio = selectedAudio(info);
  const subtitle = selectedSubtitle(info);
  const other = version?.tracks;
  const audioTrack = audio
    ? best(
        other?.audio,
        (track) =>
          baseLanguage(track.language) !== undefined &&
          baseLanguage(track.language) === baseLanguage(audio.language),
        (track) => decodableAudio.includes(track.codec.toLowerCase()),
      )?.index
    : undefined;
  const subtitleId = subtitle
    ? best(
        other?.subtitles,
        (track) =>
          track.forced === subtitle.forced &&
          baseLanguage(track.language) !== undefined &&
          baseLanguage(track.language) === baseLanguage(subtitle.language),
        (track) => isSdh(track) === isSdh(subtitle),
      )?.id
    : undefined;
  return {
    ...(audioTrack !== undefined ? { audioTrack } : {}),
    ...(subtitle === undefined ? { subtitle: 'off' as const } : {}),
    ...(subtitleId !== undefined ? { subtitle: subtitleId } : {}),
  };
}

/**
 * The item an auto-next navigation is heading for; the next player marks itself with
 * `data-autonext="navigated"` (W-9 measurement) and clears it.
 */
let autoNextTarget: string | null = null;

/**
 * What the player hands on to the next title it plays itself (auto-next, an episode of «Als
 * Nächstes», «Folge zurück»): night mode and the volume, a boost above 100 % included. Web Audio
 * stays on until the viewer leaves the player (11.10); mute and speed start afresh with a new
 * title. Taken by the player of that title; reset when the player route closes.
 */
let nextTitleCarry: { itemId: string; carry: PlayerCarry } | null = null;

/** Counts the players opened, so each one starts its own session (see the playback query). */
let playerMounts = 0;

/** Video player route (W-1 direct play, W-4 tracks, W-5 progress, W-8 quality, W-9 auto-next). */
export function PlayerPage() {
  const { itemId } = route.useParams();
  // Full screen belongs to the document so that it survives switches and auto-next (W-2); it
  // ends with the player route, and so does what auto-next carries over.
  useEffect(
    () => () => {
      nextTitleCarry = null;
      if (document.fullscreenElement) void document.exitFullscreen().catch(() => undefined);
    },
    [],
  );
  // «Für diese Serie merken» (11.3): on by default, kept while this player route is open, across
  // auto-next and the other episodes it plays (review step 9, m-4).
  const [rememberSeries, setRememberSeries] = useState(true);
  // Auto-next stays on this route with another id: a fresh player, session and measurement.
  return (
    <PlayerView
      key={itemId}
      itemId={itemId}
      rememberSeries={rememberSeries}
      onRememberSeries={setRememberSeries}
    />
  );
}

function PlayerView({
  itemId,
  rememberSeries,
  onRememberSeries,
}: {
  itemId: string;
  rememberSeries: boolean;
  onRememberSeries: (on: boolean) => void;
}) {
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
  // Neither must a player opened again: the one left ends its session on a timer, and the way
  // back can be quicker than that timer and the removal of the answer (gcTime 0).
  const [mount] = useState(() => (playerMounts += 1));
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
  // A navigation to another page (a person, another episode, back) keeps this player on screen
  // until the next page has loaded; meanwhile its keys belong to that page (usePlayerKeys).
  const router = useRouter();
  const leaving = useCallback(() => {
    const { status, location, resolvedLocation } = router.state;
    return status === 'pending' && location.pathname !== resolvedLocation?.pathname;
  }, [router]);

  const tracks: Omit<TrackChoice, 'file'> =
    trackChoice.file === (file ?? null)
      ? {
          audioTrack: trackChoice.audioTrack,
          subtitle: trackChoice.subtitle,
          rememberForSeries: trackChoice.rememberForSeries,
        }
      : {};

  // Starting playback creates a session and may start a transcode on the server, so the request
  // runs once per mount and choice: never refetched on reconnect, focus or staleness (a second
  // session would rebuild the player mid-stream). Only a busy transcoder is retried, after the
  // server's delay.
  const playback = useQuery({
    queryKey: ['playback', itemId, file ?? null, tracks, quality ?? null, generation, mount],
    queryFn: ({ signal }): Promise<PlaybackInfo> =>
      request(
        api.POST('/items/{id}/play', {
          params: { path: { id: itemId } },
          body: playbackRequest(file, tracks, quality, startMs, readPlayerPrefs().quality),
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
  // Mute, speed, a volume above 100 % and night mode carry over to the next session of this title
  // (a switch of track, quality or version); a new title starts audible at normal speed, after
  // auto-next with the night mode and volume of the episode before (m-5, review step 4).
  const [carried, setCarried] = useState<PlayerCarry>(() => {
    const handed = nextTitleCarry?.itemId === itemId ? nextTitleCarry.carry : undefined;
    nextTitleCarry = null;
    return handed ? { ...NO_CARRY, nightMode: handed.nightMode, volume: handed.volume } : NO_CARRY;
  });
  const carry = useCallback(
    (patch: Partial<PlayerCarry>) =>
      setCarried((current) =>
        (Object.keys(patch) as (keyof PlayerCarry)[]).every((key) => current[key] === patch[key])
          ? current
          : { ...current, ...patch },
      ),
    [],
  );

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
    setTrackChoice({
      file: fileId,
      ...versionTracks(
        current,
        versions.find((version) => version.id === fileId),
      ),
    });
    setStopping(false);
    await navigate({ search: (previous) => ({ ...previous, file: fileId }), replace: true });
  };

  // Another track or quality (W-4, W-8): a new session of the same file at the same position.
  const switchPlayback = async (change: PlaybackChange, positionMs: number) => {
    const current = playback.data;
    if (!current || switching) return;
    if (!(await endCurrent())) return;
    setStartMs(positionMs);
    if (change.quality !== undefined) setQuality(change.quality);
    setTrackChoice({ file: file ?? null, ...switchTracks(current, change) });
    setGeneration((value) => value + 1);
    setStopping(false);
  };

  /** Another episode in this player: the history keeps one entry for the player (W-9). */
  const playEpisode = (episode: ItemRef) => {
    nextTitleCarry = { itemId: episode.id, carry: carried };
    void navigate({
      to: '/play/$itemId',
      params: { itemId: episode.id },
      search: {},
      replace: true,
    });
  };
  const playNext = (next: ItemRef) => {
    autoNextTarget = next.id;
    playEpisode(next);
  };
  const openPerson = (personId: number) =>
    void navigate({ to: '/people/$personId', params: { personId: String(personId) } });

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
  // The player keeps the season of an episode for 30 s (its episodes' progress); leaving it, the
  // season's page asks again.
  const seasonId = detail.data?.season?.id;
  useEffect(
    () => () => {
      if (seasonId) void queryClient.invalidateQueries({ queryKey: ['item', seasonId] });
    },
    [queryClient, seasonId],
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
        leaving={leaving}
        startPositionMs={startMs}
        versions={versions}
        detail={detail.data}
        infoOpen={infoOpen}
        onToggleInfo={() => setInfoOpen((open) => !open)}
        carried={carried}
        onCarry={carry}
        rememberSeries={
          playback.data.item.kind === 'episode'
            ? { on: rememberSeries, onChange: onRememberSeries }
            : undefined
        }
        switching={switching}
        onSwitchVersion={(fileId, positionMs) => void switchVersion(fileId, positionMs)}
        onSwitch={(change, positionMs) => void switchPlayback(change, positionMs)}
        switchStartedAt={switchStartedAt}
        onNextEpisode={playNext}
        onPlayEpisode={playEpisode}
        onOpenPerson={openPerson}
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
          {/* focusOnMount, not react-aria's autoFocus, which can wait 500 ms for a transition after
              Enter opened the page (11.13). */}
          <AriaButton ref={focusOnMount} variant="secondary" icon="back" onPress={onBack}>
            {t('common.back')}
          </AriaButton>
        </div>
      </div>
    </main>
  );
}
