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
  description?: ReactNode;
  options: readonly Option[];
}

/** Segmented choice between a few options (react-aria RadioGroup, arrow keys inside). */
export function RadioGroup({ label, description, options, ...rest }: RadioGroupProps) {
  return (
    <AriaRadioGroup className={styles.field} orientation="horizontal" {...rest}>
      <Label className={styles.label}>{label}</Label>
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
