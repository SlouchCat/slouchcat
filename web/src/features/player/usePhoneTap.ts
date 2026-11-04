import { useEffect, useRef, type PointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { SEEK_STEP_S } from './keymap';

/** Two taps on the same half of the picture within this time seek (V-13, phone). */
export const DOUBLE_TAP_MS = 300;

interface PictureTapOptions {
  /** The phone layout (11.2): narrower than 768 px, or a touch screen held sideways. */
  phone: boolean;
  /** The controls are faded out. */
  hidden: boolean;
  togglePlay: () => void;
  toggleFullscreen: () => void;
  seekBy: (seconds: number) => void;
  /** Shows «−10 s» or «+10 s» over the picture. */
  showOsd: (text: string) => void;
  /** Whether the viewer wants the video to play (not what the element reports). */
  wantsPlaying: () => boolean;
  /** Fades the controls out at once. */
  hide: () => void;
  /**
   * Nothing plays any more (the offline watch-time limit): a tap still shows and hides the
   * controls, but taps do not seek and a click does not play.
   */
  disabled?: boolean;
}

/**
 * The pointer on the picture, for the online and the offline player. A mouse click plays and
 * pauses and a double click goes full screen, also in the phone layout of a narrow window (review
 * step 5). A finger or a pen in the phone layout (V-13): a tap shows the controls, or hides them
 * while the video plays; two taps on the same half within DOUBLE_TAP_MS seek 10 s back (left) or
 * on (right). The tap waits that long to tell one from two. The handlers go on the video element.
 */
export function usePhoneTap({
  phone,
  hidden,
  togglePlay,
  toggleFullscreen,
  seekBy,
  showOsd,
  wantsPlaying,
  hide,
  disabled = false,
}: PictureTapOptions) {
  const { t } = useTranslation();
  const hiddenAtTap = useRef(false);
  const lastPointer = useRef<string>('mouse');
  const lastTap = useRef<{ at: number; side: 'left' | 'right' } | null>(null);
  const tapTimer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(tapTimer.current), []);

  const onTap = (event: PointerEvent<HTMLVideoElement>) => {
    const box = event.currentTarget.getBoundingClientRect();
    const side = event.clientX < box.left + box.width / 2 ? 'left' : 'right';
    const previous = lastTap.current;
    window.clearTimeout(tapTimer.current);
    if (
      !disabled &&
      previous &&
      previous.side === side &&
      event.timeStamp - previous.at < DOUBLE_TAP_MS
    ) {
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

  const touch = () => phone && lastPointer.current !== 'mouse';

  return {
    onPointerDown: (event: PointerEvent<HTMLVideoElement>) => {
      hiddenAtTap.current = hidden;
      lastPointer.current = event.pointerType;
    },
    onPointerUp: (event: PointerEvent<HTMLVideoElement>) => {
      if (event.button !== 0) return;
      lastPointer.current = event.pointerType;
      if (touch()) onTap(event);
      else if (!disabled) togglePlay();
    },
    onDoubleClick: () => {
      // Two taps of a finger seek; a double click of the mouse goes full screen.
      if (!touch()) toggleFullscreen();
    },
  };
}
