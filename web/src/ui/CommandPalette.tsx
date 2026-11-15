import { useEffect, useId, useState, type KeyboardEvent } from 'react';
import { Dialog, Modal, ModalOverlay } from 'react-aria-components';
import { useAppNavigate } from './AppLink';
import styles from './CommandPalette.module.css';
import { focusLost } from './focusAfter';
import { focusOnMount } from './focusOnMount';
import { Icon, type IconName } from './Icon';

/** One destination of the palette. */
export interface CommandPaletteItem {
  /** Unique among the items. */
  id: string;
  title: string;
  /** Shown after the title and searched with it ("Medien" for a page of that area). */
  detail?: string;
  icon: IconName;
  /** Where Enter or a click leads. */
  href: string;
}

export interface CommandPaletteProps {
  isOpen: boolean;
  /** Escape, a press on the scrim and following an item close it through this. */
  onOpenChange: (isOpen: boolean) => void;
  /** Accessible name of the dialog, also the field's ("Bereich der Verwaltung suchen"). */
  label: string;
  placeholder: string;
  /** Accessible name of the result list ("Treffer"). */
  resultsLabel: string;
  /** Instead of the list when nothing matches ("Nichts gefunden"). */
  emptyLabel: string;
  /** The keys under the list ("Pfeiltasten wählen, Enter öffnet, Esc schliesst"). */
  hint?: string;
  items: readonly CommandPaletteItem[];
}

/** The items whose title or detail contain every typed word, ignoring case. */
export function filterCommandItems(
  items: readonly CommandPaletteItem[],
  query: string,
): CommandPaletteItem[] {
  const words = query.toLocaleLowerCase().split(/\s+/).filter(Boolean);
  return items.filter((item) => {
    const text = `${item.title} ${item.detail ?? ''}`.toLocaleLowerCase();
    return words.every((word) => text.includes(word));
  });
}

/**
 * A search palette (build plan 2.4): a modal dialog with a search field and the list of
 * destinations below it. The field has the focus and keeps it (a combobox with
 * `aria-activedescendant`): typing filters by title and detail, ArrowDown and ArrowUp choose
 * (wrapping), Enter follows the chosen item, Escape closes. The focus stays inside while it is
 * open (react-aria) and returns to what had it before, the button that opened it.
 */
export function CommandPalette({ isOpen, onOpenChange, ...props }: CommandPaletteProps) {
  return (
    <ModalOverlay
      className={styles.overlay}
      isOpen={isOpen}
      isDismissable
      onOpenChange={onOpenChange}
    >
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog} aria-label={props.label}>
          <PaletteBody {...props} close={() => onOpenChange(false)} />
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}

type PaletteBodyProps = Omit<CommandPaletteProps, 'isOpen' | 'onOpenChange'> & {
  close: () => void;
};

/** The field and the list; mounted with the dialog, so every opening starts empty. */
function PaletteBody({
  label,
  placeholder,
  resultsLabel,
  emptyLabel,
  hint,
  items,
  close,
}: PaletteBodyProps) {
  const id = useId();
  const navigate = useAppNavigate();
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  // What had the focus when the palette opened (the button): it takes the focus back when the
  // palette closes. react-aria's own return comes too late here: the field took the focus first.
  const [opener] = useState(() => document.activeElement);
  useEffect(
    () => () => {
      queueMicrotask(() => {
        if (focusLost() && opener instanceof HTMLElement && opener.isConnected) opener.focus();
      });
    },
    [opener],
  );
  const results = filterCommandItems(items, query);
  const chosen = results.length > 0 ? Math.min(active, results.length - 1) : -1;
  const optionId = (index: number) => `${id}-option-${index}`;
  const listId = `${id}-list`;

  const follow = (item: CommandPaletteItem | undefined) => {
    if (!item) return;
    close();
    navigate(item.href);
  };

  const choose = (index: number) => {
    const next = (index + results.length) % results.length;
    setActive(next);
    document.getElementById(optionId(next))?.scrollIntoView({ block: 'nearest' });
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    if (event.key === 'ArrowDown' && results.length > 0) choose(chosen + 1);
    else if (event.key === 'ArrowUp' && results.length > 0) choose(chosen - 1);
    else if (event.key === 'Enter') follow(results[chosen]);
    else return;
    event.preventDefault();
  };

  return (
    <>
      <div className={styles.field}>
        <Icon name="search" size="md" />
        <input
          ref={focusOnMount}
          className={styles.input}
          type="text"
          role="combobox"
          aria-label={label}
          aria-expanded={results.length > 0}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={chosen >= 0 ? optionId(chosen) : undefined}
          autoComplete="off"
          spellCheck={false}
          placeholder={placeholder}
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setActive(0);
          }}
          onKeyDown={onKeyDown}
        />
      </div>
      <ul id={listId} role="listbox" aria-label={resultsLabel} className={styles.list}>
        {results.map((item, index) => (
          // The field keeps the focus and handles the keys; a pointer chooses and follows here.
          // eslint-disable-next-line jsx-a11y/click-events-have-key-events
          <li
            key={item.id}
            id={optionId(index)}
            role="option"
            aria-selected={index === chosen}
            className={styles.option}
            onPointerMove={() => setActive(index)}
            onClick={() => follow(item)}
          >
            <Icon name={item.icon} size="sm" />
            <span className={styles.title}>{item.title}</span>
            {item.detail && <span className={styles.detail}>{item.detail}</span>}
            <span className={styles.end} aria-hidden="true">
              <Icon name="chevronRight" size="sm" />
            </span>
          </li>
        ))}
      </ul>
      {results.length === 0 && (
        <p className={styles.empty} role="status">
          {emptyLabel}
        </p>
      )}
      {hint && <p className={styles.hint}>{hint}</p>}
    </>
  );
}
