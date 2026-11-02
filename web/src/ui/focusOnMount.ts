/**
 * Ref callback that focuses its element as soon as it mounts: the initial focus of a dialog,
 * instead of react-aria's `autoFocus`.
 *
 * Enter on a link or a native button is a "virtual" click for react-aria, and its `autoFocus` then
 * waits until no CSS transition runs on the page any more. A modal makes the page inert, which
 * cancels the focus ring transition of the element that opened it (outline-color and
 * outline-offset at once); react-aria counts only the first cancelled property, waits for good and
 * focuses the dialog itself after 500 ms. A ref callback runs before the dialog's own effect looks
 * at the focus, so the dialog finds it inside and leaves it there.
 *
 * It is a module function, so its identity never changes and a re-render does not call it again.
 */
export function focusOnMount(element: HTMLElement | null): void {
  element?.focus();
}
