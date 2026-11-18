import { useEffect, type RefObject } from 'react';

/** How long after the page becomes visible again a playing video must have shown a new frame. */
export const VIDEO_WAKE_CHECK_MS = 1500;

/** Frames the element has shown; `undefined` where the browser does not count them. */
function presentedFrames(element: HTMLVideoElement): number | undefined {
  if (typeof element.getVideoPlaybackQuality !== 'function') return undefined;
  const quality = element.getVideoPlaybackQuality();
  return quality.totalVideoFrames - quality.droppedVideoFrames;
}

/**
 * Wakes the picture of a video that stood still while its page was hidden.
 *
 * Firefox stops decoding the picture of a video in a background tab after 10 s
 * (`media.suspend-bkgnd-video.enabled`) while the sound plays on. When the tab comes back it
 * sometimes does not decode again: the clock and the sound run, the picture stands until a seek.
 *
 * When the page becomes visible while the video plays, the hook waits `VIDEO_WAKE_CHECK_MS`. Has
 * the clock moved on in that time without a new frame (`requestVideoFrameCallback`, else the
 * presented-frame count), it seeks to the current position, which restarts the decoder. A video
 * that shows frames, waits for data or was paused or seeked meanwhile is left alone.
 */
export function useVideoWake(video: RefObject<HTMLVideoElement | null>): void {
  useEffect(() => {
    let timer: number | undefined;
    let cancelFrame = () => {};

    const cancel = () => {
      window.clearTimeout(timer);
      timer = undefined;
      cancelFrame();
      cancelFrame = () => {};
    };

    const onVisibility = () => {
      cancel();
      const element = video.current;
      if (document.visibilityState !== 'visible' || !element) return;
      if (element.paused || element.ended || element.seeking) return;

      const startTime = element.currentTime;
      const startFrames = presentedFrames(element);
      let frameShown = false;
      const watchesFrames = typeof element.requestVideoFrameCallback === 'function';
      if (watchesFrames) {
        const handle = element.requestVideoFrameCallback(() => {
          frameShown = true;
        });
        cancelFrame = () => element.cancelVideoFrameCallback(handle);
      }

      timer = window.setTimeout(() => {
        timer = undefined;
        cancelFrame();
        cancelFrame = () => {};
        if (video.current !== element || document.visibilityState !== 'visible') return;
        if (element.paused || element.ended || element.seeking) return;
        // The clock stood as well: the video waits for data, which is not this fault.
        if (element.currentTime - startTime < VIDEO_WAKE_CHECK_MS / 2000) return;
        const stuck = watchesFrames
          ? !frameShown
          : startFrames !== undefined && presentedFrames(element) === startFrames;
        if (!stuck) return;
        const position = element.currentTime;
        element.currentTime = position;
      }, VIDEO_WAKE_CHECK_MS);
    };

    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      cancel();
    };
  }, [video]);
}
