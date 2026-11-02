import type { ReactNode } from 'react';
import {
  FieldError,
  Label,
  Radio,
  Text,
  RadioGroup as AriaRadioGroup,
  type RadioGroupProps as AriaRadioGroupProps,
} from 'react-aria-components';
import styles from './Field.module.css';

interface Option {
  value: string;
  label: string;
  /** Shown but not selectable (a rating above the account's ceiling). */
  isDisabled?: boolean;
}

interface RadioGroupProps extends Omit<AriaRadioGroupProps, 'className' | 'children' | 'style'> {
  label: string;
  /**
   * The label for screen readers only, where the visible text stands next to the group already
   * (a SettingRow); the group keeps its accessible name.
   */
  hideLabel?: boolean;
  description?: ReactNode;
  options: readonly Option[];
  /** The label before the segments on one line (a period in a page header). */
  inline?: boolean;
}

/** Segmented choice between a few options (react-aria RadioGroup, arrow keys inside). */
export function RadioGroup({
  label,
  hideLabel,
  description,
  options,
  inline,
  ...rest
}: RadioGroupProps) {
  return (
    <AriaRadioGroup
      className={`${styles.field} ${inline ? styles.inline : ''}`}
      orientation="horizontal"
      {...rest}
    >
      <Label className={hideLabel ? styles.hiddenLabel : styles.label}>{label}</Label>
      {description && (
        <Text slot="description" className={styles.description}>
          {description}
        </Text>
      )}
      <div className={styles.segments}>
        {options.map((option) => (
          <Radio
            key={option.value}
            value={option.value}
            isDisabled={option.isDisabled}
            className={styles.segment}
          >
            {option.label}
          </Radio>
        ))}
      </div>
      <FieldError className={styles.error} />
    </AriaRadioGroup>
  );
}
