import { useEffect, useRef } from 'react';

const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

/** A dialog handles its own back key (it closes); the page behind it must not go back as well. */
const inDialog = (target: EventTarget | null) =>
  target instanceof Element && target.closest('[role="dialog"], [role="alertdialog"]') !== null;

/**
 * Escape and Backspace act like a remote's back key (C-4). Ignored while typing, inside a dialog
 * and when another handler already consumed the key.
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
      if (isEditable(event.target) || inDialog(event.target)) return;
      event.preventDefault();
      handler.current();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
}
