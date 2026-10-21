import type { Chapter } from '@slouchcat/api-client';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type RefObject,
} from 'react';
import { useTranslation } from 'react-i18next';
import type { AudioPlayer, AudioPlayerMenu } from '@/ui/aria';
import {
  chapterPosition,
  clampRate,
  nextChapterStart,
  previousChapterStart,
  sleepChoiceOf,
  sleepDue,
  sleepRemainingMs,
  SLEEP_OFF,
  startSleep,
  stepRate,
  type SleepChoice,
  type SleepTimer,
} from './audio';

/** ←/→ jump this far; the −15 s button as well. */
export const AUDIO_SEEK_S = 15;
const VOLUME_STEP = 0.1;
const OSD_MS = 1200;
const SLEEP_TICK_MS = 1000;

type PlayerProps = ComponentProps<typeof AudioPlayer>;

/** What the hook hands to the AudioPlayer; the page adds title, cover, time and transport. */
export type AudioControlProps = Pick<
  PlayerProps,
  | 'chapter'
  | 'rate'
  | 'volume'
  | 'muted'
  | 'sleepChoice'
  | 'sleepRemainingMs'
  | 'openMenu'
  | 'onOpenMenuChange'
  | 'onSkip'
  | 'onPreviousChapter'
  | 'onNextChapter'
  | 'onRate'
  | 'onSleep'
  | 'onVolume'
  | 'onToggleMute'
  | 'status'
>;

interface AudioControlsOptions {
  /** The <audio> element (the offline player's media element in audio mode). */
  media: RefObject<HTMLMediaElement | null>;
  chapters: readonly Chapter[];
  durationMs: number;
  /** Position on the title's timeline (ms), as the page shows it. */
  positionMs: number;
  /** The element's position on the title's timeline right now (ms), for jumps and the timer. */
  positionOf: () => number;
  /**
   * Speed to start with: the profile's last one for this title (H-3), else 1. A later value (the
   * title's detail arrived after the start) applies quietly until the listener picks a speed.
   */
  initialRate: number;
  /** False once the playback was ended (limit, server): only the back keys still work. */
  active: boolean;
  /** False: no key map and no element handling (the offline player playing a video). */
  enabled?: boolean;
  seekTo: (ms: number) => void;
  togglePlay: () => void;
  onBack: () => void;
  /** Every change of the speed (the offline player stores it per download). */
  onRateChange?: (rate: number) => void;
}

// The element belongs to the page: the hook reads it and sets its speed and volume through these
// (the page's ref arrives as an argument, which the hooks lint cannot tell from other values).
const elementOf = (media: RefObject<HTMLMediaElement | null>) => media.current;

function setElementRate(element: HTMLMediaElement, rate: number): void {
  element.defaultPlaybackRate = rate;
  element.playbackRate = rate;
}

function setElementVolume(element: HTMLMediaElement, volume: number, muted: boolean): void {
  element.volume = volume;
  element.muted = muted;
}

const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

/** Keys inside a menu belong to the menu (arrows, Enter, Escape). */
const inMenu = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  target.closest('[role="menu"], [role="menuitemradio"], [role="menuitem"]') !== null;

const isApple = () => /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);

/**
 * `[` and `]` typed through AltGr (Swiss and German layouts on Windows and Linux; Windows reports
 * Ctrl+Alt with it) or Option (macOS): the speed keys, not a shortcut.
 */
const isTypedSpeedKey = (event: KeyboardEvent) =>
  (event.key === '[' || event.key === ']') &&
  !event.metaKey &&
  (event.getModifierState('AltGraph') || (isApple() && event.altKey && !event.ctrlKey));

/**
 * State and keys of the audio player (design 4), shared by the player and the offline player:
 * speed (0.5 … 3, presets and ±0.1, kept on the element across reloads of its source), volume,
 * the sleep timer (pauses the element, which reports the progress), the menus and the key map:
 * Space/Enter/K play-pause, ←/→ 15 s, Shift+←/→ chapters, ↑/↓ volume, [ and ] speed,
 * C chapters, T sleep timer, Escape/Backspace back.
 */
export function useAudioControls({
  media,
  chapters,
  durationMs,
  positionMs,
  positionOf,
  initialRate,
  active,
  enabled = true,
  seekTo,
  togglePlay,
  onBack,
  onRateChange,
}: AudioControlsOptions): AudioControlProps {
  const { t, i18n } = useTranslation();
  const [rate, setRate] = useState(() => clampRate(initialRate));
  const [startRate, setStartRate] = useState(initialRate);
  const [ratePicked, setRatePicked] = useState(false);
  if (initialRate !== startRate) {
    setStartRate(initialRate);
    if (!ratePicked) setRate(clampRate(initialRate));
  }
  const [volume, setVolume] = useState(1);
  const [muted, setMuted] = useState(false);
  const [sleep, setSleep] = useState<SleepTimer>(SLEEP_OFF);
  const [now, setNow] = useState(() => Date.now());
  const [openMenu, setOpenMenu] = useState<AudioPlayerMenu | null>(null);
  const [osd, setOsd] = useState<string | null>(null);
  const osdTimer = useRef<number | undefined>(undefined);
  const position = useRef(positionOf);
  const rateChanged = useRef(onRateChange);
  useEffect(() => {
    position.current = positionOf;
    rateChanged.current = onRateChange;
  });

  const showOsd = useCallback((text: string) => {
    setOsd(text);
    window.clearTimeout(osdTimer.current);
    osdTimer.current = window.setTimeout(() => setOsd(null), OSD_MS);
  }, []);
  useEffect(() => () => window.clearTimeout(osdTimer.current), []);

  const rateText = useCallback(
    (value: number) =>
      t('player.audio.rate', {
        rate: new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(value),
      }),
    [i18n.language, t],
  );

  const changeRate = useCallback(
    (next: number) => {
      const value = clampRate(next);
      setRate(value);
      setRatePicked(true);
      rateChanged.current?.(value);
      showOsd(t('player.audio.rateOsd', { rate: rateText(value) }));
    },
    [rateText, showOsd, t],
  );

  // The speed lives on the element; loading a source resets playbackRate to the default rate.
  useEffect(() => {
    const element = media.current;
    if (!enabled || !element) return;
    const apply = () => setElementRate(element, rate);
    apply();
    element.addEventListener('loadedmetadata', apply);
    return () => element.removeEventListener('loadedmetadata', apply);
  }, [enabled, media, rate]);

  useEffect(() => {
    const element = media.current;
    if (!enabled || !element) return;
    const onVolume = () => {
      setVolume(element.volume);
      setMuted(element.muted);
    };
    onVolume();
    element.addEventListener('volumechange', onVolume);
    return () => element.removeEventListener('volumechange', onVolume);
  }, [enabled, media]);

  const changeVolume = useCallback(
    (next: number) => {
      const element = elementOf(media);
      if (!element) return;
      const value = Math.round(Math.min(1, Math.max(0, next)) * 100) / 100;
      setElementVolume(element, value, value === 0);
      showOsd(t('player.volume', { percent: Math.round(value * 100) }));
    },
    [media, showOsd, t],
  );
  const toggleMute = useCallback(() => {
    const element = elementOf(media);
    if (element) setElementVolume(element, element.volume, !element.muted);
  }, [media]);

  // The sleep timer runs on the device: a tick for the time it shows, and the element's time
  // updates for the end of a chapter. When it fires, the element pauses (progress is reported).
  useEffect(() => {
    const element = media.current;
    if (!enabled || !element || sleep.kind === 'off') return;
    let exact: number | undefined;
    const fire = () => {
      setSleep(SLEEP_OFF);
      if (!element.paused) element.pause();
      showOsd(t('player.audio.sleep.done'));
    };
    const check = () => {
      const current = Date.now();
      const at = position.current();
      setNow(current);
      window.clearTimeout(exact);
      if (sleepDue(sleep, current, at)) {
        fire();
        return;
      }
      // Time updates come about every 250 ms of wall time: the last second of a chapter ends on
      // a timer of its own, at the element's speed, so nothing of the next chapter plays.
      const left = sleepRemainingMs(sleep, current, at, element.playbackRate);
      if (sleep.kind === 'chapter' && !element.paused && left !== null && left < SLEEP_TICK_MS) {
        exact = window.setTimeout(fire, left);
      }
    };
    const stop = () => window.clearTimeout(exact);
    // A jump to another chapter (seek bar, chapter keys or menu) moves the timer to its end.
    const onSeeking = () => {
      stop();
      if (sleep.kind !== 'chapter') return;
      const next = startSleep('chapter', Date.now(), chapters, position.current(), durationMs);
      if (next.kind === 'chapter' && next.endMs !== sleep.endMs) setSleep(next);
    };
    const timer = window.setInterval(check, SLEEP_TICK_MS);
    element.addEventListener('timeupdate', check);
    element.addEventListener('seeking', onSeeking);
    element.addEventListener('pause', stop);
    return () => {
      window.clearInterval(timer);
      stop();
      element.removeEventListener('timeupdate', check);
      element.removeEventListener('seeking', onSeeking);
      element.removeEventListener('pause', stop);
    };
  }, [chapters, durationMs, enabled, media, showOsd, sleep, t]);

  const chooseSleep = useCallback(
    (choice: string) => {
      const current = Date.now();
      setNow(current);
      const timer = startSleep(
        choice as SleepChoice,
        current,
        chapters,
        position.current(),
        durationMs,
      );
      setSleep(timer);
      showOsd(
        timer.kind === 'off'
          ? t('player.audio.sleep.cancelled')
          : t('player.audio.sleep.set', {
              label:
                timer.kind !== 'chapter'
                  ? t('player.audio.sleep.minutes', { count: timer.minutes })
                  : chapters.length > 0
                    ? t('player.audio.sleep.chapter')
                    : t('player.audio.sleep.end'),
            }),
      );
    },
    [chapters, durationMs, showOsd, t],
  );

  const skip = useCallback(
    (seconds: number) => seekTo(position.current() + seconds * 1000),
    [seekTo],
  );
  const previousChapter = useCallback(
    () => seekTo(previousChapterStart(chapters, position.current())),
    [chapters, seekTo],
  );
  const nextChapter = useCallback(() => {
    const target = nextChapterStart(chapters, position.current());
    if (target !== null) seekTo(target);
  }, [chapters, seekTo]);

  useEffect(() => {
    if (!enabled) return;
    const open = (menu: AudioPlayerMenu, available: boolean, event: KeyboardEvent) => {
      if (!available) return;
      event.preventDefault();
      setOpenMenu(menu);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.altKey || event.ctrlKey || event.metaKey) && !isTypedSpeedKey(event)) return;
      if (inMenu(event.target)) {
        // Backspace is the remote's back key (C-4): in a menu it closes the menu, like Escape.
        if (event.key === 'Backspace') {
          event.preventDefault();
          setOpenMenu(null);
        }
        return;
      }
      // The volume slider moves with the arrows itself.
      if (event.target instanceof HTMLInputElement && event.key.startsWith('Arrow')) return;
      const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
      if (!active && key !== 'Escape' && key !== 'Backspace') return;
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
          if (event.shiftKey) previousChapter();
          else skip(-AUDIO_SEEK_S);
          return;
        case 'ArrowRight':
          event.preventDefault();
          if (event.shiftKey) nextChapter();
          else skip(AUDIO_SEEK_S);
          return;
        case 'ArrowUp':
          event.preventDefault();
          changeVolume((media.current?.volume ?? 1) + VOLUME_STEP);
          return;
        case 'ArrowDown':
          event.preventDefault();
          changeVolume((media.current?.volume ?? 1) - VOLUME_STEP);
          return;
        case '[':
          event.preventDefault();
          changeRate(stepRate(rate, -1));
          return;
        case ']':
          event.preventDefault();
          changeRate(stepRate(rate, 1));
          return;
        case 'c':
          open('chapters', chapters.length > 0, event);
          return;
        case 't':
          open('sleep', true, event);
          return;
        case 'Escape':
        case 'Backspace':
          event.preventDefault();
          onBack();
          return;
        default:
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [
    active,
    changeRate,
    changeVolume,
    chapters.length,
    enabled,
    media,
    nextChapter,
    onBack,
    previousChapter,
    rate,
    skip,
    togglePlay,
  ]);

  return {
    chapter: chapterPosition(chapters, positionMs, durationMs),
    rate,
    volume,
    muted,
    sleepChoice: sleepChoiceOf(sleep),
    sleepRemainingMs: sleepRemainingMs(sleep, now, positionMs, rate),
    openMenu,
    onOpenMenuChange: setOpenMenu,
    onSkip: skip,
    onPreviousChapter: previousChapter,
    onNextChapter: nextChapter,
    onRate: changeRate,
    onSleep: chooseSleep,
    onVolume: changeVolume,
    onToggleMute: toggleMute,
    status: osd,
  };
}
