import { useEffect, useRef, type RefObject } from 'react';
import { hasShortcutModifier, playerKeyAction } from './keymap';

/** Focused buttons and links activate themselves with Enter and Space. */
const isActivatable = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('button, a, [role="button"]') !== null;

/** Typing in a text field is not a shortcut (11.2: panels with text input keep the letters). */
const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

/** Enter and Space inside a panel belong to its rows; the letters still reach the key map. */
const inPanel = (target: EventTarget | null) =>
  target instanceof HTMLElement && target.closest('[role="dialog"]') !== null;

/**
 * What the keys do. A missing action means the player cannot do it now (no chapters, no quality
 * to switch, the server ended the playback): the key is left to the browser.
 */
export interface PlayerKeyActions {
  togglePlay: () => void;
  seekBy: (seconds: number) => void;
  seekTo: (ms: number) => void;
  changeVolume: (delta: number) => void;
  changeRate: (delta: number) => void;
  toggleMute: () => void;
  toggleFullscreen: () => void;
  previousChapter?: () => void;
  nextChapter?: () => void;
  toggleInfo?: () => void;
  /** T and U: the sound or the subtitle panel. */
  openTracks?: (kind: 'audio' | 'subtitle') => void;
  /** Q. */
  openQuality?: () => void;
  /** C. */
  openChapters?: () => void;
  /** S, inside the intro. */
  skipIntro?: () => void;
  /** N. */
  playNext?: () => void;
  /** ?. */
  toggleHelp: () => void;
  /**
   * Escape and Backspace: closes what is open (a panel, the stream info) and says whether there
   * was something; otherwise the key leaves the player (`back`), except in full screen.
   */
  closeOpen: () => boolean;
  back: () => void;
}

interface PlayerKeysOptions {
  /** The controls faded out: no zone (11.2), the arrows and OK act whatever has the focus. */
  hidden: boolean;
  /** The next-episode countdown runs (W-9). */
  countdown?: {
    /** The countdown's own section: Enter and Space there do not cancel it. */
    panel: RefObject<HTMLElement | null>;
    cancel: () => void;
    playNext: () => void;
  };
  /** The player's length (ms), for 0–9. */
  durationMs: number;
  /** Every key shows the controls. */
  onKey: () => void;
  /**
   * A navigation away from the player runs (to a person, another episode, back) and the next
   * page is still loading. The player is still on screen, but the keys are no longer its own:
   * Escape and Backspace go back as on the next page, without closing what is open here, and
   * the other keys do nothing.
   */
  leaving?: () => boolean;
  actions: PlayerKeyActions;
}

/**
 * The key map of the video player (concept 13, 11.2; W-1, C-4, W-4, W-9), shared with the offline
 * player (step 6). Two listeners on window:
 *
 * Capture phase, before any control sees the key:
 * - Every key shows the controls, also one that a control keeps to itself (the button row, the
 *   volume slider).
 * - During the next-episode countdown any key but Tab cancels it and does nothing else; N plays
 *   the next episode. Enter and Space on a focused button (play/pause on the remote's OK) still
 *   press it, on the countdown's own buttons without cancelling.
 * - With the controls faded out there is no zone: the arrows seek and change the volume and
 *   Enter, Space and K play and pause whatever has the focus, and the bar comes back.
 *
 * Bubble phase, the keys no zone kept for itself: Space/Enter/K play and pause, ←/→ 10 s (with
 * Shift a chapter), ↑/↓ volume, M mute, T sound, U subtitles, C chapters, Q quality, [ ] speed,
 * 0–9 jump to 0–90 %, F full screen, I stream info, S skip the intro, N next episode, ? help,
 * Escape/Backspace close what is open, else back to the detail page.
 *
 * While a navigation away is loading (`leaving`) only Escape and Backspace act, and they go back.
 *
 * The latest options are read when a key arrives, so the listeners are bound once.
 */
export function usePlayerKeys(options: PlayerKeysOptions): void {
  const latest = useRef(options);
  useEffect(() => {
    latest.current = options;
  });

  useEffect(() => {
    const onCapture = (event: KeyboardEvent) => {
      if (hasShortcutModifier(event) || latest.current.leaving?.()) return;
      const { hidden, countdown, onKey, actions } = latest.current;
      onKey();
      const consume = () => {
        event.preventDefault();
        event.stopPropagation();
      };
      if (countdown) {
        const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
        if (key === 'n') {
          consume();
          countdown.playNext();
          return;
        }
        // Tab moves between the controls; modifier keys alone do nothing.
        if (key === 'Tab' || ['Shift', 'Control', 'Alt', 'Meta'].includes(key)) return;
        const activates = key === 'Enter' || key === ' ';
        const inCountdown =
          event.target instanceof Node && countdown.panel.current?.contains(event.target);
        if (activates && inCountdown) return;
        countdown.cancel();
        if (activates && isActivatable(event.target)) return;
        consume();
        return;
      }
      if (!hidden || event.shiftKey) return;
      const action = playerKeyAction(event);
      if (action?.kind === 'playPause') {
        // Enter and Space on the remote's OK play and pause; the focused button stays as it is.
        consume();
        actions.togglePlay();
      } else if (action?.kind === 'seek') {
        consume();
        actions.seekBy(action.seconds);
      } else if (action?.kind === 'volume') {
        consume();
        actions.changeVolume(action.delta);
      }
    };

    const onKeyDown = (event: KeyboardEvent) => {
      if (hasShortcutModifier(event)) return;
      const { countdown, durationMs, actions, leaving } = latest.current;
      const away = leaving?.() ?? false;
      // The capture listener has handled the key.
      if ((countdown && !away) || isEditable(event.target)) return;
      const action = playerKeyAction(event);
      if (!action || (away && action.kind !== 'back')) return;
      /** Runs `run` if the player can do it now; the key is the player's then. */
      const act = (run: (() => void) | undefined) => {
        if (!run) return;
        event.preventDefault();
        run();
      };
      switch (action.kind) {
        case 'playPause':
          // Focused buttons and the rows of a panel activate themselves; everywhere else the key
          // toggles playback.
          if (action.activates && (isActivatable(event.target) || inPanel(event.target))) return;
          act(actions.togglePlay);
          return;
        case 'seek':
          act(() => actions.seekBy(action.seconds));
          return;
        case 'volume':
          act(() => actions.changeVolume(action.delta));
          return;
        case 'chapter':
          act(action.step < 0 ? actions.previousChapter : actions.nextChapter);
          return;
        case 'jump':
          // The player's length: the file's until the element knows its own (HLS may report
          // Infinity while it loads).
          if (durationMs > 0) act(() => actions.seekTo(action.fraction * durationMs));
          return;
        case 'rate':
          act(() => actions.changeRate(action.delta));
          return;
        case 'mute':
          act(actions.toggleMute);
          return;
        case 'fullscreen':
          act(actions.toggleFullscreen);
          return;
        case 'info':
          act(actions.toggleInfo);
          return;
        case 'tracks':
        case 'subtitles': {
          const { openTracks } = actions;
          const kind = action.kind === 'tracks' ? 'audio' : 'subtitle';
          act(openTracks && (() => openTracks(kind)));
          return;
        }
        case 'quality':
          act(actions.openQuality);
          return;
        case 'chapters':
          act(actions.openChapters);
          return;
        case 'skipIntro':
          act(actions.skipIntro);
          return;
        case 'nextEpisode':
          act(actions.playNext);
          return;
        case 'help':
          act(actions.toggleHelp);
          return;
        case 'back':
          // What is open first (a panel's own keys close it while the focus is in it), then back
          // to the detail page; in full screen the browser leaves full screen first. Leaving, the
          // player is gone in a moment: the key is the next page's back.
          if (!away && actions.closeOpen()) {
            event.preventDefault();
            return;
          }
          if (document.fullscreenElement) return;
          act(actions.back);
          return;
        default:
      }
    };

    window.addEventListener('keydown', onCapture, true);
    window.addEventListener('keydown', onKeyDown);
    return () => {
      window.removeEventListener('keydown', onCapture, true);
      window.removeEventListener('keydown', onKeyDown);
    };
  }, []);
}
