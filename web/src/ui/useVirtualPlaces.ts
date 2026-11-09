import { defaultRangeExtractor, useWindowVirtualizer, type Range } from '@tanstack/react-virtual';
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type RefObject } from 'react';

// The drawn part of a long list (concept 18 C-3): its places in rows of `columns`, only the rows
// in view and a screen's height above and below in the document once the list is `virtual`, each
// row measured as it is drawn. The keys follow concept 17 6.2 point 2: an arrow press or Tab
// towards a place that is not drawn or still empty scrolls its row into view and focuses it once
// its entry is there, on the same control it left (the same column of a table); the row with the
// focus stays drawn. In a grid ← and → first go through the controls of a place (a tile's heart)
// before they leave it. VirtualList (lists and grids) and DataTable (tables) draw the rows; the
// window scrolls them.

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
  /** A row's expected height in px (without the gap), until it is measured. */
  estimate: number;
  /** The gap between two rows in px (the container's CSS row gap). */
  gap?: number;
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

/** A row to draw: its index and the room to leave above it for the rows that are not drawn. */
export interface DrawnRow {
  index: number;
  /**
   * Height in px of the room above this row (the rows before it that are not drawn), less the
   * container's gaps around that room; 0 when the row follows the one drawn before it.
   */
  space: number;
}

export interface VirtualPlaces {
  /** The rows to draw, in order. */
  rows: readonly DrawnRow[];
  /** The room below the last drawn row, in px (less one gap), as `space`. */
  after: number;
  /** Measures a drawn row: the element carries `data-index` (the row's index). */
  measure: (element: Element | null) => void;
  /**
   * Scrolls place `index` into view and focuses one of its controls once its entry is there: the
   * first, the last, or the one at this position (the last of fewer).
   */
  go: (index: number, control?: Control) => void;
}

/** A control of a place: the first, the last, or the one at this position among its controls. */
export type Control = 'first' | 'last' | number;

/** The controls of a place that take the focus. */
const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), [tabindex="0"]';

/** The height of the app's fixed header in px (the token), 0 without one. */
function headerHeight(): number {
  const probe = document.createElement('div');
  probe.style.cssText =
    'position:absolute;visibility:hidden;pointer-events:none;height:var(--sc-size-header-height)';
  document.body.append(probe);
  const height = probe.getBoundingClientRect().height;
  probe.remove();
  return height;
}

interface Pending {
  index: number;
  control: Control;
  /** The column of a table row whose control takes the focus, if that row has one there. */
  column: number | null;
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
  root,
  onRange,
  ready,
  arrows = false,
}: VirtualPlacesOptions): VirtualPlaces {
  const rowCount = Math.ceil(count / Math.max(1, columns));

  // The list's top in the document: the window's scroll margin. It follows what stands above.
  // The fixed header covers the top of the window: a row scrolled up into view stops below it.
  const [margin, setMargin] = useState(0);
  const [header, setHeader] = useState(0);
  useLayoutEffect(() => {
    const node = root.current;
    if (!node || !virtual) return;
    const update = () => {
      setMargin(Math.round(node.getBoundingClientRect().top + window.scrollY));
      setHeader(headerHeight());
    };
    update();
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(node.closest('main') ?? document.body);
    return () => observer.disconnect();
  }, [root, virtual]);

  // The row with the focus stays drawn, else a scroll with the wheel would drop the focus; so does
  // the row the focus is on its way to.
  const focusedRow = useRef<number | null>(null);
  const rangeExtractor = useCallback((range: Range) => {
    const drawn = defaultRangeExtractor(range);
    const row = focusedRow.current;
    if (row === null || row >= range.count || drawn.includes(row)) return drawn;
    return [...drawn, row].sort((a, b) => a - b);
  }, []);
  const viewport = () => (typeof window === 'undefined' ? 800 : window.innerHeight);
  const overscan = Math.max(1, Math.ceil(viewport() / Math.max(1, estimate)));
  const virtualizer = useWindowVirtualizer<Element>({
    count: virtual ? rowCount : 0,
    estimateSize: () => estimate,
    gap,
    overscan,
    scrollMargin: margin,
    scrollPaddingStart: header,
    rangeExtractor,
    // Rows measured while React commits: no synchronous re-render from inside it.
    useFlushSync: false,
    initialRect: { width: window.innerWidth, height: window.innerHeight },
    initialOffset: () => window.scrollY,
  });

  // The drawn rows with the room for the others: a room of its own before a row that does not
  // follow the one before it (the row with the focus stays drawn far from the view), so every row
  // keeps its place in the document. A room between two rows takes two of the container's gaps.
  const items = virtualizer.getVirtualItems();
  const margin0 = virtualizer.options.scrollMargin;
  const rows: DrawnRow[] = virtual
    ? items.map((item, n) => {
        const previous = items[n - 1];
        if (previous === undefined) {
          return { index: item.index, space: item.index > 0 ? item.start - margin0 - gap : 0 };
        }
        return {
          index: item.index,
          space: item.index === previous.index + 1 ? 0 : item.start - previous.end - 2 * gap,
        };
      })
    : Array.from({ length: rowCount }, (_, index) => ({ index, space: 0 }));
  const last = items.at(-1);
  const after =
    virtual && last && last.index < rowCount - 1
      ? Math.max(0, virtualizer.getTotalSize() - (last.end - margin0) - gap)
      : virtual && !last
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

  // The control of `place` that takes the focus: the one in the same column of a table row (if it
  // has one), else the one at the position (the last of fewer).
  const controlOf = (place: HTMLElement, control: Control, column: number | null) => {
    if (column !== null && place instanceof HTMLTableRowElement) {
      const inColumn = place.cells[column]?.querySelector<HTMLElement>(FOCUSABLE);
      if (inColumn) return inColumn;
    }
    const list = controls(place);
    if (control === 'first') return list[0];
    if (control === 'last') return list.at(-1);
    return list[Math.min(control, list.length - 1)];
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
    if (place) controlOf(place, move.control, move.column)?.focus();
  };
  // After every render: the row may have been drawn or its page arrived.
  useEffect(settle);

  const go = (index: number, control: Control = 'first', column: number | null = null) => {
    pending.current = { index, control, column, from: document.activeElement };
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
      if (event.altKey || event.ctrlKey || event.metaKey) return;
      // A handler before this one moved the focus already (a table's own ↑/↓). Not
      // `defaultPrevented`: a browsing page prevents every arrow's scroll (SpatialPage).
      if (!(event.target instanceof HTMLElement) || document.activeElement !== event.target) return;
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
      // The control the focus leaves: the next place takes the focus on the same one.
      const own = controls(place);
      const position = Math.max(0, own.indexOf(event.target));
      const cell = event.target.closest('td, th');
      const column = cell instanceof HTMLTableCellElement ? cell.cellIndex : null;
      // In a grid ← and → go through the controls of a place first (a tile's heart), as the
      // spatial navigation does in a grid that is drawn whole.
      if (perRow > 1 && (direction === 'left' || direction === 'right')) {
        const inside = own[position + (direction === 'right' ? 1 : -1)];
        if (inside && own.includes(event.target)) {
          event.preventDefault();
          event.stopPropagation();
          inside.focus();
          return;
        }
      }
      const row = Math.floor(index / perRow);
      const lastRow = Math.floor((total - 1) / perRow);
      let target: number | null = null;
      if (direction === 'down' && row < lastRow) target = Math.min(index + perRow, total - 1);
      else if (direction === 'up' && row > 0) target = index - perRow;
      else if (direction === 'right' && perRow > 1 && index % perRow < perRow - 1) {
        target = index + 1 < total ? index + 1 : null;
      } else if (direction === 'left' && perRow > 1 && index % perRow > 0) target = index - 1;
      if (target === null) return; // the list's edge: the spatial navigation leaves it
      // Between the places of a virtual list the list moves the focus itself: the spatial
      // navigation (on the window) knows only what is drawn and measures rows that come and go.
      // A table's own keys went first (DataTable): they leave only a row that is not drawn.
      event.preventDefault();
      event.stopPropagation();
      // Sideways a place is entered at the control next to the one left: → at its first, ← at
      // its last; up and down on the same control.
      const control: Control =
        direction === 'right' ? 'first' : direction === 'left' ? 'last' : position;
      const next = placeOf(target);
      if (isReady(target) && next) controlOf(next, control, column)?.focus();
      else go(target, control, column);
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

  return { rows, after, measure: virtualizer.measureElement, go };
}
