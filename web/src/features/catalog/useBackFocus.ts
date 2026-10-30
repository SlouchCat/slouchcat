import { useLocation } from '@tanstack/react-router';
import { useEffect, useRef } from 'react';
import { setSpatialFocus, spatialFocusKey } from '@/ui';

// The card a grid page was left from, per history entry: the browser's back (or the back key on
// the detail page) returns to it with the focus (concept 08, 7.4). Only for this tab's session.
const leftFrom = new Map<string, string>();

/**
 * Remembers the focused card when the page goes away and focuses it again when the same history
 * entry comes back. `keys` are the focus keys of the cards shown now (`<prefix>-<id>`).
 */
export function useBackFocus(prefix: string, keys: readonly string[]): void {
  const entry = useLocation({ select: (location) => location.state.__TSR_key });
  const restored = useRef(false);
  const joined = keys.join('|');

  useEffect(() => {
    if (restored.current || !entry) return;
    const key = leftFrom.get(entry);
    // Wait until the card is back (the first page may still be on its way).
    if (!key || !joined.split('|').includes(key)) return;
    restored.current = true;
    leftFrom.delete(entry);
    setSpatialFocus(key);
  }, [entry, joined]);

  useEffect(
    () => () => {
      // norigin knows no key before the first focus (typed as string, it may be null).
      const key = spatialFocusKey() as string | null;
      if (entry && key?.startsWith(`${prefix}-`)) leftFrom.set(entry, key);
    },
    [entry, prefix],
  );
}
