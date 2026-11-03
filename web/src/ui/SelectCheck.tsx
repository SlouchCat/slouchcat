import { Checkbox } from 'react-aria-components';
import { Icon } from './Icon';
import styles from './SelectCheck.module.css';

interface SelectCheckProps {
  /** Spoken name ("Select <title>"); the box has no visible label of its own. */
  label: string;
  isSelected: boolean;
  onChange: (selected: boolean) => void;
}

/**
 * The checkbox of one row in a table of records (DataTable, the maintenance page): Space
 * toggles, Tab moves on, and the table's ↑/↓ move to the box of the next or previous row.
 */
export function SelectCheck({ label, isSelected, onChange }: SelectCheckProps) {
  return (
    <Checkbox
      aria-label={label}
      isSelected={isSelected}
      onChange={onChange}
      className={styles.check}
    >
      <span className={styles.box} aria-hidden="true">
        <Icon name="check" size="sm" />
      </span>
    </Checkbox>
  );
}
