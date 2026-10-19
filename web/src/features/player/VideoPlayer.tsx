import type { MediaFileInfo, PlaybackInfo } from '@slouchcat/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl } from '@/api/client';
import { formatPlayerTime } from '@/lib/format';
import { AriaButton, MenuButton } from '@/ui/aria';
import type { PlaybackEngine } from './capabilities';
import { SeekBar } from './SeekBar';
import { StreamInfo } from './StreamInfo';
import { useProgressReporter } from './useProgressReporter';
import styles from './Player.module.css';

/** Controls fade out after this long without input while the video plays. */
export const CONTROLS_HIDE_MS = 3000;
const SEEK_STEP_S = 10;
const VOLUME_STEP = 0.1;
/** Resume only when the position is not within the last seconds of the file. */
const RESUME_TAIL_MS = 5000;

interface VideoPlayerProps {
  info: PlaybackInfo;
  /** Range stream or native HLS (the element loads streamUrl itself) or hls.js (MSE). */
  engine: Exclude<PlaybackEngine, 'unsupported'>;
  /** performance.now() when the player route opened. */
  openedAt: number;
  onBack: () => void;
  /** Position to start from instead of the server's resume position (after a version switch). */
  startPositionMs?: number;
  /** Versions of the title (B-5); the version menu shows with two or more. */
  versions?: readonly MediaFileInfo[];
  /** A switch to another version is on its way. */
  switching?: boolean;
  onSwitchVersion?: (mediaFileId: string, positionMs: number) => void;
  /** The "Stream-Info" overlay (key i). */
  infoOpen?: boolean;
  onToggleInfo?: () => void;
}

const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

/** Keys inside the version menu belong to the menu (arrows, Enter, Escape). */
const inMenu = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('[role="menu"], [role="menuitemradio"]') !== null;

function bufferedRanges(video: HTMLVideoElement): [number, number][] {
  const ranges: [number, number][] = [];
  for (let index = 0; index < video.buffered.length; index += 1) {
    ranges.push([video.buffered.start(index) * 1000, video.buffered.end(index) * 1000]);
  }
  return ranges;
}

/** Where playback starts: the resume position unless it lies in the last seconds of the file. */
function resumeSeconds(resume: number, durationMs: number): number {
  if (resume <= 0 || (durationMs > 0 && resume >= durationMs - RESUME_TAIL_MS)) return 0;
  return resume / 1000;
}

export function VideoPlayer({
  info,
  engine,
  openedAt,
  onBack,
  startPositionMs,
  versions = [],
  switching = false,
  onSwitchVersion,
  infoOpen = false,
  onToggleInfo,
}: VideoPlayerProps) {
  const { t } = useTranslation();
  // Fixed for the life of this player (one playback session).
  const [resumeMs] = useState(() => startPositionMs ?? info.resumePositionMs);
  const video = useRef<HTMLVideoElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const hideTimer = useRef<number | undefined>(undefined);
  const osdTimer = useRef<number | undefined>(undefined);

  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(resumeMs);
  const [durationMs, setDurationMs] = useState(info.mediaFile.durationMs ?? 0);
  const [buffered, setBuffered] = useState<[number, number][]>([]);
  const [fullscreen, setFullscreen] = useState(false);
  const [idle, setIdle] = useState(false);
  const [osd, setOsd] = useState<string | null>(null);
  const [startMs, setStartMs] = useState<number | null>(null);
  // Last seek: from the element starting to seek until it plays again (e2e measurement, W-3).
  const [seek, setSeek] = useState<{ ms: number; count: number } | null>(null);
  const [failed, setFailed] = useState(false);
  // The version menu keeps the controls on screen while it is open.
  const [menuOpen, setMenuOpen] = useState(false);
  // The menu renders into the player, so it stays visible in full screen.
  const [portal, setPortal] = useState<HTMLElement | null>(null);
  const containerRef = useCallback((node: HTMLDivElement | null) => {
    container.current = node;
    setPortal(node);
  }, []);
  // During a version switch the old session is ended already: playing or seeking it would fail.
  const locked = useRef(switching);
  useEffect(() => {
    locked.current = switching;
  }, [switching]);

  useProgressReporter({
    itemId: info.item.id,
    mediaFileId: info.mediaFile.id,
    fallbackDurationMs: info.mediaFile.durationMs ?? 0,
    video,
    enabled: true,
  });

  // Controls: visible on any input and while paused, hidden after 3 s without input while playing.
  const scheduleHide = useCallback(() => {
    window.clearTimeout(hideTimer.current);
    hideTimer.current = window.setTimeout(() => setIdle(true), CONTROLS_HIDE_MS);
  }, []);
  const poke = useCallback(() => {
    setIdle(false);
    scheduleHide();
  }, [scheduleHide]);

  useEffect(() => {
    if (playing) scheduleHide();
    else window.clearTimeout(hideTimer.current);
  }, [playing, scheduleHide]);

  useEffect(
    () => () => {
      window.clearTimeout(hideTimer.current);
      window.clearTimeout(osdTimer.current);
    },
    [],
  );

  const showOsd = (text: string) => {
    setOsd(text);
    window.clearTimeout(osdTimer.current);
    osdTimer.current = window.setTimeout(() => setOsd(null), 1200);
  };

  const togglePlay = useCallback(() => {
    const element = video.current;
    if (!element || locked.current) return;
    if (element.paused || element.ended) element.play().catch(() => setPlaying(false));
    else element.pause();
  }, []);

  const seekBy = useCallback((seconds: number) => {
    const element = video.current;
    if (!element || locked.current) return;
    const end = Number.isFinite(element.duration) ? element.duration : Infinity;
    element.currentTime = Math.min(end, Math.max(0, element.currentTime + seconds));
    setCurrentMs(element.currentTime * 1000);
  }, []);

  const changeVolume = useCallback(
    (delta: number) => {
      const element = video.current;
      if (!element) return;
      const next = Math.round(Math.min(1, Math.max(0, element.volume + delta)) * 10) / 10;
      element.volume = next;
      element.muted = next === 0;
      showOsd(t('player.volume', { percent: Math.round(next * 100) }));
    },
    [t],
  );

  const toggleFullscreen = useCallback(() => {
    const node = container.current;
    if (!node) return;
    if (document.fullscreenElement) void document.exitFullscreen();
    else if (typeof node.requestFullscreen === 'function') void node.requestFullscreen();
  }, []);

  useEffect(() => {
    const onChange = () => setFullscreen(document.fullscreenElement !== null);
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);

  // Key map (W-1, C-4): Space/Enter play-pause, ←/→ ±10 s, ↑/↓ volume, F full screen,
  // Escape/Backspace back to the detail page.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      poke();
      if (inMenu(event.target)) return;
      switch (event.key) {
        case ' ':
        case 'Enter':
          // Focused buttons activate themselves; everywhere else the key toggles playback.
          if (isActivatable(event.target)) return;
          event.preventDefault();
          togglePlay();
          return;
        case 'k':
          togglePlay();
          return;
        case 'ArrowLeft':
          event.preventDefault();
          seekBy(-SEEK_STEP_S);
          return;
        case 'ArrowRight':
          event.preventDefault();
          seekBy(SEEK_STEP_S);
          return;
        case 'ArrowUp':
          event.preventDefault();
          changeVolume(VOLUME_STEP);
          return;
        case 'ArrowDown':
          event.preventDefault();
          changeVolume(-VOLUME_STEP);
          return;
        case 'f':
        case 'F':
          event.preventDefault();
          toggleFullscreen();
          return;
        case 'i':
        case 'I':
          event.preventDefault();
          onToggleInfo?.();
          return;
        case 'Escape':
          if (infoOpen) {
            event.preventDefault();
            onToggleInfo?.();
            return;
          }
          if (document.fullscreenElement) return;
          event.preventDefault();
          onBack();
          return;
        case 'Backspace':
          if (document.fullscreenElement) return;
          event.preventDefault();
          onBack();
          return;
        default:
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [changeVolume, infoOpen, onBack, onToggleInfo, poke, seekBy, toggleFullscreen, togglePlay]);

  // hls.js (Chromium, Firefox): loaded as its own chunk only for HLS playback (ADR 0004). It starts
  // loading at the resume position, so a transcode does not first encode the beginning.
  useEffect(() => {
    const element = video.current;
    if (engine !== 'hlsjs' || !element) return;
    let cancelled = false;
    let destroy = () => {};
    import('hls.js/light')
      .then(({ default: Hls }) => {
        if (cancelled) return;
        const hls = new Hls({
          startPosition: resumeSeconds(resumeMs, info.mediaFile.durationMs ?? 0),
          // A seek far ahead restarts the encoder; the first byte of that segment may take a while.
          fragLoadPolicy: {
            default: {
              maxTimeToFirstByteMs: 20_000,
              maxLoadTimeMs: 60_000,
              timeoutRetry: { maxNumRetry: 2, retryDelayMs: 0, maxRetryDelayMs: 0 },
              errorRetry: { maxNumRetry: 3, retryDelayMs: 500, maxRetryDelayMs: 4000 },
            },
          },
        });
        // A fatal error gets one recovery attempt per kind before the player gives up: a media
        // error (decode, buffer append) re-creates the media source, a network error restarts
        // loading at the current position.
        let mediaRecovered = false;
        let networkRecovered = false;
        hls.on(Hls.Events.ERROR, (_event, data) => {
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
      })
      .catch(() => setFailed(true));
    return () => {
      cancelled = true;
      destroy();
    };
  }, [engine, info, resumeMs]);

  // Media element events.
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    let seekStarted: number | null = null;
    let seeked = false;
    const finishSeek = () => {
      if (seekStarted === null) return;
      const ms = Math.round(performance.now() - seekStarted);
      seekStarted = null;
      setSeek((value) => ({ ms, count: (value?.count ?? 0) + 1 }));
    };
    const onLoadedMetadata = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
      const resume = resumeSeconds(resumeMs, element.duration * 1000);
      // hls.js already started at the resume position.
      if (resume > 0 && Math.abs(element.currentTime - resume) > 1) element.currentTime = resume;
      element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      setPlaying(true);
      setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
      finishSeek();
    };
    const onSeeking = () => {
      seekStarted ??= performance.now();
      seeked = false;
    };
    const onSeeked = () => {
      seeked = true;
    };
    const onPause = () => setPlaying(false);
    const onTime = () => {
      setCurrentMs(element.currentTime * 1000);
      // Seeking within the buffer fires no new `playing`: the first tick after `seeked` counts.
      if (seeked && !element.paused) finishSeek();
    };
    const onProgress = () => setBuffered(bufferedRanges(element));
    const onDuration = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
    };
    // With hls.js the element's errors reach hls.js as well, which reports them (and recovers).
    const onError = () => {
      if (engine !== 'hlsjs') setFailed(true);
    };

    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('seeking', onSeeking);
    element.addEventListener('seeked', onSeeked);
    element.addEventListener('pause', onPause);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('progress', onProgress);
    element.addEventListener('durationchange', onDuration);
    element.addEventListener('error', onError);
    return () => {
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('seeking', onSeeking);
      element.removeEventListener('seeked', onSeeked);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('progress', onProgress);
      element.removeEventListener('durationchange', onDuration);
      element.removeEventListener('error', onError);
    };
  }, [engine, info, openedAt, resumeMs]);

  // Another version: pause here and hand the position over to the next session.
  const switchTo = (mediaFileId: string) => {
    const element = video.current;
    if (!element || !onSwitchVersion) return;
    element.pause();
    onSwitchVersion(mediaFileId, Math.round(element.currentTime * 1000));
  };
  const currentVersion = info.mediaFile.versionLabel ?? info.mediaFile.container.toUpperCase();

  const elapsed = formatPlayerTime(currentMs, durationMs);
  const total = formatPlayerTime(durationMs, durationMs);
  const timeText = t('player.time', { elapsed, total });
  const hidden = idle && playing && !menuOpen;

  return (
    <main
      ref={containerRef}
      className={styles.player}
      aria-label={t('player.region')}
      data-start-ms={startMs ?? undefined}
      data-seek-ms={seek?.ms}
      data-seek-count={seek?.count}
      data-engine={engine}
      data-media-file-id={info.mediaFile.id}
      data-controls={hidden && !infoOpen ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
    >
      {/* Subtitle tracks arrive with track selection (W-6, M4); M1b plays the main streams. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={video}
        className={styles.video}
        src={engine === 'hlsjs' ? undefined : apiUrl(info.streamUrl)}
        preload="auto"
        playsInline
        onPointerUp={(event) => {
          if (event.button === 0) togglePlay();
        }}
        onDoubleClick={toggleFullscreen}
      />

      {failed && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.error.title')}</p>
          <p>{t('player.error.body')}</p>
        </div>
      )}

      {infoOpen && <StreamInfo info={info} />}

      {(switching || osd) && (
        <div className={styles.osd} role="status">
          {switching ? t('player.switching') : osd}
        </div>
      )}

      <div className={styles.top}>
        <AriaButton
          variant="ghost"
          icon="back"
          iconOnly
          aria-label={t('player.back')}
          onPress={onBack}
        />
        <h1 className={styles.title}>{info.item.title}</h1>
      </div>

      <div className={styles.bottom}>
        <SeekBar
          label={t('player.seek')}
          valueText={timeText}
          currentMs={currentMs}
          durationMs={durationMs}
          buffered={buffered}
          onSeek={(ms) => {
            const element = video.current;
            if (!element || locked.current) return;
            element.currentTime = ms / 1000;
            setCurrentMs(ms);
          }}
        />
        <div className={styles.bar}>
          <AriaButton
            variant="ghost"
            icon={playing ? 'pause' : 'play'}
            iconOnly
            aria-label={playing ? t('player.pause') : t('player.play')}
            onPress={togglePlay}
            isDisabled={switching}
            autoFocus
          />
          <span className={styles.time}>
            <span aria-hidden="true">{elapsed}</span>
            <span aria-hidden="true" className={styles.timeTotal}>
              {total}
            </span>
          </span>
          <span className={styles.spacer} />
          <AriaButton
            variant="ghost"
            icon={fullscreen ? 'fullscreenExit' : 'fullscreen'}
            iconOnly
            aria-label={fullscreen ? t('player.exitFullscreen') : t('player.fullscreen')}
            onPress={toggleFullscreen}
          />
          {onToggleInfo && (
            <AriaButton
              variant="ghost"
              icon="info"
              iconOnly
              aria-label={t('player.info')}
              aria-pressed={infoOpen}
              onPress={onToggleInfo}
            />
          )}
          {versions.length > 1 && onSwitchVersion && (
            <MenuButton
              aria-label={t('player.version')}
              buttonLabel={t('player.versionButton', { label: currentVersion })}
              label={currentVersion}
              options={versions.map((version, index) => ({
                id: version.id,
                label: version.versionLabel ?? String(index + 1),
              }))}
              selectedKey={info.mediaFile.id}
              onSelect={(id) => {
                if (!switching) switchTo(id);
              }}
              onOpenChange={setMenuOpen}
              portalContainer={portal ?? undefined}
            />
          )}
        </div>
      </div>
    </main>
  );
}
