import {
  FieldError,
  Label,
  Radio,
  RadioGroup as AriaRadioGroup,
  type RadioGroupProps as AriaRadioGroupProps,
} from 'react-aria-components';
import styles from './Field.module.css';

interface Option {
  value: string;
  label: string;
}

interface RadioGroupProps extends Omit<AriaRadioGroupProps, 'className' | 'children' | 'style'> {
  label: string;
  options: readonly Option[];
}

/** Segmented choice between a few options (react-aria RadioGroup, arrow keys inside). */
export function RadioGroup({ label, options, ...rest }: RadioGroupProps) {
  return (
    <AriaRadioGroup className={styles.field} orientation="horizontal" {...rest}>
      <Label className={styles.label}>{label}</Label>
      <div className={styles.segments}>
        {options.map((option) => (
          <Radio key={option.value} value={option.value} className={styles.segment}>
            {option.label}
          </Radio>
        ))}
      </div>
      <FieldError className={styles.error} />
    </AriaRadioGroup>
  );
}
