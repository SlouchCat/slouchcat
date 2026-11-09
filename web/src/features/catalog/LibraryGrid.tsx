import type { ItemRef } from '@slouchcat/api-client';
import { defaultRangeExtractor, useWindowVirtualizer, type Range } from '@tanstack/react-virtual';
import {
  useCallback,
  useEffect,
  useImperativeHandle,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
  type Ref,
} from 'react';
import { cardKey, ItemCell } from '@/features/discover/ItemGrid';
import { setSpatialFocus, Skeleton, SpatialGroup } from '@/ui';
import type { DrawnRange } from './libraryPages';
import { useBackFocus } from './useBackFocus';
import styles from './LibraryGrid.module.css';

// The library grid of concept 17 section 6 (E-11 to E-13): a place for every title of the list,
// only the rows in view (and a screen's height above and below) drawn, each row of a fixed height
// computed from the column width, so a jump or the way back needs no measuring. The spatial
// navigation knows only mounted cards; an arrow press or Tab towards a place that is not drawn or
// still empty is taken over here: the row scrolls into view, its page loads, and the card gets
// the focus once it is there.

/** What the page can ask of the grid (the letter bar's jump). */
export interface LibraryGridHandle {
  /** Scrolls the place `index` to the top and focuses its card once it is drawn. */
  jump: (index: number) => void;
}

interface LibraryGridProps {
  /** One place per title of the list, undefined while its page is not there. */
  slots: readonly (ItemRef | undefined)[];
  /** Focus key prefix: the grid is `<prefix>`, each card `<prefix>-<id>`. */
  focusKey: string;
  label: string;
  /** Square covers (audiobooks and podcasts). */
  square: boolean;
  tool?: (item: ItemRef) => ReactNode;
  /** The drawn places (for their pages) and the first place in view. */
  onRange: (range: DrawnRange, top: number) => void;
  ref?: Ref<LibraryGridHandle>;
}

/** The sizes the grid takes from its CSS (columns and gaps) and its probe (text, header). */
interface Metrics {
  columns: number;
  /** Height of one card: the cover from the column width plus the text under it. */
  card: number;
  rowGap: number;
  /** Top of the grid in the document (the virtualizer's scroll margin). */
  top: number;
  header: number;
}

function measure(list: HTMLElement, probe: HTMLElement, square: boolean): Metrics {
  const style = getComputedStyle(list);
  const tracks = style.gridTemplateColumns
    .split(' ')
    .map((track) => parseFloat(track))
    .filter((track) => Number.isFinite(track) && track > 0);
  const width = tracks[0] ?? list.getBoundingClientRect().width;
  const text = probe.getBoundingClientRect();
  return {
    columns: Math.max(1, tracks.length),
    card: Math.ceil(width * (square ? 1 : 1.5) + text.height),
    rowGap: parseFloat(style.rowGap) || 0,
    top: list.getBoundingClientRect().top + window.scrollY,
    header: text.width,
  };
}

const same = (a: Metrics | null, b: Metrics) =>
  a !== null &&
  a.columns === b.columns &&
  a.card === b.card &&
  a.rowGap === b.rowGap &&
  a.top === b.top &&
  a.header === b.header;

/** A focus the grid hands on once its card is drawn: the card's link or the cell's last control. */
interface Pending {
  index: number;
  part: 'card' | 'last';
  /** The element focused when the move began; another focus meanwhile cancels it. */
  from: Element | null;
}

const FOCUSABLE = 'a[href], button:not([disabled])';

export function LibraryGrid({
  slots,
  focusKey,
  label,
  square,
  tool,
  onRange,
  ref,
}: LibraryGridProps) {
  const root = useRef<HTMLDivElement>(null);
  const probe = useRef<HTMLSpanElement>(null);
  const [metrics, setMetrics] = useState<Metrics | null>(null);

  useLayoutEffect(() => {
    const node = root.current?.querySelector('ul');
    const text = probe.current;
    if (!node || !text) return;
    // Columns and gaps follow the width; the top follows what stands above (filters, chips).
    const observer = new ResizeObserver(() => {
      const next = measure(node, text, square);
      setMetrics((previous) => (same(previous, next) ? previous : next));
    });
    observer.observe(node);
    observer.observe(node.closest('main') ?? document.body);
    return () => observer.disconnect();
  }, [square]);

  const total = slots.length;
  const columns = metrics?.columns ?? 1;
  const rows = metrics ? Math.ceil(total / columns) : 0;
  const rowSize = metrics ? metrics.card + metrics.rowGap : 1;
  // The row with the focus stays drawn, else a scroll with the wheel would drop the focus.
  const focusedRow = useRef<number | null>(null);
  const rangeExtractor = useCallback((range: Range) => {
    const drawn = defaultRangeExtractor(range);
    const row = focusedRow.current;
    if (row === null || row >= range.count || drawn.includes(row)) return drawn;
    return [...drawn, row].sort((a, b) => a - b);
  }, []);
  const overscan = Math.max(1, Math.ceil(window.innerHeight / rowSize));
  const virtualizer = useWindowVirtualizer({
    count: rows,
    estimateSize: () => rowSize,
    overscan,
    scrollMargin: metrics?.top ?? 0,
    scrollPaddingStart: metrics?.header ?? 0,
    initialRect: { width: window.innerWidth, height: window.innerHeight },
    rangeExtractor,
  });
  // A new row height (another width) replaces every position.
  useLayoutEffect(() => virtualizer.measure(), [virtualizer, rowSize]);

  const virtualRows = virtualizer.getVirtualItems();
  const visible = virtualizer.range;
  const firstRow = Math.max(0, (visible?.startIndex ?? 0) - overscan);
  const lastRow = Math.min(rows - 1, (visible?.endIndex ?? 0) + overscan);
  const drawnStart = firstRow * columns;
  const drawnEnd = rows > 0 ? (lastRow + 1) * columns - 1 : -1;
  const top = (visible?.startIndex ?? 0) * columns;
  useEffect(() => {
    if (rows > 0) onRange({ start: drawnStart, end: drawnEnd }, top);
  }, [drawnEnd, drawnStart, onRange, rows, top]);

  const indexes = useMemo(
    () =>
      virtualRows.flatMap((row) =>
        Array.from({ length: columns }, (_, column) => row.index * columns + column).filter(
          (index) => index < total,
        ),
      ),
    [columns, total, virtualRows],
  );

  // What the key handlers read: they are handed to norigin once and run later.
  const live = useRef({ slots, columns, total });
  useEffect(() => {
    live.current = { slots, columns, total };
  });
  const pending = useRef<Pending | null>(null);

  const cell = (index: number) =>
    root.current?.querySelector<HTMLElement>(`[data-index="${index}"]`) ?? null;
  const ready = (index: number) =>
    live.current.slots[index] !== undefined && cell(index)?.querySelector('a[href]') != null;

  const settle = () => {
    const move = pending.current;
    if (!move) return;
    if (move.from !== null && document.activeElement !== move.from) {
      pending.current = null; // the reader went elsewhere meanwhile
      return;
    }
    const item = live.current.slots[move.index];
    const target = cell(move.index);
    if (!item || !target?.querySelector('a[href]')) return;
    pending.current = null;
    if (move.part === 'card') {
      setSpatialFocus(cardKey(focusKey, item));
    } else {
      [...target.querySelectorAll<HTMLElement>(FOCUSABLE)].at(-1)?.focus();
    }
  };
  // After every render: the row may have been drawn or its page arrived.
  useEffect(settle);

  const go = (index: number, part: Pending['part'], align: 'auto' | 'start' = 'auto') => {
    pending.current = { index, part, from: document.activeElement };
    virtualizer.scrollToIndex(Math.floor(index / live.current.columns), { align });
    settle();
  };

  useImperativeHandle(ref, () => ({ jump: (index: number) => go(index, 'card', 'start') }));

  // Arrows (E-12): towards a place that is not drawn or still empty the card keeps the press.
  const arrow = (index: number, direction: string): boolean => {
    const { columns: perRow, total: count } = live.current;
    const row = Math.floor(index / perRow);
    const last = Math.floor((count - 1) / perRow);
    let target: number | null = null;
    if (direction === 'down' && row < last) target = Math.min(index + perRow, count - 1);
    else if (direction === 'up' && row > 0) target = index - perRow;
    else if (direction === 'right' && index % perRow < perRow - 1 && index + 1 < count) {
      target = index + 1;
    } else if (direction === 'left' && index % perRow > 0) target = index - 1;
    if (target === null || ready(target)) return true;
    go(target, 'card');
    return false;
  };

  // Tab (E-12): past the last drawn card of a longer list, or onto an empty place, the next card
  // comes into view and takes the focus; Shift+Tab the other way. The end of the list lets go.
  useEffect(() => {
    const node = root.current;
    if (!node) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Tab' || event.altKey || event.ctrlKey || event.metaKey) return;
      if (!(event.target instanceof HTMLElement)) return;
      const place = event.target.closest<HTMLElement>('[data-index]');
      if (!place) return;
      const index = Number(place.dataset.index);
      const controls = [...place.querySelectorAll<HTMLElement>(FOCUSABLE)];
      const next = event.shiftKey ? index - 1 : index + 1;
      const edge = event.shiftKey ? controls[0] : controls.at(-1);
      if (event.target !== edge || next < 0 || next >= live.current.total || ready(next)) return;
      event.preventDefault();
      go(next, event.shiftKey ? 'last' : 'card');
    };
    const onFocusIn = (event: FocusEvent) => {
      const place =
        event.target instanceof HTMLElement
          ? event.target.closest<HTMLElement>('[data-index]')
          : null;
      focusedRow.current = place
        ? Math.floor(Number(place.dataset.index) / live.current.columns)
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

  // Back (E-13): the keys of the drawn cards; a card not drawn is brought back by its place.
  const keys = useMemo(
    () =>
      indexes.flatMap((index) => {
        const item = slots[index];
        return item ? [cardKey(focusKey, item)] : [];
      }),
    [focusKey, indexes, slots],
  );
  useBackFocus(focusKey, keys, {
    indexOf: (key) => {
      const index = live.current.slots.findIndex(
        (item) => item !== undefined && cardKey(focusKey, item) === key,
      );
      return index < 0 ? undefined : index;
    },
    reveal: (index) => {
      if (cell(index)) return; // drawn: the router's scroll stands, its page is on the way
      virtualizer.scrollToIndex(Math.floor(index / live.current.columns), { align: 'center' });
    },
  });

  return (
    <div
      ref={root}
      className={styles.root}
      // One row of the fixed card height per row of the list (LibraryGrid.module.css .grid).
      style={
        metrics
          ? ({ '--library-rows': rows, '--library-card': `${metrics.card}px` } as CSSProperties)
          : undefined
      }
    >
      <span ref={probe} className={styles.probe} aria-hidden="true" />
      <SpatialGroup as="ul" focusKey={focusKey} className={styles.grid} aria-label={label}>
        {indexes.map((index) => {
          const item = slots[index];
          const placement = {
            gridRow: Math.floor(index / columns) + 1,
            gridColumn: (index % columns) + 1,
          };
          return (
            <li
              key={item ? item.id : `place-${index}`}
              className={styles.cell}
              style={placement}
              data-index={index}
              aria-setsize={total}
              aria-posinset={index + 1}
            >
              {item ? (
                <ItemCell
                  item={item}
                  focusKey={focusKey}
                  showWatched
                  tool={tool}
                  clampCaption
                  onArrowPress={(direction) => arrow(index, direction)}
                />
              ) : (
                <span
                  className={square ? `${styles.placeholder} ${styles.square}` : styles.placeholder}
                >
                  <Skeleton shape="poster" />
                </span>
              )}
            </li>
          );
        })}
      </SpatialGroup>
    </div>
  );
}
