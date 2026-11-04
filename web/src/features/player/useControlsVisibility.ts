import { useCallback, useEffect, useRef, useState } from 'react';

/** Controls fade out after this long without input while the video plays. */
export const CONTROLS_HIDE_MS = 3000;
/** How long a message over the picture («Lautstärke 80 %», «+10 s») stays. */
const OSD_MS = 1200;

/**
 * The controls of the online and the offline video player: visible on any input and while paused,
 * idle after CONTROLS_HIDE_MS without input while the video plays. `wake` is the input, `hide`
 * fades them out at once (a tap on the picture). `showOsd` shows a short message over the picture.
 */
export function useControlsVisibility(playing: boolean) {
  const hideTimer = useRef<number | undefined>(undefined);
  const osdTimer = useRef<number | undefined>(undefined);
  const [idle, setIdle] = useState(false);
  const [osd, setOsd] = useState<string | null>(null);

  const scheduleHide = useCallback(() => {
    window.clearTimeout(hideTimer.current);
    hideTimer.current = window.setTimeout(() => setIdle(true), CONTROLS_HIDE_MS);
  }, []);
  const wake = useCallback(() => {
    setIdle(false);
    scheduleHide();
  }, [scheduleHide]);
  const hide = useCallback(() => {
    window.clearTimeout(hideTimer.current);
    setIdle(true);
  }, []);

  useEffect(() => {
    if (playing) scheduleHide();
    else window.clearTimeout(hideTimer.current);
  }, [playing, scheduleHide]);

  useEffect(
    () => () => {
      window.clearTimeout(hideTimer.current);
      window.clearTimeout(osdTimer.current);
    },
    [],
  );

  const showOsd = useCallback((text: string) => {
    setOsd(text);
    window.clearTimeout(osdTimer.current);
    osdTimer.current = window.setTimeout(() => setOsd(null), OSD_MS);
  }, []);

  return { idle, wake, hide, osd, showOsd };
}
