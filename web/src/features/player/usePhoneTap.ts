import { useEffect, useRef, type PointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { SEEK_STEP_S } from './keymap';

/** Two taps on the same half of the picture within this time seek (V-13, phone). */
export const DOUBLE_TAP_MS = 300;

interface PhoneTapOptions {
  /** The controls are faded out. */
  hidden: boolean;
  seekBy: (seconds: number) => void;
  /** Shows «10 s zurück» or «10 s vor» over the picture. */
  showOsd: (text: string) => void;
  /** Whether the viewer wants the video to play (not what the element reports). */
  wantsPlaying: () => boolean;
  /** Fades the controls out at once. */
  hide: () => void;
}

/**
 * The taps on the picture of a phone (V-13), for the online and the offline player: a tap shows
 * the controls, or hides them while the video plays; two taps on the same half within
 * DOUBLE_TAP_MS seek 10 s back (left) or on (right). The tap waits that long to tell one from two.
 * `onPointerDown` and `onTap` go on the video element (pointer down and pointer up).
 */
export function usePhoneTap({ hidden, seekBy, showOsd, wantsPlaying, hide }: PhoneTapOptions) {
  const { t } = useTranslation();
  const hiddenAtTap = useRef(false);
  const lastTap = useRef<{ at: number; side: 'left' | 'right' } | null>(null);
  const tapTimer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(tapTimer.current), []);

  const onPointerDown = () => {
    hiddenAtTap.current = hidden;
  };

  const onTap = (event: PointerEvent<HTMLVideoElement>) => {
    const box = event.currentTarget.getBoundingClientRect();
    const side = event.clientX < box.left + box.width / 2 ? 'left' : 'right';
    const previous = lastTap.current;
    window.clearTimeout(tapTimer.current);
    if (previous && previous.side === side && event.timeStamp - previous.at < DOUBLE_TAP_MS) {
      lastTap.current = null;
      seekBy(side === 'left' ? -SEEK_STEP_S : SEEK_STEP_S);
      showOsd(
        side === 'left'
          ? t('player.chrome.tapBack', { seconds: SEEK_STEP_S })
          : t('player.chrome.tapForward', { seconds: SEEK_STEP_S }),
      );
      return;
    }
    lastTap.current = { at: event.timeStamp, side };
    const wasHidden = hiddenAtTap.current;
    tapTimer.current = window.setTimeout(() => {
      lastTap.current = null;
      // The pointer showed the controls already (poke); a tap on visible ones hides them.
      if (!wasHidden && wantsPlaying()) hide();
    }, DOUBLE_TAP_MS);
  };

  return { onPointerDown, onTap };
}
