/**
 * Ref callback that focuses its element right after it mounts: the initial focus of a dialog,
 * instead of react-aria's `autoFocus`.
 *
 * Enter on a link or a native button is a "virtual" click for react-aria, and its `autoFocus` then
 * waits until no CSS transition runs on the page any more. A modal makes the page inert, which
 * cancels the focus ring transition of the element that opened it (outline-color and
 * outline-offset at once); react-aria counts only the first cancelled property, waits for good and
 * focuses the dialog itself after 500 ms. Focusing here does not wait for transitions, and the
 * dialog leaves a focus inside it alone: its 500 ms check and its deferred focus both give way to
 * an element that has the focus.
 *
 * The focus moves in a microtask, after React's commit, not in the ref callback itself: moving it
 * blurs the field the reader was typing in, and that field's blur and change handlers must not run
 * in the middle of a commit (in tests, user-event's `change` on blur calls `act()`, which then
 * finds React already working and leaves the root unable to render again).
 *
 * It is a module function, so its identity never changes and a re-render does not call it again.
 */
export function focusOnMount(element: HTMLElement | null): void {
  if (!element) return;
  queueMicrotask(() => {
    if (element.isConnected) element.focus();
  });
}
