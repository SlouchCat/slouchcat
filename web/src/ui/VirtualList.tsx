import { useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import { SpatialGroup } from './spatial';
import { useVirtualPlaces, type PlaceSpan } from './useVirtualPlaces';

interface VirtualListProps<T> {
  /** One place per entry of the list, undefined while its page is not there. */
  slots: readonly (T | undefined)[];
  /** Draw only the rows near the view (a long list, concept 18 C-3); else every entry. */
  virtual: boolean;
  /**
   * `grid`: the list's class lays the places out in columns (CSS grid) and a row is as many
   * places; `list` (default): one place per row.
   */
  layout?: 'list' | 'grid';
  /** Class of the list (`<ul>`): its columns, gaps and look. */
  className?: string;
  /** Class of each place (`<li>`). */
  cellClassName?: string;
  /** Spoken name of the list. */
  label: string;
  /** A row's expected height in px, or one from the column's width (a grid of square tiles). */
  estimate: number | ((columnWidth: number) => number);
  /** The entry's key in the list. */
  itemKey: (item: T) => string | number;
  renderItem: (item: T, index: number) => ReactNode;
  /** An empty place while its page is on its way (not focusable). */
  renderPlaceholder: (index: number) => ReactNode;
  /** The places to read (useVirtualPlaces). */
  onRange?: (span: PlaceSpan) => void;
  /** Focus key of the list for the spatial navigation. */
  focusKey?: string;
  /** The element that scrolls the list; the window when left out. */
  scrollElement?: RefObject<HTMLElement | null>;
}

interface Layout {
  columns: number;
  columnWidth: number;
  gap: number;
}

function measureLayout(list: HTMLElement): Layout {
  const style = getComputedStyle(list);
  const tracks = style.gridTemplateColumns
    .split(' ')
    .map((track) => parseFloat(track))
    .filter((track) => Number.isFinite(track) && track > 0);
  return {
    columns: Math.max(1, tracks.length),
    columnWidth: tracks[0] ?? list.getBoundingClientRect().width,
    gap: parseFloat(style.rowGap) || 0,
  };
}

/**
 * A list or a grid of a list operation's entries (concept 18 C-3): every entry while the list is
 * short, only the rows near the view once it is `virtual`, with room for the rest above and below
 * so the scroll bar stands for the whole list. Arrow keys (spatial navigation) and Tab reach the
 * places that are not drawn yet: their row scrolls into view and the entry takes the focus once
 * its page is there (useVirtualPlaces).
 */
export function VirtualList<T>({
  slots,
  virtual,
  layout = 'list',
  className,
  cellClassName,
  label,
  estimate,
  itemKey,
  renderItem,
  renderPlaceholder,
  onRange,
  focusKey,
  scrollElement,
}: VirtualListProps<T>) {
  const root = useRef<HTMLDivElement>(null);
  const [shape, setShape] = useState<Layout>({ columns: 1, columnWidth: 0, gap: 0 });
  useLayoutEffect(() => {
    const list = root.current?.querySelector('ul');
    if (!list) return;
    const update = () => {
      const next = measureLayout(list);
      if (layout === 'list') next.columns = 1;
      setShape((previous) =>
        previous.columns === next.columns &&
        previous.columnWidth === next.columnWidth &&
        previous.gap === next.gap
          ? previous
          : next,
      );
    };
    update();
    // Columns and gaps follow the width (no observer without layout, as in a test's jsdom).
    if (typeof ResizeObserver === 'undefined') return;
    const observer = new ResizeObserver(update);
    observer.observe(list);
    return () => observer.disconnect();
  }, [layout]);

  const { columns } = shape;
  // Room for rows that are not drawn: a whole row of the grid, not an entry of the list.
  const spacer = (height: number, before: number) => (
    <li
      key={`space-${before}`}
      role="presentation"
      aria-hidden="true"
      style={{ height, gridColumn: '1 / -1' }}
    />
  );
  const size = typeof estimate === 'number' ? estimate : estimate(shape.columnWidth);
  const places = useVirtualPlaces({
    count: slots.length,
    virtual,
    columns,
    estimate: size + shape.gap,
    gap: shape.gap,
    scrollElement,
    root,
    onRange,
    ready: (index) => slots[index] !== undefined,
    arrows: true,
  });

  return (
    <div ref={root}>
      <SpatialGroup as="ul" focusKey={focusKey} className={className} aria-label={label}>
        {places.rows.flatMap((row) => [
          ...(row.space > 0 ? [spacer(row.space, row.index)] : []),
          ...Array.from({ length: columns }, (_, column) => row.index * columns + column)
            .filter((index) => index < slots.length)
            .map((index, column) => {
              const item = slots[index];
              return (
                <li
                  key={item === undefined ? `place-${index}` : itemKey(item)}
                  className={cellClassName}
                  data-place={index}
                  // The row's first place is measured for the row (a grid stretches it).
                  {...(virtual && column === 0
                    ? { 'data-index': row.index, ref: places.measure }
                    : {})}
                  // An empty place holds a row's expected height until its entry is there.
                  style={item === undefined ? { minHeight: size } : undefined}
                  aria-setsize={slots.length}
                  aria-posinset={index + 1}
                >
                  {item === undefined ? renderPlaceholder(index) : renderItem(item, index)}
                </li>
              );
            }),
        ])}
        {places.after > 0 && spacer(places.after, -1)}
      </SpatialGroup>
    </div>
  );
}
