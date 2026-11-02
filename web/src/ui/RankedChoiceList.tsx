import { useId, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import styles from './RankedChoiceList.module.css';

export interface RankedChoice {
  value: string;
  label: string;
}

interface RankedChoiceListProps {
  /** Everything a place can hold, in the order of the selects («Ohne Text», then languages). */
  choices: readonly RankedChoice[];
  /** The value of each place, place 1 first. */
  value: readonly string[];
  onChange: (value: string[]) => void;
  /** The visible label of a place, 0-based index in («1. Sprache»). */
  rankLabel: (index: number) => string;
  /** The button that appends a place with the first free choice («Weitere Sprache»). */
  addLabel: string;
  /** Whether the place at `index` may go; the last place never does. */
  canRemove?: (index: number) => boolean;
  /** Shows the places read-only, without «×» and the add button. */
  isDisabled?: boolean;
}

/** The list with `next` on the place `index`; a choice that held another place swaps with it. */
export function chooseAt(list: readonly string[], index: number, next: string): string[] {
  const out = [...list];
  const other = list.indexOf(next);
  if (other >= 0 && other !== index) out[other] = list[index]!;
  out[index] = next;
  return out;
}

/**
 * An ordered list as fixed places (concept 12, 10.2 and S-16): each place has a select of every
 * choice. Picking a choice that sits on another place swaps the two (the hint below says so and
 * a status line names the pair); «Weitere Sprache» appends a place with the first free choice,
 * «×» removes one. The places keep their elements, so a select keeps the focus through a swap,
 * also when ↑ or ↓ on the closed select changes it; after adding, the new place gets the focus,
 * after removing, the place that moved up into its spot (or the one before it).
 */
export function RankedChoiceList({
  choices,
  value,
  onChange,
  rankLabel,
  addLabel,
  canRemove,
  isDisabled,
}: RankedChoiceListProps) {
  const { t } = useTranslation();
  const id = useId();
  const hintId = `${id}-hint`;
  const [swapped, setSwapped] = useState('');
  const selects = useRef<(HTMLSelectElement | null)[]>([]);
  const addButton = useRef<HTMLButtonElement>(null);
  // The place to focus once the list has its new length (after adding or removing).
  const focusNext = useRef<number | null>(null);
  useLayoutEffect(() => {
    if (focusNext.current === null) return;
    const index = Math.min(focusNext.current, value.length - 1);
    focusNext.current = null;
    (index >= 0 ? selects.current[index] : addButton.current)?.focus();
  }, [value.length]);

  const labelOf = (choice: string) =>
    choices.find((entry) => entry.value === choice)?.label ?? choice;
  const free = choices.find((entry) => !value.includes(entry.value));

  return (
    <div className={isDisabled ? `${styles.list} ${styles.disabled}` : styles.list}>
      <ol className={styles.places}>
        {value.map((choice, index) => {
          const selectId = `${id}-${index}`;
          const removable = value.length > 1 && (canRemove?.(index) ?? true);
          return (
            // A place is its position: the elements stay when the values swap.
            <li key={index} className={styles.place}>
              <label htmlFor={selectId} className={styles.rank}>
                {rankLabel(index)}
              </label>
              <select
                ref={(element) => {
                  selects.current[index] = element;
                }}
                id={selectId}
                className={styles.select}
                value={choice}
                disabled={isDisabled}
                aria-describedby={hintId}
                onChange={(event) => {
                  const next = event.target.value;
                  const other = value.indexOf(next);
                  onChange(chooseAt(value, index, next));
                  setSwapped(
                    other >= 0 && other !== index
                      ? t('ui.rankedChoice.swapped', { a: labelOf(next), b: labelOf(choice) })
                      : '',
                  );
                }}
              >
                {choices.map((entry) => (
                  <option key={entry.value} value={entry.value}>
                    {entry.label}
                  </option>
                ))}
              </select>
              {!isDisabled && (
                <AriaButton
                  variant="ghost"
                  icon="close"
                  isDisabled={!removable}
                  aria-label={t('ui.rankedChoice.remove', { rank: rankLabel(index) })}
                  onPress={() => {
                    focusNext.current = index;
                    setSwapped('');
                    onChange(value.filter((_, at) => at !== index));
                  }}
                />
              )}
            </li>
          );
        })}
      </ol>
      {!isDisabled && (
        <div>
          <AriaButton
            ref={addButton}
            variant="ghost"
            icon="plus"
            isDisabled={!free}
            onPress={() => {
              if (!free) return;
              focusNext.current = value.length;
              setSwapped('');
              onChange([...value, free.value]);
            }}
          >
            {addLabel}
          </AriaButton>
        </div>
      )}
      <span id={hintId} className={styles.hint}>
        {t('ui.rankedChoice.hint')}
      </span>
      <span role="status" className={styles.status}>
        {swapped}
      </span>
    </div>
  );
}
