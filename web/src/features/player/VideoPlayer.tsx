import type {
  ItemRef,
  MediaFileInfo,
  PlaybackInfo,
  Progress,
  Quality,
  TrackList,
  WatchTime,
} from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, apiUrl, request } from '@/api/client';
import { episodeCode } from '@/features/catalog/labels';
import { formatPlayerTime } from '@/lib/format';
import { VisuallyHidden } from '@/ui';
import { AriaButton, MenuButton } from '@/ui/aria';
import type { PlaybackEngine } from './capabilities';
import {
  audioKey,
  chapterAt,
  chapterLabel,
  languageName,
  qualityLabel,
  selectedSubtitle,
  subtitleKey,
  trackLabel,
} from './labels';
import { SeekBar } from './SeekBar';
import { StreamInfo } from './StreamInfo';
import { useProgressReporter } from './useProgressReporter';
import styles from './Player.module.css';

/** Controls fade out after this long without input while the video plays. */
export const CONTROLS_HIDE_MS = 3000;
/** The track menu asks for new tracks this often while it is open (I-5). */
export const TRACKS_REFRESH_MS = 30_000;
/** Seconds the next-episode countdown runs (W-9). */
export const AUTO_NEXT_SECONDS = 10;
/** The countdown is announced when it starts and once more this many seconds before its end. */
const ANNOUNCE_LAST_SECONDS = 3;
const SEEK_STEP_S = 10;
const VOLUME_STEP = 0.1;
/** Resume only when the position is not within the last seconds of the file. */
const RESUME_TAIL_MS = 5000;
/** The skip button hides this close to the end of the intro. */
const SKIP_TAIL_MS = 500;

/** A change of the playback the server has to make: a new session (W-4, W-8). */
export interface PlaybackChange {
  /** Stream index of the audio track. */
  audioTrack?: number;
  /** Track id of the subtitle; null switches subtitles off. */
  subtitleTrack?: number | null;
  quality?: Quality;
}

/**
 * Why the server ended a running playback: the watch-time limit (K-5), or `admin` for any other
 * stop by the server (the admin, W-6; a sign-out elsewhere; a revoked guest). The 401 of the
 * stream names no reason yet, so the player says neutrally that the playback ended.
 */
export type EndedReason = 'limit' | 'admin';

type MenuName = 'tracks' | 'quality' | 'chapters' | 'version';

interface VideoPlayerProps {
  info: PlaybackInfo;
  /** Range stream or native HLS (the element loads streamUrl itself) or hls.js (MSE). */
  engine: Exclude<PlaybackEngine, 'unsupported'>;
  /** performance.now() when the player route opened. */
  openedAt: number;
  onBack: () => void;
  /** Position to start from instead of the server's resume position (after a switch). */
  startPositionMs?: number;
  /** Versions of the title (B-5); the version menu shows with two or more. */
  versions?: readonly MediaFileInfo[];
  /** A switch to another version, track or quality is on its way. */
  switching?: boolean;
  onSwitchVersion?: (mediaFileId: string, positionMs: number) => void;
  /** Another audio or subtitle track or quality: a new session at the position. */
  onSwitch?: (change: PlaybackChange, positionMs: number) => void;
  /** performance.now() of the menu choice that led to this session (data-switch-ms, W-4). */
  switchStartedAt?: number;
  /** Plays the next episode (auto-next, W-9). */
  onNextEpisode?: (next: ItemRef) => void;
  /** This player opened through auto-next (data-autonext="navigated"). */
  autoNext?: boolean;
  /** The "Stream-Info" overlay (key i). */
  infoOpen?: boolean;
  onToggleInfo?: () => void;
}

const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

/** Keys inside a player menu belong to the menu (arrows, Enter, Escape). */
const inMenu = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  target.closest('[role="menu"], [role="menuitemradio"], [role="menuitem"]') !== null;

function bufferedRanges(video: HTMLVideoElement): [number, number][] {
  const ranges: [number, number][] = [];
  for (let index = 0; index < video.buffered.length; index += 1) {
    ranges.push([video.buffered.start(index) * 1000, video.buffered.end(index) * 1000]);
  }
  return ranges;
}

/** Where playback starts: the resume position unless it lies in the last seconds of the file. */
export function resumeSeconds(resume: number, durationMs: number): number {
  if (resume <= 0 || (durationMs > 0 && resume >= durationMs - RESUME_TAIL_MS)) return 0;
  return resume / 1000;
}

/**
 * Whether the stream token of this playback was revoked: the server answers 401 for a stopped
 * session (admin, watch-time limit, logout). The media element reports a failed request without
 * its status, so a one-byte request asks again.
 */
export async function streamRevoked(streamUrl: string): Promise<boolean> {
  try {
    const url = new URL(apiUrl(streamUrl), window.location.href);
    const response = await fetch(
      new Request(url, { headers: { Range: 'bytes=0-0' }, cache: 'no-store' }),
    );
    void response.body?.cancel();
    return response.status === 401;
  } catch {
    return false;
  }
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
  onSwitch,
  switchStartedAt,
  onNextEpisode,
  autoNext = false,
  infoOpen = false,
  onToggleInfo,
}: VideoPlayerProps) {
  const { t, i18n } = useTranslation();
  // Fixed for the life of this player (one playback session).
  const [resumeMs] = useState(() => startPositionMs ?? info.resumePositionMs);
  const [switchFrom] = useState(() => switchStartedAt);
  const video = useRef<HTMLVideoElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const playButton = useRef<HTMLButtonElement>(null);
  const nextPanel = useRef<HTMLElement>(null);
  const hideTimer = useRef<number | undefined>(undefined);
  const osdTimer = useRef<number | undefined>(undefined);

  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(resumeMs);
  const [durationMs, setDurationMs] = useState(info.mediaFile.durationMs ?? 0);
  const [buffered, setBuffered] = useState<[number, number][]>([]);
  // Full screen belongs to the document, so it survives a new session or episode (W-2).
  const [fullscreen, setFullscreen] = useState(() => Boolean(document.fullscreenElement));
  const [idle, setIdle] = useState(false);
  const [osd, setOsd] = useState<string | null>(null);
  const [startMs, setStartMs] = useState<number | null>(null);
  // From the menu choice of the previous session until this one plays (W-4).
  const [switchMs, setSwitchMs] = useState<number | null>(null);
  // Last seek: from the element starting to seek until it plays again (e2e measurement, W-3).
  const [seek, setSeek] = useState<{ ms: number; count: number } | null>(null);
  const [failed, setFailed] = useState(false);
  // The server ended this playback (K-5 limit, W-6 admin): the player stops for good.
  const [ended, setEnded] = useState<{ reason: EndedReason; watchTime?: WatchTime } | null>(null);
  // The open menu keeps the controls on screen; shortcut keys open menus as well.
  const [openMenu, setOpenMenu] = useState<MenuName | null>(null);
  // Auto-next (W-9): the end of the countdown, or null; cancelled stays for this session.
  const [countdownEnd, setCountdownEnd] = useState<number | null>(null);
  const [countdownCancelled, setCountdownCancelled] = useState(false);
  const [now, setNow] = useState(() => Date.now());
  // The menus render into the player, so they stay visible in full screen.
  const [portal, setPortal] = useState<HTMLElement | null>(null);
  const containerRef = useCallback((node: HTMLDivElement | null) => {
    container.current = node;
    setPortal(node);
  }, []);
  // During a switch the old session is ended already: playing or seeking it would fail.
  const locked = useRef(switching);
  // The viewer switched away from this session (track, quality, version): the old session ends on
  // the server, and its 401 or a failed request is the switch, not a stop by the server (W-1).
  const switchedAway = useRef(false);
  useEffect(() => {
    locked.current = switching || ended !== null;
    if (switching) switchedAway.current = true;
  }, [switching, ended]);
  const watchTime = useRef<WatchTime | undefined>(info.watchTime);
  const stopHls = useRef<() => void>(() => undefined);

  /** The server ended the playback: pause, stop loading, show why. */
  const endedOnce = useRef(false);
  const endPlayback = useCallback((reason: EndedReason) => {
    if (endedOnce.current) return;
    endedOnce.current = true;
    const element = video.current;
    locked.current = true;
    stopHls.current();
    if (element && !element.paused) element.pause();
    setCountdownEnd(null);
    setEnded((current) => current ?? { reason, watchTime: watchTime.current });
  }, []);

  /** A 401 of the stream: the watch-time limit when the day's minutes were (nearly) used up. */
  const onRevoked = useCallback(() => {
    if (switchedAway.current) return;
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
    fallbackDurationMs: info.mediaFile.durationMs ?? 0,
    video,
    enabled: true,
    onProgress,
  });

  // I-5: the track menu asks the server again when it opens and every 30 s while it is open, so a
  // subtitle placed next to the file shows up without restarting the playback.
  const tracksOpen = openMenu === 'tracks';
  const trackList = useQuery({
    queryKey: ['tracks', info.item.id, info.mediaFile.id],
    queryFn: ({ signal }): Promise<TrackList> =>
      request(
        api.GET('/items/{id}/tracks', {
          params: { path: { id: info.item.id }, query: { mediaFileId: info.mediaFile.id } },
          signal,
        }),
      ),
    enabled: tracksOpen,
    staleTime: 0,
    refetchInterval: tracksOpen ? TRACKS_REFRESH_MS : false,
    refetchOnWindowFocus: false,
  });
  const tracks = trackList.data ?? info.tracks;

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

  const seekTo = useCallback((ms: number) => {
    const element = video.current;
    if (!element || locked.current) return;
    const end = Number.isFinite(element.duration) ? element.duration : Infinity;
    element.currentTime = Math.min(end, Math.max(0, ms / 1000));
    setCurrentMs(element.currentTime * 1000);
  }, []);

  const seekBy = useCallback(
    (seconds: number) => {
      const element = video.current;
      if (!element) return;
      seekTo((element.currentTime + seconds) * 1000);
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
    [t],
  );

  // The whole document goes full screen, not this player: a switch or auto-next replaces the
  // player, and removing the full-screen element would end full screen (W-2). The player covers
  // the viewport and its menus render inside it, so nothing else shows.
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

  // Segments (W-9): skip the intro while inside it; the outro starts the next-episode countdown.
  const { intro, outro } = info.segments;
  const next = info.nextEpisode;
  const inIntro =
    intro !== undefined && currentMs >= intro.startMs && currentMs < intro.endMs - SKIP_TAIL_MS;
  const inOutro = outro !== undefined && currentMs >= outro.startMs && currentMs < outro.endMs;
  const skipIntro = useCallback(() => {
    if (intro) seekTo(intro.endMs);
  }, [intro, seekTo]);

  const countdownActive = countdownEnd !== null && ended === null;
  const countdownLeft = countdownActive
    ? Math.max(0, Math.ceil((countdownEnd - now) / 1000))
    : AUTO_NEXT_SECONDS;
  const playNext = useCallback(() => {
    if (!next || !onNextEpisode) return;
    setCountdownEnd(null);
    onNextEpisode(next);
  }, [next, onNextEpisode]);
  const cancelCountdown = useCallback(() => {
    setCountdownEnd(null);
    setCountdownCancelled(true);
  }, []);

  // The media events start the countdown: from the outro on, or at the end of the file.
  const maybeCountdown = useRef<(ms: number, atEnd: boolean) => void>(() => undefined);
  const playNextLater = useRef(playNext);
  useEffect(() => {
    playNextLater.current = playNext;
    maybeCountdown.current = (ms, atEnd) => {
      if (!next || !onNextEpisode || countdownCancelled || countdownEnd !== null || ended) return;
      if (atEnd || (outro !== undefined && ms >= outro.startMs)) {
        const start = Date.now();
        setNow(start);
        setCountdownEnd(start + AUTO_NEXT_SECONDS * 1000);
      }
    };
  });

  useEffect(() => {
    if (countdownEnd === null) return;
    const timer = window.setInterval(() => {
      const current = Date.now();
      setNow(current);
      if (current >= countdownEnd) {
        window.clearInterval(timer);
        playNextLater.current();
      }
    }, 250);
    return () => window.clearInterval(timer);
  }, [countdownEnd]);

  const hasTracks = tracks.audio.length > 0 || tracks.subtitles.length > 0;
  const hasQualities = info.qualities.length > 0 && onSwitch !== undefined;
  const hasChapters = info.chapters.length > 0;
  const hasVersions = versions.length > 1 && onSwitchVersion !== undefined;

  // react-aria buttons keep Enter and Space to themselves, so the window never sees them: during
  // the countdown a capture listener lets a press of any other focused button (play/pause on the
  // remote's OK) cancel it as well; the button still does its job.
  useEffect(() => {
    if (!countdownActive) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      if (!isActivatable(event.target)) return;
      if (event.target instanceof Node && nextPanel.current?.contains(event.target)) return;
      cancelCountdown();
    };
    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, [cancelCountdown, countdownActive]);

  // Key map (W-1, C-4, W-4, W-9): Space/Enter play-pause, ←/→ ±10 s, ↑/↓ volume, F full screen,
  // I stream info, T tracks, Q quality, C chapters, S skip intro, N next episode, Escape/Backspace
  // back to the detail page. During the next-episode countdown any other key cancels it.
  useEffect(() => {
    const open = (menu: MenuName, available: boolean, event: KeyboardEvent) => {
      if (!available) return;
      event.preventDefault();
      setOpenMenu(menu);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      poke();
      if (inMenu(event.target)) {
        // Backspace is the remote's back key (C-4): in a menu it closes the menu, like Escape.
        if (event.key === 'Backspace') {
          event.preventDefault();
          setOpenMenu(null);
        }
        return;
      }
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      if (countdownActive) {
        if (key === 'n') {
          event.preventDefault();
          playNext();
          return;
        }
        // Tab moves between the controls; modifier keys alone do nothing.
        if (key === 'Tab' || ['Shift', 'Control', 'Alt', 'Meta'].includes(key)) return;
        const activates = key === 'Enter' || key === ' ';
        // The countdown's own buttons take Enter and Space.
        if (
          activates &&
          event.target instanceof Node &&
          nextPanel.current?.contains(event.target)
        ) {
          return;
        }
        cancelCountdown();
        // Any other focused button (play/pause on the remote's OK) still does its job.
        if (activates && isActivatable(event.target)) return;
        event.preventDefault();
        return;
      }
      switch (key) {
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
          event.preventDefault();
          toggleFullscreen();
          return;
        case 'i':
          event.preventDefault();
          onToggleInfo?.();
          return;
        case 't':
          open('tracks', hasTracks && !switching && ended === null, event);
          return;
        case 'q':
          open('quality', hasQualities && !switching && ended === null, event);
          return;
        case 'c':
          open('chapters', hasChapters && ended === null, event);
          return;
        case 's':
          if (!inIntro) return;
          event.preventDefault();
          skipIntro();
          return;
        case 'n':
          if (!next || !onNextEpisode || ended !== null) return;
          event.preventDefault();
          playNext();
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
  }, [
    cancelCountdown,
    changeVolume,
    countdownActive,
    ended,
    hasChapters,
    hasQualities,
    hasTracks,
    inIntro,
    infoOpen,
    next,
    onBack,
    onNextEpisode,
    onToggleInfo,
    playNext,
    poke,
    seekBy,
    skipIntro,
    switching,
    toggleFullscreen,
    togglePlay,
  ]);

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
        // A 401 never heals: the server revoked the stream token (W-6 admin, K-5 limit). Any
        // other fatal error gets one recovery attempt per kind before the player gives up: a
        // media error (decode, buffer append) re-creates the media source, a network error
        // restarts loading at the current position.
        let mediaRecovered = false;
        let networkRecovered = false;
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (switchedAway.current) return;
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
  }, [engine, info, onRevoked, resumeMs]);

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
      // The start of the route's first session (data-start-ms); a later session of a switch
      // measures from the menu choice instead (data-switch-ms).
      if (switchFrom === undefined) {
        setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
      } else {
        setSwitchMs((value) => value ?? Math.round(performance.now() - switchFrom));
      }
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
    const onEnded = () => {
      setPlaying(false);
      maybeCountdown.current(element.currentTime * 1000, true);
    };
    const onTime = () => {
      setCurrentMs(element.currentTime * 1000);
      maybeCountdown.current(element.currentTime * 1000, false);
      // Seeking within the buffer fires no new `playing`: the first tick after `seeked` counts.
      if (seeked && !element.paused) finishSeek();
    };
    const onProgressEvent = () => setBuffered(bufferedRanges(element));
    const onDuration = () => {
      if (Number.isFinite(element.duration)) setDurationMs(element.duration * 1000);
    };
    // With hls.js the element's errors reach hls.js as well, which reports them (and recovers).
    // A direct stream that the server closed answers 401 to the element's next range request.
    const onError = () => {
      if (engine === 'hlsjs' || switchedAway.current) return;
      void streamRevoked(info.streamUrl).then((revoked) => {
        if (revoked) onRevoked();
        else setFailed(true);
      });
    };

    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('seeking', onSeeking);
    element.addEventListener('seeked', onSeeked);
    element.addEventListener('pause', onPause);
    element.addEventListener('ended', onEnded);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('progress', onProgressEvent);
    element.addEventListener('durationchange', onDuration);
    element.addEventListener('error', onError);
    return () => {
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('seeking', onSeeking);
      element.removeEventListener('seeked', onSeeked);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('ended', onEnded);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('progress', onProgressEvent);
      element.removeEventListener('durationchange', onDuration);
      element.removeEventListener('error', onError);
    };
  }, [engine, info, onRevoked, openedAt, resumeMs, switchFrom]);

  // Another version, track or quality: pause and stop loading here (the session is about to end)
  // and hand the position over to the next session.
  const positionNow = () => Math.round((video.current?.currentTime ?? 0) * 1000);
  const leaveSession = () => {
    switchedAway.current = true;
    const position = positionNow();
    video.current?.pause();
    stopHls.current();
    return position;
  };
  const switchVersion = (mediaFileId: string) => {
    if (!onSwitchVersion || switching) return;
    onSwitchVersion(mediaFileId, leaveSession());
  };
  const switchPlayback = (change: PlaybackChange) => {
    if (!onSwitch || switching) return;
    onSwitch(change, leaveSession());
  };

  const currentVersion = info.mediaFile.versionLabel ?? info.mediaFile.container.toUpperCase();
  const language = i18n.language;
  const subtitle = selectedSubtitle(info);
  const audioIndex =
    info.selection.audioTrack ?? tracks.audio.find((track) => track.default)?.index;
  const trackGroups = [
    {
      id: 'audio',
      label: t('player.tracks.audio'),
      selectedKey: audioIndex === undefined ? '' : audioKey(audioIndex),
      options: tracks.audio.map((track, index) => ({
        id: audioKey(track.index),
        label: trackLabel(t, track, index, language),
      })),
    },
    {
      id: 'subtitle',
      label: t('player.tracks.subtitles'),
      selectedKey: subtitleKey(info.selection.subtitleTrack),
      options: [
        { id: subtitleKey(undefined), label: t('player.tracks.off') },
        ...tracks.subtitles.map((track, index) => ({
          id: subtitleKey(track.id),
          label: trackLabel(t, track, index, language),
        })),
      ],
    },
  ];
  const onTrack = (id: string, group?: string) => {
    if (group === 'audio') {
      switchPlayback({ audioTrack: Number(id.slice('audio:'.length)) });
    } else {
      const value = id.slice('subtitle:'.length);
      switchPlayback({ subtitleTrack: value === 'off' ? null : Number(value) });
    }
  };
  const qualityOptions = info.qualities.map((option) => ({
    id: option.quality,
    label: qualityLabel(t, option, info),
    disabled: !option.available,
  }));
  // The rung the server says it delivers (rule 12), not the viewer's wish.
  const qualityKey = info.qualities.find((option) => option.current)?.quality ?? 'auto';
  const currentQuality =
    qualityOptions.find((option) => option.id === qualityKey)?.label ?? t('player.quality.auto');
  const chapterIndex = chapterAt(info.chapters, currentMs);
  const menuProps = (menu: MenuName) => ({
    // After the server ended the playback a pick would start a new session behind the notice.
    isDisabled: ended !== null,
    isOpen: openMenu === menu,
    onOpenChange: (isOpen: boolean) =>
      setOpenMenu((current) => (isOpen ? menu : current === menu ? null : current)),
    portalContainer: portal ?? undefined,
  });

  const elapsed = formatPlayerTime(currentMs, durationMs);
  const total = formatPlayerTime(durationMs, durationMs);
  const timeText = t('player.time', { elapsed, total });
  const hidden = idle && playing && openMenu === null && !countdownActive;
  const limit = ended?.watchTime?.limitMinutes;
  const nextCode = next ? episodeCode(t, next) : undefined;
  const nextName = next
    ? nextCode
      ? t('item.nextEpisodeLabel', { code: nextCode, title: next.title })
      : next.title
    : '';
  // Screen readers hear the countdown when it starts and shortly before its end, not every second.
  const announcement =
    countdownActive && next
      ? t('player.next.announce', {
          name: nextName,
          count: countdownLeft > ANNOUNCE_LAST_SECONDS ? AUTO_NEXT_SECONDS : ANNOUNCE_LAST_SECONDS,
        })
      : '';

  // The skip buttons and the countdown disappear with their state; a focus inside them would fall
  // back to the page, so it moves to the play button (C-4).
  const overlays = [
    inIntro && ended === null,
    inOutro && !next && ended === null,
    countdownActive,
  ].join();
  useLayoutEffect(() => {
    const active = document.activeElement;
    if (active === null || active === document.body) playButton.current?.focus();
  }, [overlays]);

  return (
    <main
      ref={containerRef}
      className={styles.player}
      aria-label={t('player.region')}
      data-start-ms={startMs ?? undefined}
      data-switch-ms={switchMs ?? undefined}
      data-seek-ms={seek?.ms}
      data-seek-count={seek?.count}
      data-engine={engine}
      data-media-file-id={info.mediaFile.id}
      data-playback-session-id={info.playbackSessionId}
      data-ended-reason={ended?.reason}
      data-autonext={countdownActive ? 'countdown' : autoNext ? 'navigated' : undefined}
      data-controls={hidden && !infoOpen ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
    >
      {/* Without a text subtitle track there is nothing to caption (W-4: bitmap tracks are burned in). */}
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
      >
        {info.subtitleUrl && (
          <track
            kind="subtitles"
            src={apiUrl(info.subtitleUrl)}
            srcLang={subtitle?.language}
            label={
              subtitle
                ? (subtitle.title ?? languageName(subtitle.language, language) ?? '')
                : undefined
            }
            default
          />
        )}
      </video>

      {failed && ended === null && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.error.title')}</p>
          <p>{t('player.error.body')}</p>
        </div>
      )}

      {ended && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>
            {ended.reason === 'limit' ? t('player.watchLimit.title') : t('player.ended.title')}
          </p>
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
          onSeek={seekTo}
        />
        <div className={styles.bar}>
          <AriaButton
            ref={playButton}
            variant="ghost"
            icon={playing ? 'pause' : 'play'}
            iconOnly
            aria-label={playing ? t('player.pause') : t('player.play')}
            onPress={togglePlay}
            isDisabled={switching || ended !== null}
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
          {hasTracks && onSwitch && (
            <MenuButton
              buttonLabel={t('player.tracks.button')}
              label={t('player.tracks.button')}
              groups={trackGroups}
              onSelect={onTrack}
              {...menuProps('tracks')}
            />
          )}
          {hasQualities && (
            <MenuButton
              buttonLabel={t('player.quality.button', { label: currentQuality })}
              label={currentQuality}
              options={qualityOptions}
              selectedKey={qualityKey}
              onSelect={(id) => switchPlayback({ quality: id as Quality })}
              {...menuProps('quality')}
            />
          )}
          {hasChapters && (
            <MenuButton
              buttonLabel={t('player.chapters.button')}
              label={t('player.chapters.button')}
              options={info.chapters.map((chapter, index) => ({
                id: String(index),
                label: chapterLabel(t, chapter, index, durationMs),
              }))}
              selectedKey={String(chapterIndex)}
              reselect
              onSelect={(id) => {
                const chapter = info.chapters[Number(id)];
                if (chapter) seekTo(chapter.startMs);
              }}
              {...menuProps('chapters')}
            />
          )}
          {hasVersions && (
            <MenuButton
              buttonLabel={t('player.versionButton', { label: currentVersion })}
              label={currentVersion}
              options={versions.map((version, index) => ({
                id: version.id,
                label: version.versionLabel ?? String(index + 1),
              }))}
              selectedKey={info.mediaFile.id}
              onSelect={switchVersion}
              {...menuProps('version')}
            />
          )}
        </div>
      </div>

      {inIntro && ended === null && (
        <div className={styles.skip}>
          <AriaButton variant="secondary" onPress={skipIntro}>
            {t('player.skipIntro')}
          </AriaButton>
        </div>
      )}
      {inOutro && !next && ended === null && (
        <div className={styles.skip}>
          <AriaButton variant="secondary" onPress={() => seekTo(outro.endMs)}>
            {t('player.skipOutro')}
          </AriaButton>
        </div>
      )}
      <div aria-live="polite" aria-atomic="true">
        <VisuallyHidden>{announcement}</VisuallyHidden>
      </div>
      {countdownActive && next && (
        <section ref={nextPanel} className={styles.next} aria-labelledby="next-episode-title">
          <h2 id="next-episode-title" className={styles.nextTitle}>
            {t('player.next.title')}
          </h2>
          <p className={styles.nextName}>{nextName}</p>
          <p className={styles.nextCountdown}>
            {t('player.next.countdown', { count: countdownLeft })}
          </p>
          <div className={styles.nextActions}>
            <AriaButton variant="primary" icon="play" onPress={playNext}>
              {t('player.next.play')}
            </AriaButton>
            <AriaButton variant="ghost" onPress={cancelCountdown}>
              {t('player.next.cancel')}
            </AriaButton>
          </div>
        </section>
      )}
    </main>
  );
}
