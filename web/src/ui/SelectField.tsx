import { useId } from 'react';
import styles from './SelectField.module.css';
import { useSpatialItem } from './spatial';

interface SelectOption<T extends string> {
  value: T;
  label: string;
}

interface SelectFieldProps<T extends string> {
  /** Shown before the select ("Sortieren") and its accessible name. */
  label: string;
  value: T;
  options: readonly SelectOption<T>[];
  onChange: (value: T) => void;
  focusKey?: string;
}

/**
 * A native select for the browsing pages without react-aria (P-10): the sort order of a library.
 * ↑ and ↓ stay with the select (they pick an option); ← and → leave it by spatial navigation.
 */
export function SelectField<T extends string>({
  label,
  value,
  options,
  onChange,
  focusKey,
}: SelectFieldProps<T>) {
  const id = useId();
  const { ref, onFocus } = useSpatialItem<HTMLSelectElement>({
    focusKey,
    onArrowPress: (direction) => direction === 'left' || direction === 'right',
  });
  return (
    <span className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {label}
      </label>
      <select
        ref={ref}
        id={id}
        className={styles.select}
        value={value}
        onFocus={onFocus}
        onKeyDown={(event) => {
          // A closed select would change its value on ← and → as well; here they move the focus.
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') event.preventDefault();
        }}
        onChange={(event) => onChange(event.target.value as T)}
      >
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
    </span>
  );
}
