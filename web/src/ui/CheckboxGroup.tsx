import { useId, type ReactNode } from 'react';
import {
  Checkbox,
  CheckboxGroup as AriaCheckboxGroup,
  FieldError,
  Label,
  Text,
  type CheckboxGroupProps as AriaCheckboxGroupProps,
} from 'react-aria-components';
import styles from './CheckboxGroup.module.css';
import fieldStyles from './Field.module.css';
import { Icon } from './Icon';

interface Option {
  value: string;
  label: string;
  /** A short line under the label; it describes the checkbox and is not part of its name. */
  description?: string;
}

interface CheckboxGroupProps extends Omit<
  AriaCheckboxGroupProps,
  'className' | 'children' | 'style'
> {
  label: string;
  description?: ReactNode;
  options: readonly Option[];
}

/** Several independent choices as a list of checkboxes (react-aria; Space toggles, Tab moves). */
export function CheckboxGroup({ label, description, options, ...rest }: CheckboxGroupProps) {
  const id = useId();
  return (
    <AriaCheckboxGroup className={fieldStyles.field} {...rest}>
      <Label className={fieldStyles.label}>{label}</Label>
      {description && (
        <Text slot="description" className={fieldStyles.description}>
          {description}
        </Text>
      )}
      <div className={styles.list}>
        {options.map((option) =>
          option.description ? (
            <Checkbox
              key={option.value}
              value={option.value}
              className={styles.option}
              aria-labelledby={`${id}-${option.value}-label`}
              aria-describedby={`${id}-${option.value}-description`}
            >
              <span className={styles.box} aria-hidden="true">
                <Icon name="check" size="sm" />
              </span>
              <span className={styles.text}>
                <span id={`${id}-${option.value}-label`}>{option.label}</span>
                <span id={`${id}-${option.value}-description`} className={styles.description}>
                  {option.description}
                </span>
              </span>
            </Checkbox>
          ) : (
            <Checkbox key={option.value} value={option.value} className={styles.option}>
              <span className={styles.box} aria-hidden="true">
                <Icon name="check" size="sm" />
              </span>
              {option.label}
            </Checkbox>
          ),
        )}
      </div>
      <FieldError className={fieldStyles.error} />
    </AriaCheckboxGroup>
  );
}
