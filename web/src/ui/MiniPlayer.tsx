import type { KeyboardEvent, ReactNode } from 'react';
import { Icon } from './Icon';
import styles from './MiniPlayer.module.css';
import { useSpatialItem } from './spatial';
import { StationLogo } from './StationLogo';

// The radio's mini player (concept 16, 10.3): a bar at the bottom of every page with the app
// header. Plain buttons with arrow-key navigation (norigin), no react-aria: it is part of the
// start bundle (P-10). The texts come from the app (rule 9).

/** `shifted`: plays behind live (the badge says «−12:30», concept 16 stage 2). */
export type MiniPlayerState = 'starting' | 'live' | 'shifted' | 'paused' | 'error';

interface MiniPlayerToggle {
  pressed: boolean;
  label: string;
  onPress: () => void;
}

interface MiniPlayerProps {
  /** Accessible name of the bar («Radio-Wiedergabe»). */
  label: string;
  stationName: string;
  logoUrl?: string;
  /** The cover of the title that plays; the logo shows while it has none or it fails. */
  coverUrl?: string;
  /** The second line: artist and title, «Live», or the message of a stopped station. */
  line: string;
  state: MiniPlayerState;
  /** The badge beside the controls: «LIVE», «−12:30», «Pausiert», «Verbindet …». */
  stateLabel?: string;
  /**
   * A note over the bar with its action: the pause lasted longer than the time shift window
   * («Die Pause war länger als 30 Minuten …» with «Live»).
   */
  hint?: { text: string; actionLabel: string; onAction: () => void };
  /** Opens «Jetzt läuft»; its accessible name says so. */
  openLabel: string;
  onOpen: () => void;
  /** Play or pause: `pressed` = playing, the label names what a press does. */
  toggle: MiniPlayerToggle;
  /** The favourite mark (heart); absent where it cannot be set. */
  favorite?: MiniPlayerToggle;
  closeLabel: string;
  onClose: () => void;
  /**
   * The volume beside play/pause (concept 19 E3): the app's VolumeControl, which the bar does not
   * import (react-aria, P-10), and the mute that M switches while the focus is in the bar.
   */
  volume?: { control: ReactNode; onToggleMute: () => void };
}

interface BarButtonProps {
  className: string | undefined;
  label: string;
  pressed?: boolean;
  onPress: () => void;
  testId?: string;
  children: ReactNode;
}

function BarButton({ className, label, pressed, onPress, testId, children }: BarButtonProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>();
  return (
    <button
      ref={ref}
      type="button"
      className={className}
      aria-label={label}
      aria-pressed={pressed}
      title={label}
      data-testid={testId}
      onFocus={onFocus}
      onClick={onPress}
    >
      {children}
    </button>
  );
}

/**
 * Space pauses or plays on wherever the focus is in the player (rule 8), also on the button that
 * opens «Jetzt läuft»: the capture phase takes the key before the focused button acts on it.
 */
export function useSpaceToggle(onToggle: () => void) {
  const isSpace = (event: KeyboardEvent) =>
    event.key === ' ' && !event.altKey && !event.ctrlKey && !event.metaKey;
  return {
    onKeyDownCapture: (event: KeyboardEvent) => {
      if (!isSpace(event)) return;
      event.preventDefault();
      event.stopPropagation();
      if (!event.repeat) onToggle();
    },
    onKeyUpCapture: (event: KeyboardEvent) => {
      if (!isSpace(event)) return;
      event.preventDefault();
      event.stopPropagation();
    },
  };
}

/**
 * M mutes and unmutes wherever the focus is in the radio's controls (the mini player, «Jetzt
 * läuft»), like M in the players (concept 19 E3). Not in a text field, not with a modifier.
 */
export function useMuteKey(onToggleMute: (() => void) | undefined) {
  return {
    onKeyDown: (event: KeyboardEvent) => {
      if (!onToggleMute || event.key.toLowerCase() !== 'm' || event.repeat) return;
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      const target = event.target as HTMLElement;
      if (target.closest('input, textarea, [contenteditable="true"]')) return;
      event.preventDefault();
      onToggleMute();
    },
  };
}

export function MiniPlayer({
  label,
  stationName,
  logoUrl,
  coverUrl,
  line,
  state,
  stateLabel,
  hint,
  openLabel,
  onOpen,
  toggle,
  favorite,
  closeLabel,
  onClose,
  volume,
}: MiniPlayerProps) {
  const space = useSpaceToggle(toggle.onPress);
  const mute = useMuteKey(volume?.onToggleMute);
  return (
    <section
      className={styles.bar}
      aria-label={label}
      data-state={state}
      data-testid="radio-mini-player"
      {...space}
      {...mute}
    >
      {hint && (
        <p className={styles.hint} data-testid="radio-mini-hint">
          <span>{hint.text}</span>
          <BarButton
            className={styles.hintAction}
            label={hint.actionLabel}
            onPress={hint.onAction}
            testId="radio-mini-hint-live"
          >
            {hint.actionLabel}
          </BarButton>
        </p>
      )}
      <BarButton
        className={styles.open}
        label={openLabel}
        onPress={onOpen}
        testId="radio-mini-open"
      >
        <StationLogo
          name={stationName}
          src={coverUrl ?? logoUrl}
          fallbackSrc={coverUrl ? logoUrl : undefined}
          size="sm"
        />
        <span className={styles.text}>
          <span className={styles.name}>{stationName}</span>
          <span className={styles.line} data-testid="radio-mini-line">
            {line}
          </span>
        </span>
      </BarButton>
      {stateLabel && (
        <span className={styles.state} aria-hidden="true" data-testid="radio-mini-state">
          {state === 'live' && <span className={styles.dot} />}
          {stateLabel}
        </span>
      )}
      <BarButton
        className={styles.play}
        label={toggle.label}
        onPress={toggle.onPress}
        testId="radio-mini-toggle"
      >
        <Icon name={toggle.pressed ? 'pause' : 'play'} />
      </BarButton>
      {volume && (
        <span className={styles.volume} data-testid="radio-mini-volume">
          {volume.control}
        </span>
      )}
      {favorite && (
        <BarButton
          className={styles.icon}
          label={favorite.label}
          pressed={favorite.pressed}
          onPress={favorite.onPress}
          testId="radio-mini-favorite"
        >
          <Icon name="heart" />
        </BarButton>
      )}
      <BarButton
        className={styles.icon}
        label={closeLabel}
        onPress={onClose}
        testId="radio-mini-close"
      >
        <Icon name="close" />
      </BarButton>
    </section>
  );
}
