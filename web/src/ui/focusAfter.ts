import type { RefObject } from 'react';

/**
 * Focus helpers of the administration and settings pages (concept 12, ALL-2): after a change
 * removed or disabled the control that had the focus, the page's h1 takes it.
 */

/** How long `afterRender` waits for a frame at most: a hidden tab paints no frames. */
const FRAME_FALLBACK_MS = 100;

/**
 * Resolves once React has rendered what was just set: a task, then the next frame. A browser
 * tab in the background paints no frames, so a timeout of 100 ms stands in for the frame there;
 * the focus does not wait until the tab is shown again. The timer and the frame that lose are
 * cleared. `signal` (the page unmounted) clears them as well and resolves at once: under Vitest
 * `window.setTimeout` is a Node timer that outlives jsdom, and one that fired after the file
 * ended threw "document is not defined" (PicksPage.test.tsx).
 */
export const afterRender = (signal?: AbortSignal) =>
  new Promise<void>((resolve) => {
    if (signal?.aborted) {
      resolve();
      return;
    }
    let task = 0;
    let frame = 0;
    let late = 0;
    const done = () => {
      window.clearTimeout(task);
      window.cancelAnimationFrame(frame);
      window.clearTimeout(late);
      signal?.removeEventListener('abort', done);
      resolve();
    };
    signal?.addEventListener('abort', done);
    task = window.setTimeout(() => {
      frame = window.requestAnimationFrame(done);
      late = window.setTimeout(done, FRAME_FALLBACK_MS);
    }, 0);
  });

/**
 * Resolves once React has rendered and no overlay is closing any more: a react-aria dialog,
 * popover or menu stays in the DOM while it plays its exit animation (`data-exiting`), and only
 * then gives the focus back to the element that opened it. Waits for at most 60 more rounds:
 * about one second at 60 frames per second, about six seconds in a background tab, where each
 * round takes the 100 ms of the fallback. `signal` ends the wait at once (see `afterRender`);
 * the caller then leaves the focus alone.
 */
export async function afterDialogsClosed(signal?: AbortSignal): Promise<void> {
  await afterRender(signal);
  for (
    let rounds = 0;
    rounds < 60 && !signal?.aborted && document.querySelector('[data-exiting]');
    rounds += 1
  ) {
    await afterRender(signal);
  }
  // react-aria hands the focus back in a frame after the unmount; that comes first.
  await afterRender(signal);
}

/**
 * The focus is gone: nowhere, on the body, on an element no longer in the document, or on a
 * control that turned disabled (Firefox keeps the focus there, where Tab and Enter no longer act).
 */
export function focusLost(): boolean {
  const active = document.activeElement;
  return !active || active === document.body || !active.isConnected || active.matches(':disabled');
}

/** Moves the focus to the page's h1 when it is lost (see `focusLost`). */
export function focusTitleIfLost(titleRef: RefObject<HTMLElement | null>): void {
  if (focusLost()) titleRef.current?.focus();
}
