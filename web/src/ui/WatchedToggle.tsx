import { Icon } from './Icon';
import styles from './WatchedToggle.module.css';

/**
 * The check mark «gesehen» of a title (concept 20 F-12): `none`, `half` (some episodes of a
 * season, series or podcast) or `full`. A single title knows only `none` and `full`.
 */
export type WatchedState = 'none' | 'half' | 'full';

const PRESSED = { none: false, half: 'mixed', full: true } as const;

interface WatchedToggleProps {
  state: WatchedState;
  /** The spoken name, the same in every state ("Gesehen: Die lange Nacht"). */
  label: string;
  /** What a press does, as the pointer's hint ("Als gesehen markieren"). */
  title?: string;
  /**
   * `cover`: a small disc over a picture, like the pen (poster cards, season covers). `row`: a ring
   * at the far right of a list row (episodes).
   */
  appearance?: 'cover' | 'row';
  /**
   * A display only (offline: the server decides, rule 12): no button, the state as an image named
   * by `stateLabel`; `none` shows nothing on a cover and an empty ring in a row.
   */
  readOnly?: boolean;
  /** The image's name while `readOnly` ("Gesehen", "Teilweise gesehen"). */
  stateLabel?: string;
  /** A change in flight: still focusable, a press does nothing (`aria-disabled`). */
  busy?: boolean;
  onPress?: () => void;
  /** For a row with one Tab stop (the season shelf's roving tabindex). */
  excludeFromTabOrder?: boolean;
}

/**
 * The check mark «gesehen» as a toggle (concept 20 P-5): empty, half filled or filled with its
 * check in the accent, `aria-pressed` false, `mixed` or true. A plain button, not a spatial item:
 * Tab reaches it after the card's link and pen, the arrow keys pass it by. What a press does (set,
 * ask, remove) is the page's business.
 */
export function WatchedToggle({
  state,
  label,
  title,
  appearance = 'cover',
  readOnly = false,
  stateLabel,
  busy = false,
  onPress,
  excludeFromTabOrder,
}: WatchedToggleProps) {
  const className = [styles.toggle, styles[appearance], styles[state]].join(' ');
  const mark = (
    <span className={styles.disc} aria-hidden="true">
      <Icon name="check" size="sm" />
    </span>
  );
  if (readOnly) {
    if (state === 'none') {
      return appearance === 'row' ? (
        <span className={className} aria-hidden="true">
          {mark}
        </span>
      ) : null;
    }
    return (
      <span className={className} role="img" aria-label={stateLabel ?? label} data-watched={state}>
        {mark}
      </span>
    );
  }
  return (
    <button
      type="button"
      className={className}
      aria-label={label}
      aria-pressed={PRESSED[state]}
      aria-disabled={busy || undefined}
      title={title}
      tabIndex={excludeFromTabOrder ? -1 : undefined}
      data-watched={state}
      onClick={() => {
        if (!busy) onPress?.();
      }}
    >
      {mark}
    </button>
  );
}
