import type { PlaybackInfo } from '@slouchcat/api-client';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { apiUrl } from '@/api/client';
import { formatPlayerTime } from '@/lib/format';
import { AriaButton } from '@/ui/aria';
import { SeekBar } from './SeekBar';
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
  /** performance.now() when the player route opened. */
  openedAt: number;
  onBack: () => void;
}

const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

function bufferedRanges(video: HTMLVideoElement): [number, number][] {
  const ranges: [number, number][] = [];
  for (let index = 0; index < video.buffered.length; index += 1) {
    ranges.push([video.buffered.start(index) * 1000, video.buffered.end(index) * 1000]);
  }
  return ranges;
}

export function VideoPlayer({ info, openedAt, onBack }: VideoPlayerProps) {
  const { t } = useTranslation();
  const video = useRef<HTMLVideoElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const hideTimer = useRef<number | undefined>(undefined);
  const osdTimer = useRef<number | undefined>(undefined);

  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(info.resumePositionMs);
  const [durationMs, setDurationMs] = useState(info.mediaFile.durationMs ?? 0);
  const [buffered, setBuffered] = useState<[number, number][]>([]);
  const [fullscreen, setFullscreen] = useState(false);
  const [idle, setIdle] = useState(false);
  const [osd, setOsd] = useState<string | null>(null);
  const [startMs, setStartMs] = useState<number | null>(null);
  const [failed, setFailed] = useState(false);

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
    if (!element) return;
    if (element.paused || element.ended) element.play().catch(() => setPlaying(false));
    else element.pause();
  }, []);

  const seekBy = useCallback((seconds: number) => {
    const element = video.current;
    if (!element) return;
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
        case 'Escape':
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
  }, [changeVolume, onBack, poke, seekBy, toggleFullscreen, togglePlay]);

  // Media element events.
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    const onLoadedMetadata = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
      const resume = info.resumePositionMs;
      if (resume > 0 && resume < element.duration * 1000 - RESUME_TAIL_MS) {
        element.currentTime = resume / 1000;
      }
      element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      setPlaying(true);
      setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
    };
    const onPause = () => setPlaying(false);
    const onTime = () => setCurrentMs(element.currentTime * 1000);
    const onProgress = () => setBuffered(bufferedRanges(element));
    const onDuration = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
    };
    const onError = () => setFailed(true);

    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('pause', onPause);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('progress', onProgress);
    element.addEventListener('durationchange', onDuration);
    element.addEventListener('error', onError);
    return () => {
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('progress', onProgress);
      element.removeEventListener('durationchange', onDuration);
      element.removeEventListener('error', onError);
    };
  }, [info.resumePositionMs, openedAt]);

  const elapsed = formatPlayerTime(currentMs, durationMs);
  const total = formatPlayerTime(durationMs, durationMs);
  const timeText = t('player.time', { elapsed, total });
  const hidden = idle && playing;

  return (
    <main
      ref={container}
      className={styles.player}
      aria-label={t('player.region')}
      data-start-ms={startMs ?? undefined}
      data-controls={hidden ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
    >
      {/* Subtitle tracks arrive with track selection (W-6, M4); M1a plays the main streams. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={video}
        className={styles.video}
        src={apiUrl(info.streamUrl)}
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

      {osd && (
        <div className={styles.osd} role="status">
          {osd}
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
            if (!element) return;
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
        </div>
      </div>
    </main>
  );
}
