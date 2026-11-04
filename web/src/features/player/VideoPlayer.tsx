import type {
  ItemDetail,
  ItemRef,
  MediaFileInfo,
  PlaybackInfo,
  Progress,
  Quality,
  Track,
  TrackList,
  WatchTime,
} from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type WheelEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, apiUrl, imageUrl, request } from '@/api/client';
import { sessionQuery } from '@/app/session';
import { episodeCode } from '@/features/catalog/labels';
import { formatBitrate } from '@/lib/format';
import { supportsWebAudioBoost } from '@/lib/webAudio';
import { itemQuery } from '@/features/catalog/queries';
import { VisuallyHidden } from '@/ui';
import {
  AriaButton,
  BOOST_MAX,
  focusOnMount,
  PlayerChrome,
  VOLUME_STEP,
  type PlayerChromeMenu,
} from '@/ui/aria';
import { nextChapterStart, previousChapterStart } from './audio';
import { playbackCapabilities, type PlaybackEngine } from './capabilities';
import { stepPlaybackRate } from './keymap';
import {
  chapterAt,
  chapterLabel,
  languageName,
  qualityLabel,
  selectedSubtitle,
  trackLabel,
} from './labels';
import { SettingsPanel, type SettingsPage } from './panels/SettingsPanel';
import { AudioPanel, SubtitlePanel } from './panels/TrackPanels';
import { ShortcutHelp } from './ShortcutHelp';
import { StreamInfo } from './StreamInfo';
import { usePlaybackStopWatch } from './usePlaybackStopWatch';
import { useAudioGraph } from './useAudioGraph';
import { usePlayerKeys } from './usePlayerKeys';
import { usePlayerPrefs } from './usePlayerPrefs';
import { useSubtitleStyle } from './useSubtitleStyle';
import { SubtitleLayer, useSubtitleCues } from './SubtitleLayer';
import { useProgressReporter } from './useProgressReporter';
import { useVolume } from './useVolume';
import styles from './Player.module.css';

/** Controls fade out after this long without input while the video plays. */
export const CONTROLS_HIDE_MS = 3000;
/** The track menu asks for new tracks this often while it is open (I-5). */
export const TRACKS_REFRESH_MS = 30_000;
/** Seconds the next-episode countdown runs (W-9). */
export const AUTO_NEXT_SECONDS = 10;
/** The countdown is announced when it starts and once more this many seconds before its end. */
const ANNOUNCE_LAST_SECONDS = 3;
/** Resume only when the position is not within the last seconds of the file. */
const RESUME_TAIL_MS = 5000;
/** One notch of a mouse wheel in pixels (Chromium, Firefox in pixel mode), and one line. */
const WHEEL_NOTCH_PX = 100;
const WHEEL_LINE_PX = 100 / 3;
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

type MenuName = 'chapters';

/**
 * The panel beside the video: sound (T), subtitles (U), the gear (Q opens its quality page) or the
 * shortcut help (?). One at a time.
 */
type PanelName = 'audio' | 'subtitle' | 'settings' | 'help';

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
  /** The title's details (logo, genres, series), once loaded; the head shows text until then. */
  detail?: ItemDetail;
  /** What the previous session of this title left (review m-2): a switch keeps it. */
  carried?: PlayerCarry;
  /** Reports what the next session should keep. */
  onCarry?: (patch: Partial<PlayerCarry>) => void;
}

/**
 * What a switch of track, quality or version keeps (PlayerPage mounts a new player per session):
 * mute, speed, the volume (a boost above 100 % is not stored on the device) and night mode.
 */
export interface PlayerCarry {
  muted: boolean;
  rate: number;
  volume?: number;
  nightMode: boolean;
}

export const NO_CARRY: PlayerCarry = { muted: false, rate: 1, nightMode: false };

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
  detail,
  carried = NO_CARRY,
  onCarry,
}: VideoPlayerProps) {
  const { t, i18n } = useTranslation();
  // Fixed for the life of this player (one playback session).
  const [resumeMs] = useState(() => startPositionMs ?? info.resumePositionMs);
  const [start] = useState(() => carried);
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
  // The panel beside the video (sound, subtitles, the shortcut help); it keeps the controls on
  // screen like a menu.
  const [panel, setPanel] = useState<PanelName | null>(null);
  const helpOpen = panel === 'help';
  const openPanel = useCallback((name: PanelName) => {
    setOpenMenu(null);
    setPanel(name);
  }, []);
  const togglePanel = useCallback((name: PanelName) => {
    setOpenMenu(null);
    setPanel((current) => (current === name ? null : name));
  }, []);
  const closePanel = useCallback(() => setPanel(null), []);
  // The page of the gear panel; `direct` when a key opened it on that page (no way up).
  const [settingsView, setSettingsView] = useState<{ page: SettingsPage; direct: boolean }>({
    page: 'main',
    direct: false,
  });
  const openSettings = useCallback((page: SettingsPage, direct: boolean) => {
    setOpenMenu(null);
    setSettingsView({ page, direct });
    setPanel('settings');
  }, []);
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

  /**
   * A 401 of the stream, or the stop watch with the server's reason: without one, the watch-time
   * limit when the day's minutes were (nearly) used up.
   */
  const onRevoked = useCallback(
    (reason?: EndedReason) => {
      if (switchedAway.current) return;
      const remaining = watchTime.current?.remainingMinutes;
      endPlayback(reason ?? (remaining !== undefined && remaining <= 1 ? 'limit' : 'admin'));
    },
    [endPlayback],
  );

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

  // A fully buffered title makes no stream request that could answer 401 (concept 13, 11.13).
  usePlaybackStopWatch({
    playbackSessionId: info.playbackSessionId,
    active: playing && !switching && ended === null,
    onStopped: onRevoked,
  });

  // I-5: the sound and subtitle panels ask the server again when they open and every 30 s while
  // one is open, so a subtitle placed next to the file shows up without restarting the playback.
  const tracksOpen = panel === 'audio' || panel === 'subtitle';
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

  // The head (11.2): the title's logo, for an episode the series' (a second request, usually in
  // the cache of the detail page). Until they arrive the title shows as text.
  const seriesId = detail?.series?.id;
  const series = useQuery({ ...itemQuery(seriesId ?? ''), enabled: seriesId !== undefined });
  const [prefs, setPrefs] = usePlayerPrefs();
  const [subtitleStyle, setSubtitleStyle] = useSubtitleStyle();
  const subtitleTrackElement = useRef<HTMLTrackElement>(null);
  // The profile's language sorts the panels (stage 1: `profiles.language`, 11.10); the UI language
  // follows it, so it stands in until the session is there.
  const session = useQuery(sessionQuery);
  const profileLanguage = session.data?.profile?.language ?? i18n.language;
  // The audio codecs this browser plays, for the mark «wird umgewandelt».
  const [decodableAudio] = useState(() => playbackCapabilities().audio);
  const timeDisplay = prefs.timeDisplay ?? 'elapsed';

  // The first focus goes to play/pause (focusOnMount, not react-aria's autoFocus: 11.13); every
  // session is a new player, so a switch focuses it again.
  const playRef = useCallback((element: HTMLButtonElement | null) => {
    playButton.current = element;
    focusOnMount(element);
  }, []);

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

  // What the viewer wants, not what the element reports: re-attaching the element (hls.js after a
  // media error) runs the load algorithm, which clears `paused` without a `pause` event. The
  // element would then read as paused while the player still shows "Pause", and the reloaded
  // source would play on even after a pause.
  const wantPlaying = useRef(true);

  const togglePlay = useCallback(() => {
    const element = video.current;
    if (!element || locked.current) return;
    wantPlaying.current = !wantPlaying.current;
    if (wantPlaying.current) element.play().catch(() => setPlaying(false));
    else {
      setPlaying(false);
      element.pause();
    }
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

  // Up to 150 % and night mode through Web Audio, except in Safari (11.15).
  const [webAudio, setWebAudio] = useState(() => supportsWebAudioBoost());
  const volume = useVolume(video, {
    initialMuted: start.muted,
    initialVolume: start.volume,
    max: webAudio ? BOOST_MAX : 1,
  });
  const [nightMode, setNightMode] = useState(start.nightMode);
  // The browser refused the graph: night mode and the volume above 100 % go away.
  const { setVolume: setLevel, volume: levelBefore } = volume;
  const levelAtFailure = useRef(levelBefore);
  useEffect(() => {
    levelAtFailure.current = levelBefore;
  }, [levelBefore]);
  const audioGraphUnavailable = useCallback(() => {
    setWebAudio(false);
    setNightMode(false);
    if (levelAtFailure.current > 1) setLevel(1);
  }, [setLevel]);
  useAudioGraph({
    media: video,
    enabled: webAudio,
    nightMode,
    gain: volume.gain,
    onUnavailable: audioGraphUnavailable,
  });
  const { muted: mutedNow, volume: levelNow } = volume;
  useEffect(() => {
    onCarry?.({ muted: mutedNow, volume: levelNow, nightMode });
  }, [levelNow, mutedNow, nightMode, onCarry]);
  /** «Lautstärke 80 %»; above 100 % with the warning that it may distort. */
  const volumeText = useCallback(
    (level: number) => {
      const percent = Math.round(level * 100);
      return level > 1
        ? t('player.volumeControl.boosted', { percent })
        : t('player.volume', { percent });
    },
    [t],
  );
  const stepVolume = volume.step;
  const changeVolume = useCallback(
    (delta: number) => showOsd(volumeText(stepVolume(delta))),
    [stepVolume, volumeText],
  );
  const { toggleMute: toggleElementMute, volume: level } = volume;
  const toggleMute = useCallback(() => {
    const muted = toggleElementMute();
    showOsd(muted ? t('player.chrome.muted') : volumeText(level));
  }, [level, t, toggleElementMute, volumeText]);

  // Mouse wheel over the picture (V-3): one notch is one volume step. Over the bar and over a panel
  // the wheel stays theirs (the chrome lets the pointer through to the video only in between).
  // Trackpads send many small deltas; they add up to notches.
  const wheel = useRef(0);
  const onWheel = (event: WheelEvent<HTMLElement>) => {
    // Ctrl with the wheel zooms the page.
    if (event.target !== video.current || event.deltaY === 0 || event.ctrlKey) return;
    const pixels =
      event.deltaMode === 1
        ? event.deltaY * WHEEL_LINE_PX
        : event.deltaMode === 2
          ? event.deltaY * WHEEL_NOTCH_PX * 8
          : event.deltaY;
    if (Math.sign(pixels) !== Math.sign(wheel.current)) wheel.current = 0;
    wheel.current += pixels;
    const notches = Math.trunc(wheel.current / WHEEL_NOTCH_PX);
    if (notches === 0) return;
    wheel.current -= notches * WHEEL_NOTCH_PX;
    changeVolume(-notches * VOLUME_STEP);
  };

  // Speed (V-7): [ and ] in steps of 0.25 from 0.5× to 2×; a switch keeps it.
  const [rate, setRate] = useState(start.rate);
  useEffect(() => {
    onCarry?.({ rate });
    const element = video.current;
    if (!element) return;
    // A new source falls back to the default rate; both keep the speed.
    element.defaultPlaybackRate = rate;
    element.playbackRate = rate;
  }, [onCarry, rate]);
  const changeRate = useCallback(
    (delta: number) => {
      const next = stepPlaybackRate(video.current?.playbackRate ?? 1, delta);
      setRate(next);
      const shown = new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(next);
      showOsd(t('player.chrome.rate', { rate: shown }));
    },
    [i18n.language, t],
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

  // Picture in picture (V-9): Chromium and Safari; Firefox has a button of its own on the video.
  const [pictureInPicture, setPictureInPicture] = useState(false);
  useEffect(() => {
    const element = video.current;
    if (!element) return;
    const onEnter = () => setPictureInPicture(true);
    const onLeave = () => setPictureInPicture(false);
    element.addEventListener('enterpictureinpicture', onEnter);
    element.addEventListener('leavepictureinpicture', onLeave);
    return () => {
      element.removeEventListener('enterpictureinpicture', onEnter);
      element.removeEventListener('leavepictureinpicture', onLeave);
    };
  }, []);
  const togglePictureInPicture = useCallback(() => {
    const element = video.current;
    if (!element) return;
    if (document.pictureInPictureElement) void document.exitPictureInPicture().catch(() => {});
    else void element.requestPictureInPicture().catch(() => {});
  }, []);

  // Chapter back and forward (V-2, ⇧←/⇧→): back restarts the chapter after its first seconds.
  const { chapters } = info;
  const previousChapter = useCallback(() => {
    if (chapters.length === 0) return;
    seekTo(previousChapterStart(chapters, (video.current?.currentTime ?? 0) * 1000));
  }, [chapters, seekTo]);
  const nextChapter = useCallback(() => {
    const target = nextChapterStart(chapters, (video.current?.currentTime ?? 0) * 1000);
    if (target !== null) seekTo(target);
  }, [chapters, seekTo]);

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
  // Sound and subtitles change through the server: a new session (rule 12).
  const canSwitchTracks = hasTracks && onSwitch !== undefined;
  const hasQualities = info.qualities.length > 0 && onSwitch !== undefined;
  const hasChapters = info.chapters.length > 0;
  const hasVersions = versions.length > 1 && onSwitchVersion !== undefined;

  // Faded out: no input while playing, nothing open that needs the controls.
  const hidden =
    idle && playing && openMenu === null && panel === null && !countdownActive && !infoOpen;
  // The text subtitles of this session, drawn by the player (11.10).
  const cueTexts = useSubtitleCues({
    video,
    track: subtitleTrackElement,
    src: info.subtitleUrl,
    offsetMs: subtitleStyle.offsetMs,
    pictureInPicture,
  });
  const available = ended === null;
  usePlayerKeys({
    hidden,
    countdown: countdownActive
      ? { panel: nextPanel, cancel: cancelCountdown, playNext }
      : undefined,
    durationMs,
    onKey: poke,
    actions: {
      togglePlay,
      seekBy,
      seekTo,
      changeVolume,
      changeRate,
      toggleMute,
      toggleFullscreen,
      previousChapter: hasChapters ? previousChapter : undefined,
      nextChapter: hasChapters ? nextChapter : undefined,
      toggleInfo: onToggleInfo,
      openTracks:
        canSwitchTracks && !switching && available ? (kind) => openPanel(kind) : undefined,
      openQuality:
        hasQualities && !switching && available ? () => openSettings('quality', true) : undefined,
      openChapters: hasChapters && available ? () => setOpenMenu('chapters') : undefined,
      skipIntro: inIntro ? skipIntro : undefined,
      playNext: next && onNextEpisode && available ? playNext : undefined,
      toggleHelp: () => togglePanel('help'),
      closeMenu: () => setOpenMenu(null),
      closeOpen: () => {
        if (panel !== null) closePanel();
        else if (infoOpen) onToggleInfo?.();
        else return false;
        return true;
      },
      back: onBack,
    },
  });

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
      if (wantPlaying.current) element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      wantPlaying.current = true;
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
    // A pause the element reports (the viewer, the end of the title, a stop by the server) is a
    // wish; the silent one of the load algorithm is not and fires no event.
    const onPause = () => {
      wantPlaying.current = false;
      setPlaying(false);
    };
    const onEnded = () => {
      wantPlaying.current = false;
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

  const language = i18n.language;
  const subtitle = selectedSubtitle(info);
  const audioIndex =
    info.selection.audioTrack ?? tracks.audio.find((track) => track.default)?.index;
  // Sound and subtitles (V-5): the server's pick, the panels sort only what they show.
  const audioTrack = tracks.audio.find((track) => track.index === audioIndex);
  const audioWord = audioTrack
    ? (languageName(audioTrack.language, language) ??
      trackLabel(t, audioTrack, tracks.audio.indexOf(audioTrack), language))
    : t('player.tracks.unknown', { n: 1 });
  const subtitleTrack = tracks.subtitles.find((track) => track.id === info.selection.subtitleTrack);
  const subtitleWord = subtitleTrack
    ? (languageName(subtitleTrack.language, language) ??
      trackLabel(t, subtitleTrack, tracks.subtitles.indexOf(subtitleTrack), language))
    : t('player.tracks.off');
  // A mark only (11.3 «Kosten»): a codec the browser cannot play is converted, and while the file
  // plays directly another track than the file's default makes the server remux it.
  const converts = (track: Track) =>
    track.index !== audioIndex &&
    (!decodableAudio.includes(track.codec) || (info.mode === 'direct' && !track.default));
  const panelButton = (name: 'audio' | 'subtitle', label: string, text: string) => ({
    label,
    text,
    expanded: panel === name,
    onPress: () => togglePanel(name),
    // After the server ended the playback a pick would start a new session behind the notice.
    isDisabled: switching || ended !== null,
  });
  // The gear (11.2): the server's rung and file (rule 12); a choice starts a new session.
  const qualityChoice = {
    options: info.qualities.map((option) => ({
      id: option.quality,
      label: qualityLabel(t, option, info),
      detail: option.bitrate !== undefined ? formatBitrate(option.bitrate, language) : undefined,
      disabled: !option.available,
    })),
    // The rung the server says it delivers, not the viewer's wish.
    selectedId: info.qualities.find((option) => option.current)?.quality ?? 'auto',
    onSelect: (id: string) => switchPlayback({ quality: id as Quality }),
  };
  const versionChoice = {
    options: versions.map((version, index) => ({
      id: version.id,
      label: version.versionLabel ?? String(index + 1),
    })),
    selectedId: info.mediaFile.id,
    onSelect: switchVersion,
  };
  const chapterIndex = chapterAt(info.chapters, currentMs);
  const menuProps = (menu: MenuName) => ({
    // After the server ended the playback a pick would start a new session behind the notice.
    isDisabled: ended !== null,
    isOpen: openMenu === menu,
    onOpenChange: (isOpen: boolean) => {
      if (isOpen) setPanel(null);
      setOpenMenu((current) => (isOpen ? menu : current === menu ? null : current));
    },
  });

  const chapterMenu: PlayerChromeMenu = {
    buttonLabel: t('player.chapters.button'),
    label: t('player.chapters.button'),
    options: chapters.map((chapter, index) => ({
      id: String(index),
      label: chapterLabel(t, chapter, index, durationMs),
    })),
    selectedKey: String(chapterIndex),
    reselect: true,
    onSelect: (id) => {
      const chapter = chapters[Number(id)];
      if (chapter) seekTo(chapter.startMs);
    },
    ...menuProps('chapters'),
  };

  // Forward: the next chapter, after the last one the next episode (V-2).
  const forwardStep =
    nextChapterStart(chapters, currentMs) !== null
      ? { label: t('player.chrome.nextChapter'), onPress: nextChapter }
      : next && onNextEpisode
        ? { label: t('player.chrome.nextEpisode'), onPress: playNext, isDisabled: ended !== null }
        : undefined;

  // Head and the line above the timeline (V-1): an episode shows its series with the episode
  // under it, a film its year and genres.
  const item = info.item;
  const code = episodeCode(t, item);
  const episode = item.kind === 'episode';
  const seriesTitle = detail?.series?.title ?? item.seriesTitle;
  const year = detail?.year ?? item.year;
  const genres =
    detail && detail.genres.length > 0
      ? new Intl.ListFormat(language, { type: 'unit', style: 'short' }).format(detail.genres)
      : undefined;
  const heading = episode
    ? {
        title: seriesTitle ?? item.title,
        logoUrl: imageUrl(series.data?.images.logo),
        subtitle: code ? t('player.chrome.episodeLine', { code, title: item.title }) : item.title,
      }
    : {
        title: item.title,
        logoUrl: imageUrl(detail?.images.logo),
        subtitle:
          year !== undefined && genres
            ? t('player.chrome.filmLine', { year, genres })
            : (genres ?? (year !== undefined ? String(year) : undefined)),
      };
  const chapter = chapters[chapterIndex];
  const chapterName = chapter
    ? (chapter.title ?? t('player.chapters.untitled', { n: chapterIndex + 1 }))
    : undefined;
  const lineBase = episode ? (seriesTitle ?? item.title) : item.title;
  const lineLabel = chapterName
    ? t('player.chrome.lineChapter', { title: lineBase, chapter: chapterName })
    : lineBase;

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
      data-controls={hidden ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
      onWheel={(event) => {
        poke();
        onWheel(event);
      }}
    >
      {/* Without a text subtitle track there is nothing to caption (W-4: bitmap tracks are burned in).
          A text track stays hidden: the subtitle layer draws its cues (11.10). */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={video}
        className={styles.video}
        src={engine === 'hlsjs' ? undefined : apiUrl(info.streamUrl)}
        preload="auto"
        playsInline
        data-subtitle-color={subtitleStyle.color}
        data-subtitle-background={subtitleStyle.background}
        onPointerUp={(event) => {
          if (event.button === 0) togglePlay();
        }}
        onDoubleClick={toggleFullscreen}
      >
        {info.subtitleUrl && (
          <track
            ref={subtitleTrackElement}
            kind="subtitles"
            src={apiUrl(info.subtitleUrl)}
            srcLang={subtitle?.language}
            label={
              subtitle
                ? (subtitle.title ?? languageName(subtitle.language, language) ?? '')
                : undefined
            }
          />
        )}
      </video>

      <SubtitleLayer texts={cueTexts} style={subtitleStyle} raised={!hidden} />

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
            <AriaButton ref={focusOnMount} variant="secondary" icon="back" onPress={onBack}>
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

      <PlayerChrome
        title={heading.title}
        logoUrl={heading.logoUrl}
        headingDetail={heading.subtitle}
        lineLabel={lineLabel}
        onBack={onBack}
        hidden={hidden}
        currentMs={currentMs}
        durationMs={durationMs}
        buffered={buffered}
        rate={rate}
        timeDisplay={timeDisplay}
        onTimeDisplayChange={(display) => setPrefs({ timeDisplay: display })}
        chapters={chapters}
        segments={info.segments}
        onSeek={seekTo}
        onPreviousChapter={hasChapters ? previousChapter : undefined}
        onNextChapter={hasChapters ? nextChapter : undefined}
        playing={playing}
        onTogglePlay={togglePlay}
        onSkip={seekBy}
        playButtonRef={playRef}
        previous={
          hasChapters
            ? { label: t('player.chrome.previousChapter'), onPress: previousChapter }
            : undefined
        }
        next={forwardStep}
        transportDisabled={switching || ended !== null}
        volume={volume.volume}
        muted={volume.muted}
        onVolumeChange={volume.setVolume}
        onToggleMute={volume.toggleMute}
        volumeBoost={webAudio}
        fullscreen={fullscreen}
        onToggleFullscreen={toggleFullscreen}
        pictureInPicture={
          document.pictureInPictureEnabled
            ? { active: pictureInPicture, onToggle: togglePictureInPicture }
            : undefined
        }
        subtitles={
          canSwitchTracks
            ? panelButton(
                'subtitle',
                t('player.panel.subtitleButton', { language: subtitleWord }),
                subtitleWord,
              )
            : undefined
        }
        audio={
          canSwitchTracks
            ? panelButton(
                'audio',
                t('player.panel.audioButton', { language: audioWord }),
                audioWord,
              )
            : undefined
        }

        chapterMenu={hasChapters ? chapterMenu : undefined}
        settings={{
          label: t('player.settings.title'),
          expanded: panel === 'settings',
          onPress: () => (panel === 'settings' ? closePanel() : openSettings('main', false)),
          isDisabled: ended !== null,
        }}
        portalContainer={portal ?? undefined}
        helpOpen={helpOpen}
        onToggleHelp={() => togglePanel('help')}
      />
      {helpOpen && <ShortcutHelp onClose={closePanel} />}
      {panel === 'settings' && (
        <SettingsPanel
          page={settingsView.page}
          direct={settingsView.direct}
          onPage={(page) => setSettingsView({ page, direct: false })}
          onClose={closePanel}
          quality={hasQualities && !switching ? qualityChoice : undefined}
          version={hasVersions && !switching ? versionChoice : undefined}
          rate={rate}
          onRate={setRate}
          streamInfo={onToggleInfo ? { open: infoOpen, onToggle: onToggleInfo } : undefined}
          subtitleStyle={{ value: subtitleStyle, onChange: setSubtitleStyle }}
          nightMode={webAudio ? { on: nightMode, onChange: setNightMode } : undefined}
        />
      )}
      {panel === 'audio' && (
        <AudioPanel
          tracks={tracks.audio}
          profileLanguage={profileLanguage}
          selectedIndex={audioIndex}
          converts={converts}
          onSelect={(index) => switchPlayback({ audioTrack: index })}
          onClose={closePanel}
        />
      )}
      {panel === 'subtitle' && (
        <SubtitlePanel
          tracks={tracks.subtitles}
          profileLanguage={profileLanguage}
          selectedId={info.selection.subtitleTrack}
          onSelect={(id) => switchPlayback({ subtitleTrack: id })}
          onClose={closePanel}
          subtitleStyle={{ value: subtitleStyle, onChange: setSubtitleStyle }}
        />
      )}

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
