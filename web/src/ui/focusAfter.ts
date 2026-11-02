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
 * the focus does not wait until the tab is shown again.
 */
export const afterRender = () =>
  new Promise<void>((resolve) => {
    window.setTimeout(() => {
      void Promise.race([
        new Promise<void>((frame) => window.requestAnimationFrame(() => frame())),
        new Promise<void>((late) => window.setTimeout(late, FRAME_FALLBACK_MS)),
      ]).then(resolve);
    }, 0);
  });

/**
 * Resolves once React has rendered and no overlay is closing any more: a react-aria dialog,
 * popover or menu stays in the DOM while it plays its exit animation (`data-exiting`), and only
 * then gives the focus back to the element that opened it. Waits for about one second at most.
 */
export async function afterDialogsClosed(): Promise<void> {
  await afterRender();
  for (let rounds = 0; rounds < 60 && document.querySelector('[data-exiting]'); rounds += 1) {
    await afterRender();
  }
  // react-aria hands the focus back in a frame after the unmount; that comes first.
  await afterRender();
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
