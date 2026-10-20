import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { languageName, trackLabel } from '@/features/player/labels';
import { SeekBar } from '@/features/player/SeekBar';
import { formatPlayerTime } from '@/lib/format';
import { AriaButton, MenuButton } from '@/ui/aria';
import { offlineLoader } from './loader';
import { readFile, subtitleFile } from './opfs';
import { recordPlayback, type OfflinePlayback, type PlaybackSample } from './playback';
import { buildPlaylist } from './playlist';
import type { DownloadManifest, StoredWatchTime } from './types';
import { remainingMs } from './watchTime';
import styles from '@/features/player/Player.module.css';

// The player of a download (design 4): hls.js plays a VOD playlist built from the manifest
// through a loader that reads the byte ranges from OPFS (spike decision AN-20), so seeking needs
// no network and no search through the fragments. Without MediaSource the whole file plays from a
// blob URL. Subtitles are <track> elements of the stored WebVTT files with a local menu. Progress
// goes into the event queue (O-5); the offline watch time stops playback at 0 (AN-18).

/** Interval of `playing` events, as online (useProgressReporter). */
export const OFFLINE_PROGRESS_MS = 10_000;
const CONTROLS_HIDE_MS = 3000;
const SEEK_STEP_S = 10;
const VOLUME_STEP = 0.1;
const RESUME_TAIL_MS = 5000;
const WATCH_CHECK_MS = 1000;

export type OfflineEngine = 'mse' | 'blob';

interface StoredSubtitle {
  track: DownloadManifest['subtitles'][number]['track'];
  url: string;
}

interface OfflinePlayerProps {
  title: string;
  manifest: DownloadManifest;
  media: File;
  playback: OfflinePlayback;
  /** Stored subtitle track ids (StoredDownload.subtitlesStored). */
  subtitleIds: readonly number[];
  /** Position to resume from (source timeline). */
  resumeMs: number;
  /** Watch time of the profile; undefined without a daily limit. */
  watch: StoredWatchTime | undefined;
  /** Server time minus device time of the last sync. */
  serverOffsetMs: number;
  onBack: () => void;
}

const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

const inMenu = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  target.closest('[role="menu"], [role="menuitemradio"], [role="menuitem"]') !== null;

/**
 * Seconds the file's timeline is ahead of the source timeline in the blob fallback: download
 * fragments keep the HLS timeline (stream A: media time = source time + 10 s). hls.js maps them
 * back itself; a media element playing the file directly starts there.
 */
export function timelineOffset(
  video: Pick<HTMLVideoElement, 'buffered' | 'seekable' | 'duration'>,
  durationMs: number,
): number {
  const buffered = video.buffered.length > 0 ? video.buffered.start(0) : 0;
  const seekable = video.seekable.length > 0 ? video.seekable.start(0) : 0;
  const start = Math.max(buffered, seekable);
  if (start > 0.25) return start;
  const extra = Number.isFinite(video.duration) ? video.duration - durationMs / 1000 : 0;
  return extra > 0.5 ? extra : 0;
}

export function OfflinePlayer({
  title,
  manifest,
  media,
  playback,
  subtitleIds,
  resumeMs,
  watch,
  serverOffsetMs,
  onBack,
}: OfflinePlayerProps) {
  const { t, i18n } = useTranslation();
  const video = useRef<HTMLVideoElement>(null);
  const hideTimer = useRef<number | undefined>(undefined);
  const osdTimer = useRef<number | undefined>(undefined);
  const [openedAt] = useState(() => performance.now());
  const [engine, setEngine] = useState<OfflineEngine | null>(null);
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(resumeMs);
  const durationMs = manifest.durationMs;
  const [buffered, setBuffered] = useState<[number, number][]>([]);
  const [idle, setIdle] = useState(false);
  const [osd, setOsd] = useState<string | null>(null);
  const [startMs, setStartMs] = useState<number | null>(null);
  const [seek, setSeek] = useState<{ ms: number; count: number } | null>(null);
  const [failed, setFailed] = useState(false);
  // Used up before the start: the player never plays (AN-18).
  const [usedUp] = useState(
    () => watch !== undefined && remainingMs(watch, Date.now() + serverOffsetMs) <= 0,
  );
  const [limitReached, setLimitReached] = useState(usedUp);
  const [subtitles, setSubtitles] = useState<StoredSubtitle[]>([]);
  const [subtitle, setSubtitle] = useState<number | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [fullscreen, setFullscreen] = useState(() => Boolean(document.fullscreenElement));
  const [portal, setPortal] = useState<HTMLElement | null>(null);
  // Seconds the element's timeline is ahead of the source (blob fallback only).
  const offset = useRef(0);
  const watchRef = useRef(watch);
  const locked = useRef(usedUp);

  // ---------- watch time (AN-18) ----------

  const playedSince = useRef<number | null>(null);
  const unrecordedMs = useRef(0);
  const stopClock = useCallback(() => {
    if (playedSince.current !== null) {
      unrecordedMs.current += performance.now() - playedSince.current;
      playedSince.current = null;
    }
  }, []);
  /** Milliseconds the profile may still play: the stored rest minus what is not booked yet. */
  const remaining = useCallback(() => {
    const current = watchRef.current;
    if (!current) return Infinity;
    const running = playedSince.current === null ? 0 : performance.now() - playedSince.current;
    return remainingMs(current, Date.now() + serverOffsetMs) - unrecordedMs.current - running;
  }, [serverOffsetMs]);

  // ---------- progress events (O-5) ----------

  const seq = useRef(1);
  const sendEvent = useCallback(
    (state: PlaybackSample['state']) => {
      const element = video.current;
      if (!element || startMs === null) return;
      const running = playedSince.current !== null;
      stopClock();
      const played = Math.round(unrecordedMs.current);
      unrecordedMs.current = 0;
      if (running) playedSince.current = performance.now();
      const sample: PlaybackSample = {
        positionMs: Math.max(0, Math.round((element.currentTime - offset.current) * 1000)),
        durationMs: Math.max(1, durationMs),
        playedMs: played,
        seq: seq.current,
        state,
      };
      seq.current += 1;
      void recordPlayback(playback, sample).then(
        (next) => {
          if (next) watchRef.current = next;
        },
        () => undefined,
      );
    },
    [durationMs, playback, startMs, stopClock],
  );
  const sendLater = useRef(sendEvent);
  useEffect(() => {
    sendLater.current = sendEvent;
  });

  /** The day's watch time is used up: stop for good, with the hint. */
  const stopForLimit = useCallback(() => {
    const element = video.current;
    locked.current = true;
    setLimitReached(true);
    if (element && !element.paused) element.pause();
  }, []);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (!locked.current && remaining() <= 0) stopForLimit();
    }, WATCH_CHECK_MS);
    return () => window.clearInterval(timer);
  }, [remaining, stopForLimit]);

  // ---------- engine: hls.js over the OPFS loader, else a blob URL ----------

  useEffect(() => {
    const element = video.current;
    if (!element) return;
    let cancelled = false;
    let destroy = () => {};
    import('hls.js/light')
      .then(({ default: Hls }) => {
        if (cancelled) return;
        if (!Hls.isSupported()) {
          const url = URL.createObjectURL(media);
          destroy = () => URL.revokeObjectURL(url);
          setBlobUrl(url);
          setEngine('blob');
          return;
        }
        const resume = resumeMs > 0 && resumeMs < durationMs - RESUME_TAIL_MS ? resumeMs / 1000 : 0;
        const hls = new Hls({
          loader: offlineLoader({ playlist: buildPlaylist(manifest), media }),
          startPosition: resume,
        });
        let recovered = false;
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (!data.fatal) return;
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !recovered) {
            recovered = true;
            hls.recoverMediaError();
          } else {
            setFailed(true);
          }
        });
        // The URL only names the playlist for hls.js; the loader never fetches it.
        hls.loadSource(`${window.location.origin}/offline/media/${playback.downloadId}/index.m3u8`);
        hls.attachMedia(element);
        destroy = () => hls.destroy();
        setEngine('mse');
      })
      .catch(() => setFailed(true));
    return () => {
      cancelled = true;
      destroy();
    };
  }, [durationMs, manifest, media, playback.downloadId, resumeMs]);

  // ---------- subtitles from OPFS ----------

  useEffect(() => {
    let active = true;
    const urls: string[] = [];
    void (async () => {
      const list: StoredSubtitle[] = [];
      for (const entry of manifest.subtitles) {
        if (!subtitleIds.includes(entry.track.id)) continue;
        const file = await readFile(playback.downloadId, subtitleFile(entry.track.id)).catch(
          () => null,
        );
        if (!file) continue;
        const url = URL.createObjectURL(file);
        urls.push(url);
        list.push({ track: entry.track, url });
      }
      if (!active) return;
      setSubtitles(list);
      setSubtitle(list.find((entry) => entry.track.default)?.track.id ?? null);
    })();
    return () => {
      active = false;
      for (const url of urls) URL.revokeObjectURL(url);
    };
  }, [manifest.subtitles, playback.downloadId, subtitleIds]);

  // The chosen subtitle shows, the others are off (the menu is local, no new session).
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    const tracks = element.textTracks;
    for (let index = 0; index < tracks.length; index += 1) {
      const track = tracks[index];
      if (track) track.mode = track.id === String(subtitle) ? 'showing' : 'disabled';
    }
  }, [subtitle, subtitles]);

  // ---------- controls ----------

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
  const showOsd = useCallback((text: string) => {
    setOsd(text);
    window.clearTimeout(osdTimer.current);
    osdTimer.current = window.setTimeout(() => setOsd(null), 1200);
  }, []);

  const togglePlay = useCallback(() => {
    const element = video.current;
    if (!element || locked.current) return;
    if (element.paused || element.ended) element.play().catch(() => setPlaying(false));
    else element.pause();
  }, []);
  const seekTo = useCallback((ms: number) => {
    const element = video.current;
    if (!element || locked.current) return;
    const end = Number.isFinite(element.duration) ? element.duration : Infinity;
    element.currentTime = Math.min(end, Math.max(offset.current, ms / 1000 + offset.current));
    setCurrentMs((element.currentTime - offset.current) * 1000);
  }, []);
  const seekBy = useCallback(
    (seconds: number) => {
      const element = video.current;
      if (element) seekTo((element.currentTime - offset.current + seconds) * 1000);
    },
    [seekTo],
  );
  const changeVolume = useCallback(
    (delta: number) => {
      const element = video.current;
      if (!element) return;
      const next = Math.round(Math.min(1, Math.max(0, element.volume + delta)) * 10) / 10;
      element.volume = next;
      element.muted = next === 0;
      showOsd(t('player.volume', { percent: Math.round(next * 100) }));
    },
    [showOsd, t],
  );
  const toggleFullscreen = useCallback(() => {
    const root = document.documentElement;
    if (document.fullscreenElement) void document.exitFullscreen();
    else if (typeof root.requestFullscreen === 'function') void root.requestFullscreen();
  }, []);
  useEffect(() => {
    const onChange = () => setFullscreen(Boolean(document.fullscreenElement));
    document.addEventListener('fullscreenchange', onChange);
    return () => document.removeEventListener('fullscreenchange', onChange);
  }, []);
  useEffect(
    () => () => {
      if (document.fullscreenElement) void document.exitFullscreen().catch(() => undefined);
    },
    [],
  );

  // Keys as in the main player: Space/Enter/K play-pause, ←/→ ±10 s, ↑/↓ volume, F full screen,
  // T subtitles, Escape/Backspace back.
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      poke();
      if (inMenu(event.target)) {
        if (event.key === 'Backspace') {
          event.preventDefault();
          setMenuOpen(false);
        }
        return;
      }
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      switch (key) {
        case ' ':
        case 'Enter':
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
          event.preventDefault();
          toggleFullscreen();
          return;
        case 't':
          if (subtitles.length === 0 || limitReached) return;
          event.preventDefault();
          setMenuOpen(true);
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
  }, [
    changeVolume,
    limitReached,
    onBack,
    poke,
    seekBy,
    subtitles.length,
    toggleFullscreen,
    togglePlay,
  ]);

  // ---------- media element events ----------

  useEffect(() => {
    const element = video.current;
    if (!element || engine === null) return;
    let seekStarted: number | null = null;
    let seeked = false;
    const finishSeek = () => {
      if (seekStarted === null) return;
      const ms = Math.round(performance.now() - seekStarted);
      seekStarted = null;
      setSeek((value) => ({ ms, count: (value?.count ?? 0) + 1 }));
    };
    // Blob fallback: the cues are on the source timeline, the file's timeline is ahead. A track
    // loads its cues when it is switched on, so every load shifts what is not shifted yet.
    const shiftCues = () => {
      if (engine !== 'blob') return;
      const shift = offset.current;
      for (const node of element.querySelectorAll('track')) {
        // Every browser has `track`; a DOM without text tracks (tests) has none.
        const cues = (node.track as TextTrack | undefined)?.cues;
        const applied = Number(node.dataset.shift ?? 0);
        if (!cues || cues.length === 0 || applied === shift) continue;
        for (let index = 0; index < cues.length; index += 1) {
          const cue = cues[index];
          if (!cue) continue;
          cue.startTime += shift - applied;
          cue.endTime += shift - applied;
        }
        node.dataset.shift = String(shift);
      }
    };
    const onLoadedMetadata = () => {
      if (engine === 'blob') {
        offset.current = timelineOffset(element, durationMs);
        const resume = resumeMs > 0 && resumeMs < durationMs - RESUME_TAIL_MS ? resumeMs / 1000 : 0;
        element.currentTime = offset.current + resume;
        shiftCues();
      }
      if (!locked.current) element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      setPlaying(true);
      setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
      playedSince.current ??= performance.now();
      finishSeek();
    };
    const onWaiting = () => stopClock();
    const onSeeking = () => {
      seekStarted ??= performance.now();
      seeked = false;
    };
    const onSeeked = () => {
      seeked = true;
    };
    const onPause = () => {
      setPlaying(false);
      if (element.ended) return;
      stopClock();
      sendLater.current('paused');
    };
    const onEnded = () => {
      setPlaying(false);
      stopClock();
      sendLater.current('ended');
    };
    const onTime = () => {
      setCurrentMs(Math.max(0, (element.currentTime - offset.current) * 1000));
      if (seeked && !element.paused) finishSeek();
    };
    const onProgress = () => {
      const ranges: [number, number][] = [];
      for (let index = 0; index < element.buffered.length; index += 1) {
        ranges.push([
          (element.buffered.start(index) - offset.current) * 1000,
          (element.buffered.end(index) - offset.current) * 1000,
        ]);
      }
      setBuffered(ranges);
    };
    const onError = () => {
      if (engine === 'blob') setFailed(true);
    };
    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('waiting', onWaiting);
    element.addEventListener('seeking', onSeeking);
    element.addEventListener('seeked', onSeeked);
    element.addEventListener('pause', onPause);
    element.addEventListener('ended', onEnded);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('progress', onProgress);
    element.addEventListener('error', onError);
    // `load` of a <track> does not bubble: listen in the capture phase.
    element.addEventListener('load', shiftCues, true);
    return () => {
      element.removeEventListener('load', shiftCues, true);
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('waiting', onWaiting);
      element.removeEventListener('seeking', onSeeking);
      element.removeEventListener('seeked', onSeeked);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('ended', onEnded);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('progress', onProgress);
      element.removeEventListener('error', onError);
    };
  }, [durationMs, engine, openedAt, resumeMs, stopClock]);

  // Every 10 s while playing, when the page is hidden or left, and when the player closes.
  useEffect(() => {
    const element = video.current;
    if (!element || startMs === null) return;
    const interval = window.setInterval(() => {
      if (!element.paused && !element.ended) sendLater.current('playing');
    }, OFFLINE_PROGRESS_MS);
    const onHidden = () => {
      if (document.visibilityState === 'hidden') {
        sendLater.current(element.paused ? 'paused' : 'playing');
      }
    };
    const onPageHide = () => sendLater.current('paused');
    document.addEventListener('visibilitychange', onHidden);
    window.addEventListener('pagehide', onPageHide);
    return () => {
      window.clearInterval(interval);
      document.removeEventListener('visibilitychange', onHidden);
      window.removeEventListener('pagehide', onPageHide);
      if (!element.ended) sendLater.current('paused');
    };
  }, [startMs]);

  const language = i18n.language;
  const elapsed = formatPlayerTime(currentMs, durationMs);
  const total = formatPlayerTime(durationMs, durationMs);
  const hidden = idle && playing && !menuOpen;
  const limit = watch?.limitMinutes;
  const subtitleOptions = [
    { id: 'off', label: t('player.tracks.off') },
    ...subtitles.map((entry, index) => ({
      id: String(entry.track.id),
      label: trackLabel(t, entry.track, index, language),
    })),
  ];

  return (
    <main
      ref={setPortal}
      className={styles.player}
      aria-label={t('player.region')}
      data-engine={engine ?? undefined}
      data-download-id={playback.downloadId}
      data-profile-id={playback.profileId}
      data-start-ms={startMs ?? undefined}
      data-seek-ms={seek?.ms}
      data-seek-count={seek?.count}
      data-ended-reason={limitReached ? 'limit' : undefined}
      data-controls={hidden ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
    >
      {/* The text tracks are the stored WebVTT files (below); a download without any has none. */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={video}
        className={styles.video}
        src={engine === 'blob' ? (blobUrl ?? undefined) : undefined}
        preload="auto"
        playsInline
        disableRemotePlayback
        onPointerUp={(event) => {
          if (event.button === 0) togglePlay();
        }}
        onDoubleClick={toggleFullscreen}
      >
        {subtitles.map((entry) => (
          <track
            key={entry.track.id}
            id={String(entry.track.id)}
            kind="subtitles"
            src={entry.url}
            srcLang={entry.track.language}
            label={entry.track.title ?? languageName(entry.track.language, language) ?? ''}
            default={entry.track.id === subtitle}
          />
        ))}
      </video>

      {failed && !limitReached && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.error.title')}</p>
          <p>{t('offline.player.failed')}</p>
        </div>
      )}

      {limitReached && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.watchLimit.title')}</p>
          <p>
            {limit !== undefined
              ? t('player.watchLimit.body', { count: limit })
              : t('player.watchLimit.bodyUnknown')}
          </p>
          <div>
            <AriaButton variant="secondary" icon="back" onPress={onBack} autoFocus>
              {t('common.back')}
            </AriaButton>
          </div>
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
        <h1 className={styles.title}>{title}</h1>
      </div>

      <div className={styles.bottom}>
        <SeekBar
          label={t('player.seek')}
          valueText={t('player.time', { elapsed, total })}
          currentMs={currentMs}
          durationMs={durationMs}
          buffered={buffered}
          onSeek={seekTo}
        />
        <div className={styles.bar}>
          <AriaButton
            variant="ghost"
            icon={playing ? 'pause' : 'play'}
            iconOnly
            aria-label={playing ? t('player.pause') : t('player.play')}
            onPress={togglePlay}
            isDisabled={limitReached}
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
          {subtitles.length > 0 && (
            <MenuButton
              buttonLabel={t('offline.player.subtitles')}
              label={t('offline.player.subtitles')}
              options={subtitleOptions}
              selectedKey={subtitle === null ? 'off' : String(subtitle)}
              onSelect={(id) => setSubtitle(id === 'off' ? null : Number(id))}
              isOpen={menuOpen}
              onOpenChange={setMenuOpen}
              isDisabled={limitReached}
              portalContainer={portal ?? undefined}
            />
          )}
        </div>
      </div>
    </main>
  );
}
