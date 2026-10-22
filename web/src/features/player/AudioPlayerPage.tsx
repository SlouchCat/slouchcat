import type { ItemDetail, PlaybackInfo, Progress, WatchTime } from '@slouchcat/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl, imageUrl } from '@/api/client';
import { AriaButton, AudioPlayer } from '@/ui/aria';
import type { PlaybackEngine } from './capabilities';
import { useAudioControls } from './useAudioControls';
import { useProgressReporter } from './useProgressReporter';
import { resumeSeconds, streamRevoked, type EndedReason } from './VideoPlayer';

interface AudioPlayerPageProps {
  /** startPlayback's answer with `mediaKind` `audio`. */
  info: PlaybackInfo;
  /** The Range stream (also the enclosure proxy of an episode) or hls.js (multi-part books). */
  engine: Exclude<PlaybackEngine, 'unsupported'>;
  /** performance.now() when the player route opened. */
  openedAt: number;
  onBack: () => void;
  /** Position to start from instead of the server's resume position (a chapter link). */
  startPositionMs?: number;
  /** The title's detail: the podcast of an episode, and the speed if the start had none. */
  detail?: ItemDetail;
}

/**
 * Seconds to start from: an explicit position (a chapter link) as it is, within the title; the
 * resume position without the last seconds of the title (resumeSeconds).
 */
export function startSeconds(ms: number, explicit: boolean, durationMs: number): number {
  if (!explicit) return resumeSeconds(ms, durationMs);
  return Math.max(0, durationMs > 0 ? Math.min(ms, durationMs - 1) : ms) / 1000;
}

/**
 * The player of an audiobook or a podcast episode (M6, H-1, H-3): an <audio> element (hls.js for
 * a multi-part book, the Range stream otherwise) behind the AudioPlayer. The speed starts from the
 * profile's last one for the title and goes to the server with every progress event; the watch
 * time, a stop by the server and the end of the stream show as in the video player.
 */
export function AudioPlayerPage({
  info,
  engine,
  openedAt,
  onBack,
  startPositionMs,
  detail,
}: AudioPlayerPageProps) {
  const { t } = useTranslation();
  // Fixed for the life of this player (one playback session).
  const [resumeMs] = useState(() => startPositionMs ?? info.resumePositionMs);
  const [explicitStart] = useState(() => startPositionMs !== undefined);
  const audio = useRef<HTMLAudioElement>(null);
  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(resumeMs);
  // The duration the server knows; the element's own replaces it once it has loaded.
  const [knownDurationMs] = useState(() => info.mediaFile.durationMs ?? info.item.durationMs ?? 0);
  const [durationMs, setDurationMs] = useState(knownDurationMs);
  const [startMs, setStartMs] = useState<number | null>(null);
  const [failed, setFailed] = useState(false);
  // The server ended this playback (K-5 limit, W-6 admin): the player stops for good.
  const [ended, setEnded] = useState<{ reason: EndedReason; watchTime?: WatchTime } | null>(null);
  const locked = useRef(false);
  const watchTime = useRef<WatchTime | undefined>(info.watchTime);
  const stopHls = useRef<() => void>(() => undefined);

  const endedOnce = useRef(false);
  const endPlayback = useCallback((reason: EndedReason) => {
    if (endedOnce.current) return;
    endedOnce.current = true;
    locked.current = true;
    stopHls.current();
    const element = audio.current;
    if (element && !element.paused) element.pause();
    setEnded((current) => current ?? { reason, watchTime: watchTime.current });
  }, []);

  /** A 401 of the stream: the watch-time limit when the day's minutes were (nearly) used up. */
  const onRevoked = useCallback(() => {
    const remaining = watchTime.current?.remainingMinutes;
    endPlayback(remaining !== undefined && remaining <= 1 ? 'limit' : 'admin');
  }, [endPlayback]);

  const onProgress = useCallback(
    (progress: Progress) => {
      if (!progress.watchTime) return;
      watchTime.current = progress.watchTime;
      if (progress.watchTime.remainingMinutes === 0) endPlayback('limit');
    },
    [endPlayback],
  );

  useProgressReporter({
    itemId: info.item.id,
    mediaFileId: info.mediaFile.id,
    playbackSessionId: info.playbackSessionId,
    fallbackDurationMs: knownDurationMs,
    video: audio,
    enabled: true,
    reportRate: true,
    onProgress,
  });

  // What the listener wants, not what the element reports: re-attaching the element (hls.js after
  // a media error) runs the load algorithm, which clears `paused` without a `pause` event. The
  // element would then read as paused while the player still shows "Pause", and the reloaded
  // source would play on even after a pause.
  const wantPlaying = useRef(true);

  const togglePlay = useCallback(() => {
    const element = audio.current;
    if (!element || locked.current) return;
    wantPlaying.current = !wantPlaying.current;
    if (wantPlaying.current) element.play().catch(() => setPlaying(false));
    else {
      setPlaying(false);
      element.pause();
    }
  }, []);

  const seekTo = useCallback((ms: number) => {
    const element = audio.current;
    if (!element || locked.current) return;
    const end = Number.isFinite(element.duration) ? element.duration : Infinity;
    element.currentTime = Math.min(end, Math.max(0, ms / 1000));
    setCurrentMs(element.currentTime * 1000);
  }, []);

  const controls = useAudioControls({
    media: audio,
    chapters: info.chapters,
    durationMs,
    positionMs: currentMs,
    positionOf: () => (audio.current?.currentTime ?? 0) * 1000,
    // The start answer's speed; without one, the title's detail (fetched after the start).
    initialRate: info.item.progress?.playbackRate ?? detail?.progress?.playbackRate ?? 1,
    active: ended === null,
    seekTo,
    togglePlay,
    onBack,
  });
  // hls.js (Chromium, Firefox) for a multi-part book: loaded as its own chunk, from the resume
  // position, as in the video player.
  useEffect(() => {
    const element = audio.current;
    if (engine !== 'hlsjs' || !element) return;
    let cancelled = false;
    let destroy = () => {};
    import('hls.js/light')
      .then(({ default: Hls }) => {
        if (cancelled) return;
        const hls = new Hls({
          startPosition: startSeconds(resumeMs, explicitStart, knownDurationMs),
        });
        let mediaRecovered = false;
        let networkRecovered = false;
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (data.response?.code === 401) {
            onRevoked();
            return;
          }
          if (!data.fatal) return;
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !mediaRecovered) {
            mediaRecovered = true;
            hls.recoverMediaError();
          } else if (data.type === Hls.ErrorTypes.NETWORK_ERROR && !networkRecovered) {
            networkRecovered = true;
            hls.startLoad();
          } else {
            setFailed(true);
          }
        });
        hls.loadSource(apiUrl(info.streamUrl));
        hls.attachMedia(element);
        destroy = () => hls.destroy();
        stopHls.current = () => hls.stopLoad();
      })
      .catch(() => setFailed(true));
    return () => {
      cancelled = true;
      stopHls.current = () => undefined;
      destroy();
    };
  }, [engine, explicitStart, info, knownDurationMs, onRevoked, resumeMs]);

  // Media element events.
  useEffect(() => {
    const element = audio.current;
    if (!element) return;
    const onLoadedMetadata = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
      const resume = startSeconds(resumeMs, explicitStart, element.duration * 1000);
      // hls.js already started at the resume position.
      if (resume > 0 && Math.abs(element.currentTime - resume) > 1) element.currentTime = resume;
      if (!locked.current && wantPlaying.current) element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      wantPlaying.current = true;
      setPlaying(true);
      setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
    };
    // A pause the element reports (the listener, the sleep timer, the end of the title) is a wish;
    // the silent one of the load algorithm is not and fires no event.
    const onPause = () => {
      wantPlaying.current = false;
      setPlaying(false);
    };
    const onTime = () => setCurrentMs(element.currentTime * 1000);
    const onDuration = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
    };
    // With hls.js the element's errors reach hls.js as well, which reports them (and recovers).
    // A direct stream the server closed answers 401 to the element's next range request.
    const onError = () => {
      if (engine === 'hlsjs') return;
      void streamRevoked(info.streamUrl).then((revoked) => {
        if (revoked) onRevoked();
        else setFailed(true);
      });
    };
    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('pause', onPause);
    element.addEventListener('ended', onPause);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('durationchange', onDuration);
    element.addEventListener('error', onError);
    return () => {
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('ended', onPause);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('durationchange', onDuration);
      element.removeEventListener('error', onError);
    };
  }, [engine, explicitStart, info, onRevoked, openedAt, resumeMs]);

  const { item } = info;
  const limit = ended?.watchTime?.limitMinutes;
  const alert = ended ? (
    <>
      <p>{ended.reason === 'limit' ? t('player.watchLimit.title') : t('player.ended.title')}</p>
      <p>
        {ended.reason === 'admin'
          ? t('player.ended.body')
          : limit !== undefined
            ? t('player.watchLimit.body', { count: limit })
            : t('player.watchLimit.bodyUnknown')}
      </p>
      <div>
        <AriaButton variant="secondary" icon="back" onPress={onBack} autoFocus>
          {t('common.back')}
        </AriaButton>
      </div>
    </>
  ) : failed ? (
    <>
      <p>{t('player.error.title')}</p>
      <p>{t('player.error.body')}</p>
    </>
  ) : null;

  return (
    <AudioPlayer
      {...controls}
      title={item.title}
      byline={detail?.podcast?.title ?? item.author ?? detail?.author}
      coverUrl={imageUrl(item.images.posterLarge ?? item.images.poster)}
      accent={item.accent}
      chapters={info.chapters}
      currentMs={currentMs}
      durationMs={durationMs}
      playing={playing}
      onTogglePlay={togglePlay}
      onSeek={seekTo}
      onBack={onBack}
      isDisabled={ended !== null}
      alert={alert}
      data-start-ms={startMs ?? undefined}
      data-engine={engine}
      data-media-file-id={info.mediaFile.id}
      data-playback-session-id={info.playbackSessionId}
      data-ended-reason={ended?.reason}
      data-rate={controls.rate}
    >
      {/* Spoken word without text tracks: nothing to caption. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <audio
        ref={audio}
        src={engine === 'hlsjs' ? undefined : apiUrl(info.streamUrl)}
        preload="auto"
      />
    </AudioPlayer>
  );
}
