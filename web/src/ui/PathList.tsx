import type { Ref } from 'react';
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
  /** The list element, so a dialog can move the focus into it when it opens. */
  listRef?: Ref<HTMLDivElement>;
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
  listRef,
}: PathListProps) {
  return (
    <ListBox
      ref={listRef}
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
