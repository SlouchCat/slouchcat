import styles from './ChoiceChips.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

interface ChoiceOption {
  value: string;
  label: string;
}

interface ChipButtonProps {
  option: ChoiceOption;
  selected: boolean;
  onSelect: (value: string) => void;
  focusKey?: string;
}

function ChipButton({ option, selected, onSelect, focusKey }: ChipButtonProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      className={styles.chip}
      aria-pressed={selected}
      onFocus={onFocus}
      onClick={() => onSelect(option.value)}
    >
      {option.label}
    </button>
  );
}

interface ChoiceChipsProps {
  /** Spoken name of the group ("Staffeln", "Versionen"). */
  label: string;
  options: readonly ChoiceOption[];
  value: string;
  onChange: (value: string) => void;
  /** Focus key prefix; each chip gets `<prefix>-<value>`. */
  focusKey?: string;
}

/**
 * One-of-many choice for the browsing pages (seasons, versions) without react-aria (P-10): toggle
 * buttons (`aria-pressed`) in a row. Arrow keys move between them (spatial navigation), Enter picks.
 */
export function ChoiceChips({ label, options, value, onChange, focusKey }: ChoiceChipsProps) {
  return (
    <div role="group" aria-label={label}>
      <SpatialGroup focusKey={focusKey} className={styles.group}>
        {options.map((option) => (
          <ChipButton
            key={option.value}
            option={option}
            selected={option.value === value}
            onSelect={onChange}
            focusKey={focusKey ? `${focusKey}-${option.value}` : undefined}
          />
        ))}
      </SpatialGroup>
    </div>
  );
}
