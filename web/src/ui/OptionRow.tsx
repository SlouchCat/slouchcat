import type { ReactNode } from 'react';
import { ListBox, ListBoxItem, Text as AriaText } from 'react-aria-components';
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

interface OptionListProps {
  /** Spoken name of the list («Ton», «Untertitel»). */
  label: string;
  /** The chosen row, or null when none is. */
  selectedId: string | null;
  /** Enter, Space or a click on a row that is not chosen yet. */
  onSelect: (id: string) => void;
  /** OptionRow elements. */
  children: ReactNode;
}

/**
 * The rows of a player panel as one Tab stop (react-aria ListBox): ↑ and ↓ move between the rows
 * without choosing (a new audio track restarts the stream, so it must not follow the focus), Enter,
 * Space or a click chooses. There is no SelectableList in web/src/ui to build on; a radio group
 * would choose on every arrow press, which is why this is a listbox with single selection.
 * Escape is left to the panel: a chosen row cannot be unchosen.
 */
export function OptionList({ label, selectedId, onSelect, children }: OptionListProps) {
  return (
    <ListBox
      aria-label={label}
      className={styles.list}
      selectionMode="single"
      disallowEmptySelection
      selectedKeys={selectedId === null ? [] : [selectedId]}
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
