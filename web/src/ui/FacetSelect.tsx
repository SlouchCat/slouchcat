import { useRef, useState } from 'react';
import { Dialog, Popover } from 'react-aria-components';
import { useDensityPortalProps } from './adminDensity';
import { AriaButton } from './AriaButton';
import styles from './FacetSelect.module.css';
import { FilterSheet } from './FilterSheet';
import { Icon } from './Icon';
import { useSpatialItem } from './spatial';

// One filter of the radio page with several values to pick and the number of stations each
// value gives (concept 16, 10.1 and variant A of the mockup): a button that names the filter and
// how many values are chosen, and below it the values as toggle buttons. On a wide screen they
// open in a popover under the button, on a phone in a sheet from below (FilterSheet). A pick
// applies at once; Escape or «Fertig» closes and the focus returns to the button.

export interface FacetOption {
  value: string;
  label: string;
  /** Stations the value would give under the other filters. */
  count?: number;
}

interface FacetOptionsProps {
  label: string;
  options: readonly FacetOption[];
  values: readonly string[];
  onChange: (values: string[]) => void;
  /** Shown instead of the list when there is no value to choose. */
  emptyLabel: string;
}

/** The values of a facet as toggle buttons with their counts. */
export function FacetOptions({ label, options, values, onChange, emptyLabel }: FacetOptionsProps) {
  if (options.length === 0) return <p className={styles.empty}>{emptyLabel}</p>;
  const toggle = (value: string) =>
    onChange(values.includes(value) ? values.filter((v) => v !== value) : [...values, value]);
  return (
    <div className={styles.options} role="group" aria-label={label}>
      {options.map((option) => {
        const selected = values.includes(option.value);
        return (
          <button
            key={option.value}
            type="button"
            className={styles.option}
            aria-pressed={selected}
            data-zero={option.count === 0 && !selected ? '' : undefined}
            onClick={() => toggle(option.value)}
          >
            <span className={styles.check} aria-hidden="true">
              {selected && <Icon name="check" size="sm" />}
            </span>
            <span className={styles.optionLabel}>{option.label}</span>
            {option.count !== undefined && <span className={styles.count}>{option.count}</span>}
          </button>
        );
      })}
    </div>
  );
}

interface FacetSelectProps extends FacetOptionsProps {
  clearLabel: string;
  doneLabel: string;
  /** Opens in a sheet from below (a phone) with these texts instead of a popover. */
  sheet?: { closeLabel: string; showLabel: string };
  focusKey?: string;
}

export function FacetSelect({
  label,
  options,
  values,
  onChange,
  emptyLabel,
  clearLabel,
  doneLabel,
  sheet,
  focusKey,
}: FacetSelectProps) {
  const [open, setOpen] = useState(false);
  const density = useDensityPortalProps();
  // The popover hangs from the button's box; the button itself belongs to norigin.
  const anchor = useRef<HTMLSpanElement>(null);
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  const list = (
    <FacetOptions
      label={label}
      options={options}
      values={values}
      onChange={onChange}
      emptyLabel={emptyLabel}
    />
  );
  const clear = (
    <AriaButton
      variant="ghost"
      size="sm"
      isDisabled={values.length === 0}
      onPress={() => onChange([])}
    >
      {clearLabel}
    </AriaButton>
  );

  return (
    <span ref={anchor} className={styles.anchor}>
      <button
        ref={ref}
        type="button"
        className={styles.trigger}
        aria-expanded={open}
        aria-haspopup="dialog"
        data-set={values.length > 0 ? '' : undefined}
        onFocus={onFocus}
        onClick={() => setOpen(true)}
      >
        {label}
        {values.length > 0 && <span className={styles.badge}>{values.length}</span>}
        <Icon name="chevronDown" size="sm" />
      </button>
      {sheet ? (
        <FilterSheet
          isOpen={open}
          onOpenChange={setOpen}
          title={label}
          closeLabel={sheet.closeLabel}
          footer={
            <>
              {clear}
              <AriaButton variant="primary" onPress={() => setOpen(false)}>
                {sheet.showLabel}
              </AriaButton>
            </>
          }
        >
          {list}
        </FilterSheet>
      ) : (
        <Popover
          className={styles.popover}
          triggerRef={anchor}
          isOpen={open}
          onOpenChange={setOpen}
          placement="bottom start"
          offset={6}
          {...density}
        >
          <Dialog className={styles.dialog} aria-label={label}>
            <div className={styles.scroll}>{list}</div>
            <div className={styles.footer}>
              {clear}
              <AriaButton variant="primary" size="sm" onPress={() => setOpen(false)}>
                {doneLabel}
              </AriaButton>
            </div>
          </Dialog>
        </Popover>
      )}
    </span>
  );
}
