import { useEffect, useId, useRef, type Ref } from 'react';
import styles from './SearchField.module.css';
import { useSpatialItem } from './spatial';

interface SearchFieldProps {
  /** Label of the field; hidden visually unless `showLabel` (the placeholder says what to type). */
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  focusKey?: string;
  /** Focus the field once it is on screen (the search page opens ready to type). */
  focusOnMount?: boolean;
  /** `year`: a short field for four digits (search filters), shown with its label. */
  variant?: 'search' | 'year';
  showLabel?: boolean;
  ref?: Ref<HTMLInputElement>;
}

/**
 * A plain text field for the browsing pages without react-aria (P-10): the search box and the
 * year filters. Arrow left and right stay in the text; up and down leave the field by spatial
 * navigation (to the header, to the filters below).
 */
export function SearchField({
  label,
  value,
  onChange,
  placeholder,
  focusKey,
  focusOnMount = false,
  variant = 'search',
  showLabel = variant === 'year',
  ref,
}: SearchFieldProps) {
  const id = useId();
  const own = useRef<HTMLInputElement | null>(null);
  const { ref: spatialRef, onFocus } = useSpatialItem<HTMLInputElement>({
    focusKey,
    onArrowPress: (direction) => direction === 'up' || direction === 'down',
  });

  useEffect(() => {
    if (focusOnMount) own.current?.focus();
  }, [focusOnMount]);

  const setRef = (node: HTMLInputElement | null) => {
    own.current = node;
    for (const target of [spatialRef, ref]) {
      if (typeof target === 'function') target(node);
      else if (target) target.current = node;
    }
  };

  return (
    <div className={`${styles.field} ${variant === 'year' ? styles.year : ''}`}>
      <label htmlFor={id} className={showLabel ? styles.label : styles.hidden}>
        {label}
      </label>
      <input
        ref={setRef}
        id={id}
        className={styles.input}
        type={variant === 'year' ? 'text' : 'search'}
        inputMode={variant === 'year' ? 'numeric' : undefined}
        maxLength={variant === 'year' ? 4 : 100}
        enterKeyHint="search"
        autoComplete="off"
        spellCheck={false}
        placeholder={placeholder}
        value={value}
        onFocus={onFocus}
        onChange={(event) => onChange(event.target.value)}
      />
    </div>
  );
}
