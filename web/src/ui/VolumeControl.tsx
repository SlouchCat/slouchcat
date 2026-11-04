import { useRef, useState, type PointerEvent, type Ref } from 'react';
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
}

/**
 * Volume (V-3): a mute button and a slider beside it that opens while the pointer is over the
 * control or the focus is in it (always open without hover, on touch screens). The slider moves
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
}: VolumeControlProps) {
  const { t } = useTranslation();
  const track = useRef<HTMLSpanElement>(null);
  const [dragging, setDragging] = useState(false);
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

  return (
    <div className={styles.control}>
      <AriaButton
        ref={buttonRef}
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
        <span className={styles.drawer}>
          <div
            ref={sliderRef}
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
