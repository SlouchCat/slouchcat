import type { ReactNode } from 'react';
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
  return (
    <AriaCheckboxGroup className={fieldStyles.field} {...rest}>
      <Label className={fieldStyles.label}>{label}</Label>
      {description && (
        <Text slot="description" className={fieldStyles.description}>
          {description}
        </Text>
      )}
      <div className={styles.list}>
        {options.map((option) => (
          <Checkbox key={option.value} value={option.value} className={styles.option}>
            <span className={styles.box} aria-hidden="true">
              <Icon name="check" size="sm" />
            </span>
            {option.label}
          </Checkbox>
        ))}
      </div>
      <FieldError className={fieldStyles.error} />
    </AriaCheckboxGroup>
  );
}
