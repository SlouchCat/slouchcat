import { useId } from 'react';
import styles from './SelectField.module.css';
import { useSpatialItem } from './spatial';

export interface SelectOption<T extends string> {
  value: T;
  label: string;
}

/** A labelled group of options (`<optgroup>`): the facets of a tag rule. */
export interface SelectGroup<T extends string> {
  group: string;
  options: readonly SelectOption<T>[];
}

interface SelectFieldProps<T extends string> {
  /** Shown before the select ("Sortieren") and its accessible name. */
  label: string;
  /** The label for screen readers only: a select in a table row, named by its column. */
  hideLabel?: boolean;
  value: T;
  options: readonly (SelectOption<T> | SelectGroup<T>)[];
  onChange: (value: T) => void;
  focusKey?: string;
}

const optionOf = <T extends string>(option: SelectOption<T>) => (
  <option key={option.value} value={option.value}>
    {option.label}
  </option>
);

/**
 * A native select for the browsing pages without react-aria (P-10): the sort order of a library.
 * ↑ and ↓ stay with the select (they pick an option); ← and → leave it by spatial navigation.
 */
export function SelectField<T extends string>({
  label,
  hideLabel = false,
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
      {!hideLabel && (
        <label htmlFor={id} className={styles.label}>
          {label}
        </label>
      )}
      <select
        ref={ref}
        id={id}
        aria-label={hideLabel ? label : undefined}
        className={styles.select}
        value={value}
        onFocus={onFocus}
        onKeyDown={(event) => {
          // A closed select would change its value on ← and → as well; here they move the focus.
          if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') event.preventDefault();
        }}
        onChange={(event) => onChange(event.target.value as T)}
      >
        {options.map((entry) =>
          'group' in entry ? (
            <optgroup key={entry.group} label={entry.group}>
              {entry.options.map(optionOf)}
            </optgroup>
          ) : (
            optionOf(entry)
          ),
        )}
      </select>
    </span>
  );
}
