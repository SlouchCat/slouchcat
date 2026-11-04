import { useRef, type RefObject, type WheelEvent } from 'react';
import { VOLUME_STEP } from '@/ui/aria';

/** One notch of a mouse wheel in pixels (Chromium, Firefox in pixel mode), and one line. */
const WHEEL_NOTCH_PX = 100;
const WHEEL_LINE_PX = 100 / 3;

/**
 * The mouse wheel over the picture (V-3), for the online and the offline player: one notch is one
 * volume step. Over the bar and over a panel the wheel stays theirs (the chrome lets the pointer
 * through to the video only in between). Trackpads send many small deltas; they add up to
 * notches. The handler goes on the player region.
 */
export function useWheelVolume(
  media: RefObject<HTMLMediaElement | null>,
  changeVolume: (delta: number) => void,
) {
  const wheel = useRef(0);
  return (event: WheelEvent<HTMLElement>) => {
    // Ctrl with the wheel zooms the page.
    if (event.target !== media.current || event.deltaY === 0 || event.ctrlKey) return;
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
}
