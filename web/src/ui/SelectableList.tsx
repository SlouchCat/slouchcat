import type { ReactNode } from 'react';
import { Checkbox, CheckboxGroup, Label } from 'react-aria-components';
import { AppLink } from './AppLink';
import checkStyles from './CheckboxGroup.module.css';
import { Icon } from './Icon';
import styles from './SelectableList.module.css';

export interface SelectableItem {
  id: string;
  title: string;
  /** The title links here (the administration view of the title). */
  href?: string;
  /** Lines under the title: path, kind, error. */
  meta?: ReactNode;
  /** Spoken name of the checkbox ("Select <title>"). */
  selectLabel: string;
}

interface SelectableListProps {
  /** Name of the list (and of the checkbox group). */
  label: string;
  items: readonly SelectableItem[];
  /** Selected ids; omit for a list without selection. */
  value?: string[];
  onChange?: (value: string[]) => void;
}

/**
 * List of titles or files with a checkbox per row (react-aria CheckboxGroup: Tab moves, Space
 * toggles) and a link on each title. Without `onChange` the rows have no checkbox.
 */
export function SelectableList({ label, items, value, onChange }: SelectableListProps) {
  const rows = (
    <ul className={styles.list}>
      {items.map((item) => (
        <li key={item.id} className={styles.row}>
          {onChange && (
            <Checkbox value={item.id} aria-label={item.selectLabel} className={styles.check}>
              <span className={checkStyles.box} aria-hidden="true">
                <Icon name="check" size="sm" />
              </span>
            </Checkbox>
          )}
          <div className={styles.main}>
            {item.href ? (
              <AppLink href={item.href} className={styles.title}>
                {item.title}
              </AppLink>
            ) : (
              <span className={styles.title}>{item.title}</span>
            )}
            {item.meta && <div className={styles.meta}>{item.meta}</div>}
          </div>
        </li>
      ))}
    </ul>
  );

  if (!onChange) {
    return (
      <section className={styles.group} aria-label={label}>
        {rows}
      </section>
    );
  }
  return (
    <CheckboxGroup className={styles.group} value={value ?? []} onChange={onChange}>
      <Label className={styles.label}>{label}</Label>
      {rows}
    </CheckboxGroup>
  );
}

interface SelectCheckProps {
  /** Spoken name ("Select <title>"); the box has no visible label of its own. */
  label: string;
  isSelected: boolean;
  onChange: (selected: boolean) => void;
}

/**
 * The checkbox of one row in a table of records (DataTable), drawn like the list's: Space
 * toggles, Tab moves on, and the table's ↑/↓ move to the box of the next row.
 */
export function SelectCheck({ label, isSelected, onChange }: SelectCheckProps) {
  return (
    <Checkbox
      aria-label={label}
      isSelected={isSelected}
      onChange={onChange}
      className={styles.check}
    >
      <span className={checkStyles.box} aria-hidden="true">
        <Icon name="check" size="sm" />
      </span>
    </Checkbox>
  );
}
