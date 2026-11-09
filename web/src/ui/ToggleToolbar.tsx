import { ToggleButton, Toolbar } from 'react-aria-components';
import styles from './ToggleToolbar.module.css';
import { VisuallyHidden } from './VisuallyHidden';

export interface ToggleToolbarOption {
  value: string;
  label: string;
  /** The count shown beside the label ("3", or "–" while unknown). */
  count: string;
  /** Spoken instead of the count ("3 laufen"). */
  countLabel?: string;
}

interface ToggleToolbarProps {
  /** Spoken name of the toolbar ("Nach Art filtern"). */
  label: string;
  options: readonly ToggleToolbarOption[];
  /** The pressed option; exactly one is pressed. */
  value: string;
  onChange: (value: string) => void;
}

/**
 * One-of-many filter as a toolbar of toggle buttons with counts (concept 18, 17.3): `role="toolbar"`,
 * each button `aria-pressed`, ←/→ move between the buttons and Tab leaves the toolbar (react-aria's
 * Toolbar; it comes back to the last focused button). Pressing the pressed button keeps it.
 */
export function ToggleToolbar({ label, options, value, onChange }: ToggleToolbarProps) {
  return (
    <Toolbar aria-label={label} className={styles.toolbar}>
      {options.map((option) => (
        <ToggleButton
          key={option.value}
          className={styles.button}
          isSelected={option.value === value}
          onChange={() => onChange(option.value)}
        >
          {option.label}{' '}
          <span className={styles.count}>
            <span aria-hidden={option.countLabel ? true : undefined}>{option.count}</span>
            {option.countLabel && <VisuallyHidden>{option.countLabel}</VisuallyHidden>}
          </span>
        </ToggleButton>
      ))}
    </Toolbar>
  );
}
