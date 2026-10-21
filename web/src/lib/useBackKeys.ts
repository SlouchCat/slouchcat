import { useEffect, useRef } from 'react';

const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

/** An empty search field: Escape there has nothing to clear and goes back (a remote has no Tab). */
const isEmptySearch = (target: EventTarget | null) =>
  target instanceof HTMLInputElement && target.type === 'search' && target.value === '';

/** A dialog handles its own back key (it closes); the page behind it must not go back as well. */
const inDialog = (target: EventTarget | null) =>
  target instanceof Element && target.closest('[role="dialog"], [role="alertdialog"]') !== null;

/**
 * Escape and Backspace act like a remote's back key (C-4). Ignored while typing (except Escape in an
 * empty search field), inside a dialog and when another handler already consumed the key.
 */
export function useBackKeys(onBack: () => void): void {
  const handler = useRef(onBack);
  useEffect(() => {
    handler.current = onBack;
  });
  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
      if (event.key !== 'Escape' && event.key !== 'Backspace') return;
      const typing =
        isEditable(event.target) && !(event.key === 'Escape' && isEmptySearch(event.target));
      if (typing || inDialog(event.target)) return;
      event.preventDefault();
      handler.current();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
}
