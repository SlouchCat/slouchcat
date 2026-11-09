import { useLocation } from '@tanstack/react-router';
import { useEffect, useRef } from 'react';
import { setSpatialFocus, spatialFocusKey } from '@/ui';

// The card a grid page was left from, per history entry: the browser's back (or the back key on
// the detail page) returns to it with the focus (concept 08, 7.4). Only for this tab's session.
const leftFrom = new Map<string, { key: string; index?: number }>();

/**
 * How often a virtual list scrolls a card's place back while its card is missing: the router's
 * scroll restoration can come after the first time (Firefox) and take the place out of view.
 */
const MAX_REVEALS = 3;

/** A virtual list's way back to a card that is not mounted (concept 17 E-13). */
export interface BackFocusList {
  /** The place of a card's focus key in the whole list; undefined when it is not there. */
  indexOf: (key: string) => number | undefined;
  /**
   * Brings place `index` into view, so its card mounts once its page is there, and keeps it drawn
   * until the focus arrives; false when it did not need to scroll (the place is drawn, its page on
   * the way) or cannot.
   */
  reveal: (index: number) => boolean;
}

/**
 * Remembers the focused card when the page goes away and focuses it again when the same history
 * entry comes back. `keys` are the focus keys of the cards shown now (`<prefix>-<id>`); for a
 * virtual list the mounted ones, and `list` brings the card's place back into view when it is not
 * mounted once the list shows any card (its page dropped from the cache, another column count),
 * again when the shown cards change without it (at most MAX_REVEALS times).
 */
export function useBackFocus(prefix: string, keys: readonly string[], list?: BackFocusList): void {
  const entry = useLocation({ select: (location) => location.state.__TSR_key });
  const restored = useRef(false);
  const reveals = useRef(0);
  const joined = keys.join('|');
  const current = useRef(list);
  useEffect(() => {
    current.current = list;
  });

  useEffect(() => {
    if (restored.current || !entry) return;
    const left = leftFrom.get(entry);
    if (!left) return;
    // Wait until the card is back (the first page may still be on its way).
    if (!joined.split('|').includes(left.key)) {
      if (joined !== '' && left.index !== undefined && reveals.current < MAX_REVEALS) {
        if (current.current?.reveal(left.index)) reveals.current += 1;
      }
      return;
    }
    restored.current = true;
    leftFrom.delete(entry);
    // The focus is set a little later (norigin queues it): the router's scroll restoration must
    // not take the card out of the list meanwhile.
    if (left.index !== undefined) current.current?.reveal(left.index);
    setSpatialFocus(left.key);
  }, [entry, joined]);

  useEffect(
    () => () => {
      // norigin knows no key before the first focus (typed as string, it may be null).
      const key = spatialFocusKey() as string | null;
      if (entry && key?.startsWith(`${prefix}-`)) {
        leftFrom.set(entry, { key, index: current.current?.indexOf(key) });
      }
    },
    [entry, prefix],
  );
}
