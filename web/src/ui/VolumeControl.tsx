import { tokens } from '@slouchcat/design-tokens';
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type FocusEvent,
  type KeyboardEvent,
  type PointerEvent,
  type Ref,
} from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { Icon } from './Icon';
import styles from './VolumeControl.module.css';

/** One press of an arrow key on the slider. */
export const VOLUME_STEP = 0.05;
/** The top of the slider with boost (Web Audio gain, F-5). */
export const BOOST_MAX = 1.5;
/** How long a value asked for may stay unanswered before the next step starts from the shown one. */
const PENDING_MS = 1000;
/** The popover stays open this long after the pointer left it (motion.duration.chrome, E2). */
const CLOSE_DELAY_MS = Number.parseInt(tokens.motion.duration.chrome, 10);
/** Room between the button and the popover, and the popover and the window's edge (px). */
const GAP_PX = 4;
const EDGE_PX = 8;

/** A volume in 0 … max, rounded to hundredths (no 0.15000000000000002). */
export function clampVolume(volume: number, max: number): number {
  if (!Number.isFinite(volume)) return 0;
  return Math.round(Math.min(max, Math.max(0, volume)) * 100) / 100;
}

interface VolumeControlProps {
  /** 0 … 1, with `boost` 0 … 1.5. */
  volume: number;
  muted: boolean;
  onVolumeChange: (volume: number) => void;
  onToggleMute: () => void;
  /**
   * The slider goes up to 150 % and warns above 100 % (F-5). Off where the page has no gain node,
   * and in Safari (11.10: hidden there until the Web Audio spike says otherwise).
   */
  boost?: boolean;
  /** Only the mute button: Safari on the iPhone does not let a page set the volume (11.2). */
  muteOnly?: boolean;
  isDisabled?: boolean;
  /**
   * Tab index of the mute button, for a row with a roving tab index (concept 13, 11.2): only the
   * button is a member of the row. The slider follows it, so it is a Tab stop right after the
   * button while the button is the row's stop, and out of the Tab order otherwise.
   */
  tabIndex?: number;
  buttonRef?: Ref<HTMLButtonElement>;
  sliderRef?: Ref<HTMLDivElement>;
  /**
   * Where the popover opens with a pointer: `side` beside the button (right, else left, where the
   * window has room), `above` over it (else below): the mini player at the bottom of the page.
   */
  placement?: 'side' | 'above';
  /** The slider always open in the row, as on a touch screen (concept 19 E2). */
  inline?: boolean;
}

function subscribeHover(onChange: () => void): () => void {
  if (typeof window.matchMedia !== 'function') return () => undefined;
  const query = window.matchMedia('(hover: hover)');
  query.addEventListener('change', onChange);
  return () => query.removeEventListener('change', onChange);
}

/** Whether the device has a pointer that hovers (and the browser has the popover API). */
const hoverPointer = () =>
  typeof window.matchMedia === 'function' &&
  window.matchMedia('(hover: hover)').matches &&
  typeof HTMLElement.prototype.showPopover === 'function';

/** Hands the element to a ref of the page as well (a module function: props stay unchanged). */
function passRef<E>(ref: Ref<E> | undefined, element: E | null) {
  if (typeof ref === 'function') ref(element);
  else if (ref) ref.current = element;
}

/**
 * Places the open popover in the top layer, fixed to the window: beside the button where the
 * window has room for it (right first), or over it (below where the top has no room).
 */
function placePopover(popover: HTMLElement, button: HTMLElement, placement: 'side' | 'above') {
  const anchor = button.getBoundingClientRect();
  const width = popover.offsetWidth;
  const height = popover.offsetHeight;
  const room = { width: window.innerWidth, height: window.innerHeight };
  const within = (value: number, size: number, total: number) =>
    Math.max(EDGE_PX, Math.min(value, total - size - EDGE_PX));
  let left: number;
  let top: number;
  if (placement === 'side') {
    left =
      anchor.right + GAP_PX + width <= room.width - EDGE_PX
        ? anchor.right + GAP_PX
        : anchor.left - GAP_PX - width;
    top = anchor.top + anchor.height / 2 - height / 2;
  } else {
    left = anchor.left + anchor.width / 2 - width / 2;
    top =
      anchor.top - GAP_PX - height >= EDGE_PX
        ? anchor.top - GAP_PX - height
        : anchor.bottom + GAP_PX;
  }
  popover.style.left = `${within(left, width, room.width)}px`;
  popover.style.top = `${within(top, height, room.height)}px`;
}

/**
 * Volume (V-3): a mute button and a slider. With a pointer that hovers the slider opens as a
 * popover over the neighbours while the pointer is over the control or the focus is in it, and
 * stays open CLOSE_DELAY_MS after the pointer left; Escape closes it (concept 19 E2, OE-2). It
 * lies in the top layer, so no `overflow` clips it and the row does not wrap; closed it stays in
 * the Tab order and for screen readers, hidden. On a touch screen (and with `inline`) it stays
 * open in the row and covers nothing. The slider moves
 * with ←/→ (5 %), PageUp/PageDown (10 %), Home and End, and keeps those keys from the page's key
 * map. ↑ and ↓ go on to the page: in the player's button row they change the zone (11.2), and the
 * players' key maps change the volume with them. The percentage shows beside it, above 100 % in
 * the warning colour with a sign.
 */
export function VolumeControl({
  volume,
  muted,
  onVolumeChange,
  onToggleMute,
  boost = false,
  muteOnly = false,
  isDisabled = false,
  tabIndex,
  buttonRef,
  sliderRef,
  placement = 'side',
  inline = false,
}: VolumeControlProps) {
  const { t } = useTranslation();
  const track = useRef<HTMLSpanElement>(null);
  const button = useRef<HTMLButtonElement | null>(null);
  const slider = useRef<HTMLDivElement | null>(null);
  const drawer = useRef<HTMLSpanElement>(null);
  const [dragging, setDragging] = useState(false);
  const pointer = useSyncExternalStore(subscribeHover, hoverPointer, () => false);
  const popover = pointer && !inline && !muteOnly;
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const closeTimer = useRef<number | undefined>(undefined);
  const open = popover && !dismissed && (hovered || focused || dragging);
  const openRef = useRef(open);
  const max = boost ? BOOST_MAX : 1;
  const shown = muted ? 0 : clampVolume(volume, max);
  const percent = Math.round(shown * 100);
  const boosted = shown > 1;
  const valueText = boosted
    ? t('player.volumeControl.boosted', { percent })
    : t('player.volume', { percent });

  // The page reports a new volume a moment later (the media element's volumechange), so two quick
  // presses would both step from the old value. The values asked for and not yet shown are kept,
  // with the value shown when the first of them was asked for: the next step starts from the last
  // one asked for until the page catches up. A shown value that is neither that start nor one of
  // them means the page set the volume itself (a key of its own, a clamp): the list is dropped.
  // The values arrive in the order they were asked for. A page that never answers (a muted
  // element, a value it rejects) must not keep the list alive: it goes when the slider loses the
  // focus and when the last value asked for is older than PENDING_MS.
  const pending = useRef<{ from: number; values: number[]; at: number }>({
    from: 0,
    values: [],
    at: 0,
  });
  const current = () => {
    const { from, values, at } = pending.current;
    if (Date.now() - at > PENDING_MS) values.length = 0;
    const reached = values.indexOf(shown);
    if (reached >= 0) {
      values.splice(0, reached + 1);
      pending.current.from = shown;
    } else if (shown !== from) values.length = 0;
    return values.at(-1) ?? shown;
  };
  const set = (next: number) => {
    current();
    const value = clampVolume(next, max);
    if (pending.current.values.length === 0) pending.current.from = shown;
    pending.current.values.push(value);
    pending.current.at = Date.now();
    onVolumeChange(value);
  };
  const positionAt = (event: PointerEvent<HTMLDivElement>) => {
    const rect = track.current?.getBoundingClientRect();
    if (!rect || rect.width === 0) return null;
    return Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)) * max;
  };
  const keys: Record<string, (from: number) => number> = {
    ArrowRight: (from) => from + VOLUME_STEP,
    ArrowLeft: (from) => from - VOLUME_STEP,
    PageUp: (from) => from + 2 * VOLUME_STEP,
    PageDown: (from) => from - 2 * VOLUME_STEP,
    Home: () => 0,
    End: () => max,
  };

  const buttonRefs = useCallback(
    (element: HTMLButtonElement | null) => {
      button.current = element;
      passRef(buttonRef, element);
    },
    [buttonRef],
  );
  const sliderRefs = useCallback(
    (element: HTMLDivElement | null) => {
      slider.current = element;
      passRef(sliderRef, element);
    },
    [sliderRef],
  );

  // The popover follows the open state. While open it keeps its place by the button, checked each
  // frame: the page scrolls, the window or the row changes, a late font moves the button.
  useLayoutEffect(() => {
    const element = drawer.current;
    const anchor = button.current;
    openRef.current = open;
    if (!popover || !element || !anchor) return;
    if (!open) {
      if (element.matches(':popover-open')) element.hidePopover();
      // Closed it is a hidden pixel by the button: window coordinates would put it far off and
      // widen a scrolling ancestor.
      element.style.removeProperty('left');
      element.style.removeProperty('top');
      return;
    }
    if (!element.matches(':popover-open')) element.showPopover();
    let placed = '';
    let frame = 0;
    const follow = () => {
      const rect = anchor.getBoundingClientRect();
      const at = [
        rect.left,
        rect.top,
        rect.width,
        rect.height,
        element.offsetWidth,
        element.offsetHeight,
        window.innerWidth,
        window.innerHeight,
      ].join();
      if (at !== placed) {
        placed = at;
        placePopover(element, anchor, placement);
      }
      frame = requestAnimationFrame(follow);
    };
    follow();
    return () => cancelAnimationFrame(frame);
  }, [open, placement, popover]);
  useEffect(() => () => window.clearTimeout(closeTimer.current), []);
  // The browser may close the popover itself (full screen): the state follows until the next
  // hover or focus.
  useEffect(() => {
    const element = drawer.current;
    if (!popover || !element) return;
    const onToggle = (event: Event) => {
      if ((event as ToggleEvent).newState !== 'closed' || !openRef.current) return;
      window.clearTimeout(closeTimer.current);
      setHovered(false);
      setDismissed(true);
    };
    element.addEventListener('toggle', onToggle);
    return () => element.removeEventListener('toggle', onToggle);
  }, [popover]);

  const enter = () => {
    window.clearTimeout(closeTimer.current);
    setHovered(true);
    setDismissed(false);
  };
  const leave = () => {
    window.clearTimeout(closeTimer.current);
    closeTimer.current = window.setTimeout(() => setHovered(false), CLOSE_DELAY_MS);
  };
  // Only a focus the viewer sees opens it (Tab, the arrows of a row): a click focuses the button
  // too, and the popover would then stay open after the pointer left.
  const onFocus = (event: FocusEvent) => {
    if (!event.target.matches(':focus-visible')) return;
    setFocused(true);
    if (event.target === slider.current) setDismissed(false);
  };
  const onBlur = (event: FocusEvent<HTMLDivElement>) => {
    if (event.currentTarget.contains(event.relatedTarget)) return;
    setFocused(false);
    setDismissed(false);
  };
  // Escape closes the open popover, and only that (the player's Escape goes back); the focus moves
  // from the slider to the button, so it stays visible.
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== 'Escape' || !open) return;
    event.preventDefault();
    event.stopPropagation();
    window.clearTimeout(closeTimer.current);
    setHovered(false);
    setDismissed(true);
    if (document.activeElement === slider.current) button.current?.focus();
  };

  return (
    // The pointer and focus events only open and close the popover; the button and the slider inside
    // are the controls.
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions
    <div
      className={styles.control}
      data-popover={popover || undefined}
      data-open={open || undefined}
      onPointerEnter={popover ? enter : undefined}
      onPointerLeave={popover ? leave : undefined}
      onFocus={popover ? onFocus : undefined}
      onBlur={popover ? onBlur : undefined}
      onKeyDown={popover ? onKeyDown : undefined}
    >
      <AriaButton
        ref={buttonRefs}
        variant="ghost"
        icon={shown === 0 ? 'volumeMute' : 'volume'}
        iconOnly
        aria-label={t('player.volumeControl.mute')}
        aria-pressed={muted}
        onPress={onToggleMute}
        isDisabled={isDisabled}
        // react-aria's Button has no tabIndex; a negative one takes it out of the Tab order.
        excludeFromTabOrder={tabIndex !== undefined && tabIndex < 0}
      />
      {!muteOnly && (
        <span
          ref={drawer}
          className={popover ? styles.popover : styles.drawer}
          popover={popover ? 'manual' : undefined}
        >
          <div
            ref={sliderRefs}
            className={styles.slider}
            role="slider"
            tabIndex={isDisabled ? -1 : (tabIndex ?? 0)}
            aria-label={t('player.volumeControl.slider')}
            aria-valuemin={0}
            aria-valuemax={Math.round(max * 100)}
            aria-valuenow={percent}
            aria-valuetext={valueText}
            aria-disabled={isDisabled || undefined}
            data-dragging={dragging || undefined}
            onKeyDown={(event) => {
              const next = keys[event.key];
              const modified = event.shiftKey || event.altKey || event.ctrlKey || event.metaKey;
              if (isDisabled || !next || modified) return;
              event.preventDefault();
              event.stopPropagation();
              set(next(current()));
            }}
            onPointerDown={(event) => {
              const value = isDisabled ? null : positionAt(event);
              if (value === null) return;
              event.currentTarget.setPointerCapture(event.pointerId);
              setDragging(true);
              set(value);
            }}
            onPointerMove={(event) => {
              const value = dragging ? positionAt(event) : null;
              if (value !== null) set(value);
            }}
            onPointerUp={(event) => {
              if (!dragging) return;
              event.currentTarget.releasePointerCapture(event.pointerId);
              setDragging(false);
            }}
            onPointerCancel={() => setDragging(false)}
            onBlur={() => {
              pending.current.values.length = 0;
            }}
          >
            {/* The position is measured on the track, inside the slider's margin: its ends are
                0 % and the maximum. */}
            <span ref={track} className={styles.track}>
              <span
                className={styles.fill}
                style={{ width: `${(Math.min(shown, 1) / max) * 100}%` }}
              />
              {boosted && (
                <span
                  className={styles.boost}
                  data-boost=""
                  style={{ left: `${100 / max}%`, width: `${((shown - 1) / max) * 100}%` }}
                />
              )}
              {boost && <span className={styles.full} style={{ left: `${100 / max}%` }} />}
              <span className={styles.thumb} style={{ left: `${(shown / max) * 100}%` }} />
            </span>
          </div>
          <span className={styles.percent} data-boosted={boosted || undefined} aria-hidden="true">
            {boosted && <Icon name="warning" size="sm" />}
            {t('player.volumeControl.percent', { percent })}
          </span>
        </span>
      )}
    </div>
  );
}
