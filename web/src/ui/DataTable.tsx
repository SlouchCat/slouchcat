import { useEffect, useRef, type ReactNode } from 'react';
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
}

interface DataTableProps<T> {
  /** Name of the table (its caption, visible to screen readers only). */
  caption: string;
  columns: readonly DataColumn<T>[];
  rows: readonly T[];
  rowKey: (row: T) => string;
}

const FOCUSABLE = 'a[href], button:not(:disabled), input:not(:disabled), [tabindex="0"]';

/**
 * Moves focus between rows with ↑ and ↓: from a control in one row to the control in the same
 * column of the next row (or its first control). Tab keeps the DOM order and leaves the table.
 */
function moveBetweenRows(event: KeyboardEvent) {
  if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  const target = event.target as HTMLElement;
  const cell = target.closest('td, th');
  const row = cell?.parentElement;
  if (!(cell instanceof HTMLTableCellElement) || !(row instanceof HTMLTableRowElement)) return;
  const next = event.key === 'ArrowDown' ? row.nextElementSibling : row.previousElementSibling;
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
 * scrolls sideways inside its frame instead of widening the page.
 */
export function DataTable<T>({ caption, columns, rows, rowKey }: DataTableProps<T>) {
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
      <table className={styles.table}>
        <caption className={styles.caption}>{caption}</caption>
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
          {rows.map((row) => (
            <tr key={rowKey(row)} className={styles.row} data-row-key={rowKey(row)}>
              {columns.map((column) => {
                const className = `${styles.cell} ${column.align === 'end' ? styles.end : ''}`;
                return column.rowHeader ? (
                  <th key={column.key} scope="row" className={className}>
                    {column.cell(row)}
                  </th>
                ) : (
                  <td key={column.key} className={className}>
                    {column.cell(row)}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
