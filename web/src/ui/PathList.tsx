import { useEffect, useRef, useState } from 'react';
import { ListBox, ListBoxItem, Text as AriaText } from 'react-aria-components';
import { Icon } from './Icon';
import styles from './PathList.module.css';

export interface PathItem {
  /** Stable key of the row: the path relative to the root, or the id of a root. */
  id: string;
  /** What the row is called: the folder name, or the path of a root. */
  name: string;
  /** Second line, in a monospaced font: the absolute path. */
  detail?: string;
  /** A mark on the row: the library it belongs to, "symlink", "not readable". */
  note?: string;
  /** The row can be shown but not entered (a symlink, a root outside the browsable ones). */
  closed?: boolean;
}

interface PathListProps {
  /** Spoken name of the list. */
  label: string;
  items: readonly PathItem[];
  /** The row that is marked; null for none. */
  selectedId?: string | null;
  onSelectionChange?: (id: string | null) => void;
  /** Enter, Right arrow or a double click on a row. */
  onAction?: (id: string) => void;
  /** Shown instead of the rows when there are none. */
  emptyText: string;
  /**
   * Keys the list itself does not use. The ListBox answers to the arrows, Home, End and Enter;
   * a folder dialog adds Backspace and the sideways arrows on top of that. It is a listener on
   * the element, because ListBox takes no onKeyDown of its own and a wrapper with a key handler
   * would be a non-interactive element pretending to be one.
   */
  onKeyDown?: (event: KeyboardEvent) => void;
  /**
   * Move the focus to the first row as soon as there is one (a dialog that opens on the list).
   * It happens once per mounted list: give the list a key per directory to focus it again.
   */
  autoFocus?: boolean;
}

/**
 * A list of folders with one Tab stop (react-aria ListBox): the arrow keys move from row to row,
 * Home and End jump, Enter enters the folder. SelectableList is a CheckboxGroup with a Tab stop
 * per row, which is the opposite of what a long folder list needs.
 *
 * A closed row stays in the list and keeps its mark, but it cannot be entered: the scanner sees a
 * symlinked folder too, and a folder that is missing from the dialog sends the administrator
 * looking for the mistake in Slouchcat.
 */
export function PathList({
  label,
  items,
  selectedId,
  onSelectionChange,
  onAction,
  emptyText,
  onKeyDown,
  autoFocus,
}: PathListProps) {
  const [node, setNode] = useState<HTMLDivElement | null>(null);
  const handler = useRef(onKeyDown);
  useEffect(() => {
    handler.current = onKeyDown;
  }, [onKeyDown]);
  useEffect(() => {
    if (!node) return;
    const listener = (event: KeyboardEvent) => handler.current?.(event);
    node.addEventListener('keydown', listener);
    return () => node.removeEventListener('keydown', listener);
  }, [node]);
  // The rows arrive with the answer of the server, after the list itself: this runs after every
  // render until there is a row to focus, then stops. The ListBox's own autoFocus is no use
  // here, because the dialog shows a skeleton while the first answer is on its way.
  const pending = useRef(autoFocus === true);
  useEffect(() => {
    if (!pending.current || !node) return;
    const first = node.querySelector<HTMLElement>('[role="option"]');
    if (!first) return;
    pending.current = false;
    first.focus();
  });
  return (
    <ListBox
      ref={setNode}
      aria-label={label}
      className={styles.list}
      items={items}
      selectionMode="single"
      selectedKeys={selectedId ? [selectedId] : []}
      disallowEmptySelection={false}
      onSelectionChange={(keys) => {
        if (keys === 'all') return;
        const [first] = [...keys];
        onSelectionChange?.(first === undefined ? null : String(first));
      }}
      onAction={(key) => onAction?.(String(key))}
      renderEmptyState={() => <div className={styles.empty}>{emptyText}</div>}
    >
      {(item: PathItem) => (
        <ListBoxItem
          id={item.id}
          textValue={item.name}
          className={styles.item}
          data-closed={item.closed ? '' : undefined}
        >
          <Icon name="folder" size="sm" />
          <span className={styles.main}>
            <AriaText slot="label" className={styles.name}>
              {item.name}
            </AriaText>
            {item.detail && (
              <AriaText slot="description" className={styles.detail}>
                {item.detail}
              </AriaText>
            )}
          </span>
          {item.note && <span className={styles.note}>{item.note}</span>}
          {!item.closed && <Icon name="chevronRight" size="sm" />}
        </ListBoxItem>
      )}
    </ListBox>
  );
}
