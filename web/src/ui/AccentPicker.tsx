import {
  FieldError,
  Label,
  Radio,
  RadioGroup as AriaRadioGroup,
  type RadioGroupProps as AriaRadioGroupProps,
} from 'react-aria-components';
import { swatchStyle } from './accent';
import styles from './AccentPicker.module.css';
import fieldStyles from './Field.module.css';
import { Icon } from './Icon';
import { VisuallyHidden } from './VisuallyHidden';

interface AccentOption {
  /** Accent token name (`teal`). */
  value: string;
  /** Spoken name of the colour. */
  label: string;
}

interface AccentPickerProps extends Omit<AriaRadioGroupProps, 'className' | 'children' | 'style'> {
  label: string;
  options: readonly AccentOption[];
}

/**
 * Choice of a profile's accent colour: one swatch per accent token (colours come from the token
 * variables, never from literals). A radio group, so the arrow keys move between swatches.
 */
export function AccentPicker({ label, options, ...rest }: AccentPickerProps) {
  return (
    <AriaRadioGroup className={fieldStyles.field} orientation="horizontal" {...rest}>
      <Label className={fieldStyles.label}>{label}</Label>
      <div className={styles.swatches}>
        {options.map((option) => (
          <Radio
            key={option.value}
            value={option.value}
            className={styles.option}
            style={swatchStyle(option.value)}
          >
            <span className={styles.swatch} aria-hidden="true">
              <Icon name="check" size="md" />
            </span>
            <VisuallyHidden>{option.label}</VisuallyHidden>
          </Radio>
        ))}
      </div>
      <FieldError className={fieldStyles.error} />
    </AriaRadioGroup>
  );
}
