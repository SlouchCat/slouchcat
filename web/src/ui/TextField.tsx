import type { ReactNode } from 'react';
import {
  FieldError,
  Input,
  Label,
  Text,
  TextField as AriaTextField,
  type TextFieldProps as AriaTextFieldProps,
} from 'react-aria-components';
import styles from './Field.module.css';

interface TextFieldProps extends Omit<AriaTextFieldProps, 'className' | 'children' | 'style'> {
  label: string;
  description?: ReactNode;
  placeholder?: string;
  /** Monospace input for paths and tokens. */
  mono?: boolean;
}

/** Labelled text input (react-aria) with description and inline validation message. */
export function TextField({ label, description, placeholder, mono, ...rest }: TextFieldProps) {
  return (
    <AriaTextField className={styles.field} {...rest}>
      <Label className={styles.label}>{label}</Label>
      <Input className={`${styles.input} ${mono ? styles.mono : ''}`} placeholder={placeholder} />
      {description && (
        <Text slot="description" className={styles.description}>
          {description}
        </Text>
      )}
      <FieldError className={styles.error} />
    </AriaTextField>
  );
}
