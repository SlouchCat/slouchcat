import {
  defaultRangeExtractor,
  elementScroll,
  observeElementOffset,
  observeElementRect,
  observeWindowOffset,
  observeWindowRect,
  useVirtualizer,
  windowScroll,
  type Range,
  type VirtualItem,
} from '@tanstack/react-virtual';
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

// The drawn part of a long list (concept 18 C-3): its places in rows of `columns`, only the rows
// in view and a screen's height above and below in the document once the list is `virtual`, each
// row measured as it is drawn. The keys follow concept 17 6.2 point 2: an arrow press or Tab
// towards a place that is not drawn or still empty scrolls its row into view and focuses it once
// its entry is there; the row with the focus stays drawn. VirtualList (lists and grids) and
// DataTable (tables) draw the rows.

/** The places a list should have read: `start` to `end`, both included. */
export interface PlaceSpan {
  start: number;
  end: number;
}

interface VirtualPlacesOptions {
  /** The number of places (the list's total). */
  count: number;
  /** Draw only the rows near the view; else every row. */
  virtual: boolean;
  /** Places per row (1 for a list). */
  columns?: number;
  /** A row's expected height in px (with its gap), until it is measured. */
  estimate: number;
  /** The gap between two rows in px (the container's CSS row gap). */
  gap?: number;
  /** The element that scrolls the list (a dialog's list); the window when left out. */
  scrollElement?: RefObject<HTMLElement | null>;
  /** The list's element: the keys are handled on it and its top is the scroll margin. */
  root: RefObject<HTMLElement | null>;
  /**
   * The places to read: the rows in view and half a screen above and below (concept 18 C-3), or
   * every place of a list that is not virtual.
   */
  onRange?: (span: PlaceSpan) => void;
  /** Whether place `index` holds its entry (not an empty place of a page on its way). */
  ready: (index: number) => boolean;
  /** Arrow keys between places (spatial navigation, a table's rows); Tab always. */
  arrows?: boolean;
}

export interface VirtualPlaces {
  /** The rows to draw, in order: their index and, when virtual, their start. */
  rows: readonly VirtualItem[] | readonly { index: number; key: number }[];
  /** Space above the first and below the last drawn row, in px. */
  before: number;
  after: number;
  /** Measures a drawn row: the element carries `data-index` (the row's index). */
  measure: (element: Element | null) => void;
  /** Scrolls place `index` into view and focuses its first control once its entry is there. */
  go: (index: number, part?: 'first' | 'last') => void;
}

/** The controls of a place that take the focus. */
const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), [tabindex="0"]';

interface Pending {
  index: number;
  part: 'first' | 'last';
  /** The element focused when the move began; another focus meanwhile cancels it. */
  from: Element | null;
}

const ARROWS: Record<string, 'up' | 'down' | 'left' | 'right'> = {
  ArrowUp: 'up',
  ArrowDown: 'down',
  ArrowLeft: 'left',
  ArrowRight: 'right',
};

/**
 * The rows of a list to draw and the keys at the edge of what is drawn. A place is an element
 * with `data-place` (its index) inside `root`; its controls take the focus.
 */
export function useVirtualPlaces({
  count,
  virtual,
  columns = 1,
  estimate,
  gap = 0,
  scrollElement,
  root,
  onRange,
  ready,
  arrows = false,
}: VirtualPlacesOptions): VirtualPlaces {
  const rowCount = Math.ceil(count / Math.max(1, columns));
  const onWindow = scrollElement === undefined;

  // The list's top in the document: the window's scroll margin. It follows what stands above.
  const [margin, setMargin] = useState(0);
  useLayoutEffect(() => {
    const node = root.current;
    if (!node || !onWindow || !virtual) return;
    const update = () => setMargin(Math.round(node.getBoundingClientRect().top + window.scrollY));
    update();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(node.closest('main') ?? document.body);
    return () => observer.disconnect();
  }, [onWindow, root, virtual]);

  // The row with the focus stays drawn, else a scroll with the wheel would drop the focus; so does
  // the row the focus is on its way to.
  const focusedRow = useRef<number | null>(null);
  const rangeExtractor = useCallback((range: Range) => {
    const drawn = defaultRangeExtractor(range);
    const row = focusedRow.current;
    if (row === null || row >= range.count || drawn.includes(row)) return drawn;
    return [...drawn, row].sort((a, b) => a - b);
  }, []);
  const viewport = () =>
    scrollElement?.current?.clientHeight ??
    (typeof window === 'undefined' ? 800 : window.innerHeight);
  const overscan = Math.max(1, Math.ceil(viewport() / Math.max(1, estimate)));
  // The virtualizer's results are read during this render only, never memoized downstream.
  // eslint-disable-next-line react-hooks/incompatible-library -- see above
  const virtualizer = useVirtualizer<Element, Element>({
    count: virtual ? rowCount : 0,
    estimateSize: () => estimate,
    gap,
    overscan,
    scrollMargin: onWindow ? margin : 0,
    rangeExtractor,
    // Rows measured while React commits: no synchronous re-render from inside it.
    useFlushSync: false,
    getScrollElement: () =>
      onWindow ? (window as unknown as Element) : (scrollElement.current ?? null),
    ...(onWindow
      ? {
          observeElementRect: observeWindowRect as unknown as typeof observeElementRect,
          observeElementOffset: observeWindowOffset as unknown as typeof observeElementOffset,
          scrollToFn: windowScroll as unknown as typeof elementScroll,
          initialRect: { width: window.innerWidth, height: window.innerHeight },
          initialOffset: () => window.scrollY,
        }
      : {}),
  });

  const items = virtualizer.getVirtualItems();
  const rows = virtual
    ? items
    : Array.from({ length: rowCount }, (_, index) => ({ index, key: index }));
  const first = items[0];
  const last = items.at(-1);
  const before = virtual && first ? first.start - virtualizer.options.scrollMargin : 0;
  const after =
    virtual && last
      ? Math.max(0, virtualizer.getTotalSize() - (last.end - virtualizer.options.scrollMargin))
      : virtual
        ? virtualizer.getTotalSize()
        : 0;

  // The places to read: the visible rows and half a screen around them (or every place).
  const visible = virtualizer.range;
  const extra = Math.ceil(viewport() / 2 / Math.max(1, estimate));
  const spanStart = virtual ? Math.max(0, ((visible?.startIndex ?? 0) - extra) * columns) : 0;
  const spanEnd = virtual
    ? Math.min(count - 1, ((visible?.endIndex ?? 0) + extra + 1) * columns - 1)
    : count - 1;
  useEffect(() => {
    if (count > 0) onRange?.({ start: spanStart, end: spanEnd });
  }, [count, onRange, spanEnd, spanStart]);

  // What the key handlers read: they run later.
  const live = useRef({ count, columns, ready, virtual });
  useEffect(() => {
    live.current = { count, columns, ready, virtual };
  });
  const pending = useRef<Pending | null>(null);

  const placeOf = (index: number) =>
    root.current?.querySelector<HTMLElement>(`[data-place="${index}"]`) ?? null;
  const controls = (place: HTMLElement) => [...place.querySelectorAll<HTMLElement>(FOCUSABLE)];
  const isReady = (index: number) => {
    const place = placeOf(index);
    return live.current.ready(index) && place !== null && controls(place).length > 0;
  };

  const settle = () => {
    const move = pending.current;
    if (!move) return;
    if (move.from !== null && document.activeElement !== move.from) {
      pending.current = null; // the reader went elsewhere meanwhile
      return;
    }
    if (!isReady(move.index)) return;
    pending.current = null;
    const place = placeOf(move.index);
    const list = place ? controls(place) : [];
    (move.part === 'first' ? list[0] : list.at(-1))?.focus();
  };
  // After every render: the row may have been drawn or its page arrived.
  useEffect(settle);

  const go = (index: number, part: 'first' | 'last' = 'first') => {
    pending.current = { index, part, from: document.activeElement };
    const row = Math.floor(index / live.current.columns);
    focusedRow.current = row;
    if (live.current.virtual) virtualizer.scrollToIndex(row, { align: 'auto' });
    settle();
  };

  useEffect(() => {
    const node = root.current;
    if (!node) return;
    const onKeyDown = (event: KeyboardEvent) => {
      // A list drawn whole keeps the keys of its controls and of the spatial navigation.
      if (!live.current.virtual) return;
      if (event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
      if (!(event.target instanceof HTMLElement)) return;
      const place = event.target.closest<HTMLElement>('[data-place]');
      if (!place || !node.contains(place)) return;
      const index = Number(place.dataset.place);
      const { count: total, columns: perRow } = live.current;
      if (event.key === 'Tab') {
        // Past the last control of a place, or before the first, onto a place that is not drawn
        // or still empty: that place comes into view and takes the focus. The list's end lets go.
        const list = controls(place);
        const next = event.shiftKey ? index - 1 : index + 1;
        const edge = event.shiftKey ? list[0] : list.at(-1);
        if (event.target !== edge || next < 0 || next >= total || isReady(next)) return;
        event.preventDefault();
        event.stopPropagation();
        go(next, event.shiftKey ? 'last' : 'first');
        return;
      }
      const direction = arrows && !event.shiftKey ? ARROWS[event.key] : undefined;
      if (!direction) return;
      // Fields, selects and the like keep their arrows.
      if (event.target.closest('input:not([type="checkbox"]), select, textarea')) return;
      const row = Math.floor(index / perRow);
      const lastRow = Math.floor((total - 1) / perRow);
      let target: number | null = null;
      if (direction === 'down' && row < lastRow) target = Math.min(index + perRow, total - 1);
      else if (direction === 'up' && row > 0) target = index - perRow;
      else if (direction === 'right' && perRow > 1 && index % perRow < perRow - 1) {
        target = index + 1 < total ? index + 1 : null;
      } else if (direction === 'left' && perRow > 1 && index % perRow > 0) target = index - 1;
      if (target === null || isReady(target)) return;
      // The spatial navigation (on the window) and the table's own keys know only drawn places.
      event.preventDefault();
      event.stopPropagation();
      go(target);
    };
    const onFocusIn = (event: FocusEvent) => {
      const place =
        event.target instanceof HTMLElement
          ? event.target.closest<HTMLElement>('[data-place]')
          : null;
      focusedRow.current = place
        ? Math.floor(Number(place.dataset.place) / live.current.columns)
        : null;
    };
    const onFocusOut = (event: FocusEvent) => {
      if (!(event.relatedTarget instanceof Node) || !node.contains(event.relatedTarget)) {
        focusedRow.current = null;
      }
    };
    node.addEventListener('keydown', onKeyDown);
    node.addEventListener('focusin', onFocusIn);
    node.addEventListener('focusout', onFocusOut);
    return () => {
      node.removeEventListener('keydown', onKeyDown);
      node.removeEventListener('focusin', onFocusIn);
      node.removeEventListener('focusout', onFocusOut);
    };
  });

  return { rows, before, after, measure: virtualizer.measureElement, go };
}
