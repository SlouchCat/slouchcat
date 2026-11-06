import { ToggleButton } from 'react-aria-components';
import { Icon } from './Icon';
import styles from './LockToggle.module.css';

interface LockToggleProps {
  /** Spoken name ("Sperre Titel"); the pressed state says whether it is locked. */
  label: string;
  isLocked: boolean;
  onChange: () => void;
  /** The visible word for each state ("gesperrt", "frei"). */
  lockedText: string;
  freeText: string;
  /** Nothing to toggle (a free field without change). */
  isDisabled?: boolean;
}

/**
 * The lock of one field (react-aria toggle button, concept 15 7.4): pressed while the field is
 * locked, so a refresh keeps it. Space or Enter toggles; the holder decides what toggling means.
 */
export function LockToggle({
  label,
  isLocked,
  onChange,
  lockedText,
  freeText,
  isDisabled,
}: LockToggleProps) {
  return (
    <ToggleButton
      aria-label={label}
      isSelected={isLocked}
      onChange={onChange}
      isDisabled={isDisabled}
      className={styles.toggle}
    >
      <Icon name={isLocked ? 'lock' : 'unlock'} size="sm" />
      <span aria-hidden="true">{isLocked ? lockedText : freeText}</span>
    </ToggleButton>
  );
}
