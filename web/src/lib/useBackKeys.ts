import { useEffect, useRef } from 'react';

const isEditable = (target: EventTarget | null) =>
  target instanceof HTMLElement &&
  (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

/**
 * Escape and Backspace act like a remote's back key (C-4). Ignored while typing and when another
 * handler (an open dialog) already consumed the key.
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
      if (isEditable(event.target)) return;
      event.preventDefault();
      handler.current();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
}
