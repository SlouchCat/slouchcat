import styles from './FilterChips.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

interface FilterOption {
  value: string;
  label: string;
}

function FilterChip({
  option,
  selected,
  onToggle,
  focusKey,
}: {
  option: FilterOption;
  selected: boolean;
  onToggle: (value: string) => void;
  focusKey?: string;
}) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      className={styles.chip}
      aria-pressed={selected}
      onFocus={onFocus}
      onClick={() => onToggle(option.value)}
    >
      {option.label}
    </button>
  );
}

interface FilterChipsProps {
  /** Spoken name of the group ("Medientyp"). */
  label: string;
  options: readonly FilterOption[];
  /** The selected values; none selected means no restriction. */
  values: readonly string[];
  onChange: (values: string[]) => void;
  /** Focus key prefix; each chip gets `<prefix>-<value>`. */
  focusKey?: string;
}

/**
 * Many-of-many filter for the browsing pages (search) without react-aria (P-10): toggle buttons
 * (`aria-pressed`) in a row, like ChoiceChips. Arrow keys move between them, Enter toggles.
 */
export function FilterChips({ label, options, values, onChange, focusKey }: FilterChipsProps) {
  const toggle = (value: string) =>
    onChange(
      values.includes(value)
        ? values.filter((entry) => entry !== value)
        : options.map((option) => option.value).filter((v) => v === value || values.includes(v)),
    );
  return (
    <div role="group" aria-label={label}>
      <SpatialGroup focusKey={focusKey} className={styles.group}>
        {options.map((option) => (
          <FilterChip
            key={option.value}
            option={option}
            selected={values.includes(option.value)}
            onToggle={toggle}
            focusKey={focusKey ? `${focusKey}-${option.value}` : undefined}
          />
        ))}
      </SpatialGroup>
    </div>
  );
}
