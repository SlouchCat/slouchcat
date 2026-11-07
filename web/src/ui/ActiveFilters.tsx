import { Icon } from './Icon';
import styles from './ActiveFilters.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

// The active filters of a page as removable chips, and «Alle Filter zurücksetzen» behind them
// (concept 16, 10.1). Arrow keys move between the chips (norigin), Enter removes one.

export interface ActiveFilter {
  key: string;
  label: string;
  /** Accessible name of the chip's button («Filter Pop entfernen»). */
  removeLabel: string;
  onRemove: () => void;
}

function Chip({ filter }: { filter: ActiveFilter }) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>();
  return (
    <button
      ref={ref}
      type="button"
      className={styles.chip}
      aria-label={filter.removeLabel}
      onFocus={onFocus}
      onClick={filter.onRemove}
    >
      <span>{filter.label}</span>
      <Icon name="close" size="sm" />
    </button>
  );
}

function Reset({ label, onPress }: { label: string; onPress: () => void }) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>();
  return (
    <button ref={ref} type="button" className={styles.reset} onFocus={onFocus} onClick={onPress}>
      {label}
    </button>
  );
}

interface ActiveFiltersProps {
  /** Spoken name of the group («Aktive Filter»). */
  label: string;
  filters: readonly ActiveFilter[];
  resetLabel: string;
  onReset: () => void;
  focusKey?: string;
}

export function ActiveFilters({
  label,
  filters,
  resetLabel,
  onReset,
  focusKey,
}: ActiveFiltersProps) {
  if (filters.length === 0) return null;
  return (
    <div role="group" aria-label={label}>
      <SpatialGroup focusKey={focusKey} className={styles.row} ordered>
        {filters.map((filter) => (
          <Chip key={filter.key} filter={filter} />
        ))}
        <Reset label={resetLabel} onPress={onReset} />
      </SpatialGroup>
    </div>
  );
}
