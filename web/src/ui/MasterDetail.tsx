import { useRef, type FocusEvent, type ReactNode } from 'react';
import {
  Header,
  ListBox,
  ListBoxItem,
  ListBoxSection,
  Text,
  type Selection,
} from 'react-aria-components';
import styles from './MasterDetail.module.css';

export interface MasterDetailItem {
  id: string;
  label: string;
  /** A second line ("Entwurf", "3 von 5 Profilen gelesen"). */
  description?: string;
  /** Items with the same section stand together under its title ("Entwürfe"). */
  section?: string;
}

interface MasterDetailProps {
  /** Spoken name of the list ("Bereiche"). */
  label: string;
  items: readonly MasterDetailItem[];
  selectedId: string;
  onSelect: (id: string) => void;
  /** The detail of the selected item. */
  children: ReactNode;
}

/** Consecutive items of one section, in their order. */
function sectionsOf(items: readonly MasterDetailItem[]) {
  const sections: { title?: string; items: MasterDetailItem[] }[] = [];
  for (const item of items) {
    const last = sections.at(-1);
    if (last && last.title === item.section) last.items.push(item);
    else sections.push({ title: item.section, items: [item] });
  }
  return sections;
}

/**
 * A list on the left and the detail of its selected item on the right (concept 12, 11.2, variant
 * B). The list is one Tab stop (react-aria ListBox): ↑ and ↓ move, Enter or Space selects, Tab
 * moves on into the detail; coming back into the list, the focus lands on the selected item. A
 * page keeps the selection in `?item=` (useSearchSelection, S-14);
 * what was typed in the detail lives in the page's state (useDrafts, S-17). On narrow screens the
 * detail follows the list.
 */
export function MasterDetail({ label, items, selectedId, onSelect, children }: MasterDetailProps) {
  const listRef = useRef<HTMLDivElement>(null);
  // react-aria brings the focus back to the item that had it last, which need not be the
  // selected one (the reader moved with the arrows and left, or the page changed the selection).
  // Entering from outside, the selected item takes it; react-aria follows that focus.
  const onFocus = (event: FocusEvent<HTMLDivElement>) => {
    const list = listRef.current;
    if (!list?.contains(event.target)) return;
    if (event.relatedTarget instanceof Node && list.contains(event.relatedTarget)) return;
    const selected = list.querySelector<HTMLElement>('[role="option"][aria-selected="true"]');
    if (selected && selected !== event.target) selected.focus();
  };
  const option = (item: MasterDetailItem) => (
    <ListBoxItem key={item.id} id={item.id} textValue={item.label} className={styles.item}>
      <Text slot="label">{item.label}</Text>
      {item.description && (
        <Text slot="description" className={styles.description}>
          {item.description}
        </Text>
      )}
    </ListBoxItem>
  );
  return (
    <div className={styles.frame} onFocus={onFocus}>
      <ListBox
        ref={listRef}
        aria-label={label}
        className={styles.list}
        selectionMode="single"
        disallowEmptySelection
        selectedKeys={[selectedId]}
        onSelectionChange={(keys: Selection) => {
          if (keys === 'all') return;
          const [key] = [...keys];
          if (key !== undefined) onSelect(String(key));
        }}
      >
        {sectionsOf(items).map((section, index) =>
          section.title === undefined ? (
            section.items.map(option)
          ) : (
            <ListBoxSection key={`${section.title}-${index}`} className={styles.section}>
              <Header className={styles.header}>{section.title}</Header>
              {section.items.map(option)}
            </ListBoxSection>
          ),
        )}
      </ListBox>
      <div className={styles.detail}>{children}</div>
    </div>
  );
}
