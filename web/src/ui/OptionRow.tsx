import type { ReactNode } from 'react';
import {
  Header,
  ListBox,
  ListBoxItem,
  ListBoxSection,
  Text as AriaText,
} from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { Chip } from './Chip';
import { Icon } from './Icon';
import styles from './OptionRow.module.css';

/** Marks of an option (V-5): the file's default, a forced track, a subtitle that is burnt in. */
export type OptionMark = 'default' | 'forced' | 'burnIn';

const MARK_KEYS = {
  default: 'player.option.default',
  forced: 'player.option.forced',
  burnIn: 'player.option.burnIn',
} as const;

interface OptionRowProps {
  /** Key of the row, the one `onSelect` reports. */
  id: string;
  /** First line: «Deutsch», «Aus», «1080p». */
  label: string;
  /** Second line: «DTS 5.1», «SDH». */
  detail?: string;
  marks?: readonly OptionMark[];
  isDisabled?: boolean;
}

/**
 * One choice in a player panel: a tick when it is chosen, two lines of text and marks. A burnt-in
 * subtitle is marked in the warning colour, because choosing it starts a conversion. The marks
 * belong to the second line, so a screen reader reads them with it.
 */
export function OptionRow({ id, label, detail, marks = [], isDisabled }: OptionRowProps) {
  const { t } = useTranslation();
  return (
    <ListBoxItem id={id} textValue={label} className={styles.row} isDisabled={isDisabled}>
      {({ isSelected }) => (
        <>
          <span className={styles.check}>{isSelected && <Icon name="check" size="sm" />}</span>
          <span className={styles.text}>
            <AriaText slot="label" className={styles.label}>
              {label}
            </AriaText>
            {(detail || marks.length > 0) && (
              <AriaText slot="description" className={styles.detail}>
                {detail && <span>{detail}</span>}
                {marks.map((mark) => (
                  <Chip key={mark} tone={mark === 'burnIn' ? 'warning' : 'info'}>
                    {t(MARK_KEYS[mark])}
                  </Chip>
                ))}
              </AriaText>
            )}
          </span>
        </>
      )}
    </ListBoxItem>
  );
}

interface OptionMoreRowProps {
  id: string;
  /** «Weitere Sprachen (17)». */
  label: string;
  /** Whether the rows it stands for are shown. */
  isExpanded: boolean;
  onToggle: () => void;
}

/**
 * A row that shows or hides the rows after it («Weitere Sprachen (17)», 11.10) instead of being
 * chosen. Its id belongs in the list's `actionIds`: the list cannot choose it, Enter or a click
 * toggles it, and ↑/↓ walk over it like over any other row.
 */
export function OptionMoreRow({ id, label, isExpanded, onToggle }: OptionMoreRowProps) {
  return (
    <ListBoxItem id={id} textValue={label} className={styles.row} onAction={onToggle}>
      <span className={styles.check} />
      <span className={styles.text}>
        <AriaText slot="label" className={styles.label}>
          {label}
        </AriaText>
      </span>
      <span className={styles.chevron} data-expanded={isExpanded || undefined}>
        <Icon name="chevronDown" size="sm" />
      </span>
    </ListBoxItem>
  );
}

interface OptionSectionProps {
  /** Heading of the group: «Deine Sprachen», «Weitere Sprachen». */
  title: string;
  children: ReactNode;
}

/**
 * A group of rows with a heading inside the same list: ↑/↓ run on from the last row of one group
 * to the first of the next, and the list stays one Tab stop.
 */
export function OptionSection({ title, children }: OptionSectionProps) {
  return (
    <ListBoxSection className={styles.section}>
      <Header className={styles.header}>{title}</Header>
      {children}
    </ListBoxSection>
  );
}

interface OptionListProps {
  /** Spoken name of the list («Ton», «Untertitel»). */
  label: string;
  /** The chosen row, or null when none is. */
  selectedId: string | null;
  /** Enter, Space or a click on a row that is not chosen yet. */
  onSelect: (id: string) => void;
  /** Rows that act instead of being chosen (OptionMoreRow). */
  actionIds?: readonly string[];
  /** OptionRow, OptionMoreRow and OptionSection elements. */
  children: ReactNode;
}

// Two options React Aria's ListBox hands on at runtime (to useListState and
// useSelectableCollection) but leaves out of its types:
// - disallowTypeAhead: type to select would take the player's letter keys (M, T, U, C, K …),
//   which work in every zone but a text field (11.2), whenever a row starts with that letter. A
//   keyboard delegate without getKeyForSearch would do the same, but it needs the collection,
//   which only exists inside the ListBox.
// - disabledBehavior "selection": the rows in disabledKeys (the action rows) cannot be chosen but
//   stay focusable and act on Enter or a click.
// OptionRow.test.tsx checks both (a letter reaches the page; the more row toggles and is never
// chosen), so an update that drops the pass-through fails there.
const passedThrough: object = { disallowTypeAhead: true, disabledBehavior: 'selection' };

/**
 * The rows of a player panel as one Tab stop (react-aria ListBox): ↑ and ↓ move between the rows
 * without choosing (a new audio track restarts the stream, so it must not follow the focus), Enter,
 * Space or a click chooses. There is no SelectableList in web/src/ui to build on; a radio group
 * would choose on every arrow press, which is why this is a listbox with single selection.
 * Escape is left to the panel: a chosen row cannot be unchosen.
 */
export function OptionList({
  label,
  selectedId,
  onSelect,
  actionIds = [],
  children,
}: OptionListProps) {
  return (
    <ListBox
      {...passedThrough}
      aria-label={label}
      className={styles.list}
      selectionMode="single"
      disallowEmptySelection
      selectedKeys={selectedId === null ? [] : [selectedId]}
      disabledKeys={actionIds}
      onSelectionChange={(keys) => {
        if (keys === 'all') return;
        const [key] = keys;
        if (key !== undefined) onSelect(String(key));
      }}
    >
      {children}
    </ListBox>
  );
}
