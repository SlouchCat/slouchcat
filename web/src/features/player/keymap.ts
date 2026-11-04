import { VOLUME_STEP } from '@/ui/aria';

/** ←/→ jump this far (the −10 s button as well). */
export const SEEK_STEP_S = 10;
/** `[` and `]` change the speed by this much, within RATE_MIN … RATE_MAX (V-7, V-12). */
export const RATE_STEP = 0.25;
export const RATE_MIN = 0.5;
export const RATE_MAX = 2;

/** What a key asks the video player to do (concept 13, 11.2 «Tastenkarte»). */
export type PlayerKeyAction =
  /** Space, Enter, K. `activates`: a focused button takes Space and Enter itself. */
  | { kind: 'playPause'; activates: boolean }
  | { kind: 'seek'; seconds: number }
  | { kind: 'volume'; delta: number }
  | { kind: 'chapter'; step: -1 | 1 }
  /** 0–9: to 0–90 % of the title. */
  | { kind: 'jump'; fraction: number }
  | { kind: 'rate'; delta: number }
  | {
      kind:
        | 'mute'
        | 'tracks'
        | 'subtitles'
        | 'chapters'
        | 'quality'
        | 'fullscreen'
        | 'info'
        | 'skipIntro'
        | 'nextEpisode'
        | 'help'
        | 'back';
    };

type KeyInput = Pick<KeyboardEvent, 'key' | 'shiftKey' | 'altKey' | 'ctrlKey' | 'metaKey'> &
  Partial<Pick<KeyboardEvent, 'getModifierState'>>;

const isApple = () => /Mac|iPhone|iPad|iPod/.test(navigator.userAgent);

/**
 * `[` and `]` typed through AltGr (Swiss and German layouts on Windows and Linux; Windows reports
 * Ctrl+Alt with it) or Option (macOS): the speed keys, not a shortcut.
 */
export function isTypedSpeedKey(event: KeyInput): boolean {
  return (
    (event.key === '[' || event.key === ']') &&
    !event.metaKey &&
    (event.getModifierState?.('AltGraph') === true || (isApple() && event.altKey && !event.ctrlKey))
  );
}

/**
 * Alt, Ctrl or Meta make a key the browser's (a shortcut), except for the speed keys typed with
 * AltGr or Option.
 */
export function hasShortcutModifier(event: KeyInput): boolean {
  return (event.altKey || event.ctrlKey || event.metaKey) && !isTypedSpeedKey(event);
}

const letters: Record<string, PlayerKeyAction> = {
  k: { kind: 'playPause', activates: false },
  m: { kind: 'mute' },
  t: { kind: 'tracks' },
  u: { kind: 'subtitles' },
  c: { kind: 'chapters' },
  q: { kind: 'quality' },
  f: { kind: 'fullscreen' },
  i: { kind: 'info' },
  s: { kind: 'skipIntro' },
  n: { kind: 'nextEpisode' },
  '?': { kind: 'help' },
  '[': { kind: 'rate', delta: -RATE_STEP },
  ']': { kind: 'rate', delta: RATE_STEP },
};

/**
 * The action of a key, or null for a key the player leaves alone (and every key with Alt, Ctrl or
 * Meta: those belong to the browser, but `[` and `]` typed with AltGr or Option count). Letters count in either case. Which zone has the focus
 * decides elsewhere whether a key reaches the key map at all: the button row keeps the arrows to
 * itself (PlayerChrome), a menu or a panel its own keys.
 */
export function playerKeyAction(event: KeyInput): PlayerKeyAction | null {
  if (hasShortcutModifier(event)) return null;
  const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;
  switch (key) {
    case ' ':
    case 'Enter':
      return { kind: 'playPause', activates: true };
    case 'ArrowLeft':
      return event.shiftKey
        ? { kind: 'chapter', step: -1 }
        : { kind: 'seek', seconds: -SEEK_STEP_S };
    case 'ArrowRight':
      return event.shiftKey ? { kind: 'chapter', step: 1 } : { kind: 'seek', seconds: SEEK_STEP_S };
    case 'ArrowUp':
      return event.shiftKey ? null : { kind: 'volume', delta: VOLUME_STEP };
    case 'ArrowDown':
      return event.shiftKey ? null : { kind: 'volume', delta: -VOLUME_STEP };
    case 'Escape':
    case 'Backspace':
      return { kind: 'back' };
    default:
      if (/^[0-9]$/.test(key)) return { kind: 'jump', fraction: Number(key) / 10 };
      return letters[key] ?? null;
  }
}

/** The speed after `[` or `]`: in steps of RATE_STEP, within RATE_MIN … RATE_MAX. */
export function stepPlaybackRate(rate: number, delta: number): number {
  const next = Math.round((rate + delta) / RATE_STEP) * RATE_STEP;
  return Math.min(RATE_MAX, Math.max(RATE_MIN, next));
}
