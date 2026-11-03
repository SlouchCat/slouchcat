import { Fragment, useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { Icon } from './Icon';
import styles from './DataTable.module.css';

export interface DataColumn<T> {
  key: string;
  header: string;
  cell: (row: T) => ReactNode;
  /** The cell names its row for screen readers (`<th scope="row">`), usually the first column. */
  rowHeader?: boolean;
  /** Keep the header for screen readers only (an actions column). */
  hideHeader?: boolean;
  /** Right-aligned content (actions, numbers). */
  align?: 'start' | 'end';
  /**
   * The column's width in a table with `layout="fixed"`: a share of the table (`'24%'`) or a
   * length in rem for content of a known size (a checkbox, a button). Columns without one share
   * what is left equally. Ignored in the automatic layout.
   */
  width?: string;
}

/** Rows that fold out (concept 12, 11.2): what a row shows below itself when it is open. */
export interface DataTableExpand<T> {
  /** The fold-out of a row, in a row of its own below it (`<tr><td colSpan>`). */
  render: (row: T) => ReactNode;
  /** Open rows when the page holds them; uncontrolled (all closed at first) without. */
  expandedKeys?: ReadonlySet<string>;
  onExpandedChange?: (key: string, expanded: boolean) => void;
}

interface DataTableProps<T> {
  /** Name of the table (its caption, visible to screen readers only). */
  caption: string;
  columns: readonly DataColumn<T>[];
  rows: readonly T[];
  rowKey: (row: T) => string;
  /**
   * Rows that fold out: the cell of the row-header column (else the first) becomes a button with
   * `aria-expanded` and `aria-controls`; its content must not hold controls of its own.
   */
  expand?: DataTableExpand<T>;
  /**
   * `fixed`: the columns take their `width`, not the width of their content, so several tables
   * with the same columns (one per group of a page) line up (K12-9). Text wraps inside its
   * column; below a least width the table scrolls sideways in its frame. `auto` (the default):
   * the browser sizes the columns from their content.
   */
  layout?: 'auto' | 'fixed';
}

const FOCUSABLE = 'a[href], button:not(:disabled), input:not(:disabled), [tabindex="0"]';

/**
 * Controls that use ↑ and ↓ themselves (a select picks, a field moves the caret). A checkbox
 * has no use for them and keeps moving between rows.
 */
const OWN_ARROWS =
  'input:not([type="checkbox"]), select, textarea, [contenteditable], [role="radiogroup"], [role="slider"], [role="listbox"], [role="combobox"]';

/** A row of a record, not the fold-out below it. */
const isRecordRow = (row: Element | null): row is HTMLTableRowElement =>
  row instanceof HTMLTableRowElement && row.hasAttribute('data-row-key') && !row.hidden;

/**
 * Moves focus between rows with ↑ and ↓: from a control in one row to the control in the same
 * column of the next row (or its first control). Tab keeps the DOM order and leaves the table.
 * Inside a fold-out and inside controls with arrows of their own the keys stay where they are.
 */
function moveBetweenRows(event: KeyboardEvent) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  const target = event.target as HTMLElement;
  if (target.closest(OWN_ARROWS)) return;
  const cell = target.closest('td, th');
  const row = cell?.parentElement ?? null;
  if (!(cell instanceof HTMLTableCellElement) || !isRecordRow(row)) return;
  const step = (from: Element) =>
    event.key === 'ArrowDown' ? from.nextElementSibling : from.previousElementSibling;
  // Fold-outs, open or closed, are rows of their own; the arrows go from record to record.
  let next = step(row);
  while (next instanceof HTMLTableRowElement && !isRecordRow(next)) next = step(next);
  if (!(next instanceof HTMLTableRowElement)) return;
  const sameColumn = next.cells[cell.cellIndex]?.querySelector<HTMLElement>(FOCUSABLE);
  const destination = sameColumn ?? next.querySelector<HTMLElement>(FOCUSABLE);
  if (!destination) return;
  event.preventDefault();
  destination.focus();
}

/**
 * A table of records (devices, streams) with header row and row headers. Each row's controls
 * (buttons, links) are in the Tab order; ↑/↓ jump between rows. On narrow screens the table
 * scrolls sideways inside its frame instead of widening the page. With `expand` a row folds out
 * into a row of its own below it; the table, its headers and the roles row/cell stay as they are.
 */
export function DataTable<T>({
  caption,
  columns,
  rows,
  rowKey,
  expand,
  layout = 'auto',
}: DataTableProps<T>) {
  const id = useId();
  const [ownExpanded, setOwnExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const expanded = expand?.expandedKeys ?? ownExpanded;
  const toggle = (key: string) => {
    const open = !expanded.has(key);
    const next = new Set(ownExpanded);
    if (open) next.add(key);
    else next.delete(key);
    setOwnExpanded(next);
    expand?.onExpandedChange?.(key, open);
  };
  const toggleColumn = expand
    ? (columns.find((column) => column.rowHeader) ?? columns[0])?.key
    : undefined;
  // A native listener: the rows are not widgets of their own, only their controls are.
  const body = useRef<HTMLTableSectionElement>(null);
  useEffect(() => {
    const node = body.current;
    if (!node) return;
    node.addEventListener('keydown', moveBetweenRows);
    return () => node.removeEventListener('keydown', moveBetweenRows);
  }, []);
  return (
    <div className={styles.frame}>
      <table className={`${styles.table} ${layout === 'fixed' ? styles.fixed : ''}`}>
        <caption className={styles.caption}>{caption}</caption>
        {layout === 'fixed' && (
          <colgroup>
            {columns.map((column) => (
              <col
                key={column.key}
                style={column.width === undefined ? undefined : { width: column.width }}
              />
            ))}
          </colgroup>
        )}
        <thead>
          <tr>
            {columns.map((column) => (
              <th
                key={column.key}
                scope="col"
                className={`${styles.head} ${column.align === 'end' ? styles.end : ''}`}
              >
                {column.hideHeader ? (
                  <span className={styles.caption}>{column.header}</span>
                ) : (
                  column.header
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody ref={body}>
          {rows.map((row, index) => {
            const key = rowKey(row);
            const open = expanded.has(key);
            const detailId = `${id}-${index}`;
            return (
              <Fragment key={key}>
                <tr className={styles.row} data-row-key={key}>
                  {columns.map((column) => {
                    const className = `${styles.cell} ${column.align === 'end' ? styles.end : ''}`;
                    const content =
                      column.key === toggleColumn ? (
                        <button
                          type="button"
                          className={styles.toggle}
                          aria-expanded={open}
                          aria-controls={detailId}
                          onClick={() => toggle(key)}
                        >
                          <span className={`${styles.chevron} ${open ? styles.open : ''}`}>
                            <Icon name="chevronRight" size="sm" />
                          </span>
                          {column.cell(row)}
                        </button>
                      ) : (
                        column.cell(row)
                      );
                    return column.rowHeader ? (
                      <th key={column.key} scope="row" className={className}>
                        {content}
                      </th>
                    ) : (
                      <td key={column.key} className={className}>
                        {content}
                      </td>
                    );
                  })}
                </tr>
                {expand && (
                  <tr className={styles.detailRow} hidden={!open}>
                    <td id={detailId} colSpan={columns.length} className={styles.detail}>
                      {open && expand.render(row)}
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
