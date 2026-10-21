import type { ReactNode, Ref } from 'react';
import {
  FieldError,
  Input,
  Label,
  Text,
  TextArea,
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
  /** The `<input>` element, for code that has to select its text. */
  inputRef?: Ref<HTMLInputElement>;
  /** Select the whole text when the field gets focus (a link to copy). */
  selectOnFocus?: boolean;
  /** Several lines (a `<textarea>` of this many rows) for longer text such as a message. */
  rows?: number;
}

/**
 * Labelled text input (react-aria) with description and inline validation message; with `rows` a
 * multi-line text area.
 */
export function TextField({
  label,
  description,
  placeholder,
  mono,
  inputRef,
  selectOnFocus,
  rows,
  ...rest
}: TextFieldProps) {
  return (
    <AriaTextField className={styles.field} {...rest}>
      <Label className={styles.label}>{label}</Label>
      {rows ? (
        <TextArea
          rows={rows}
          className={`${styles.input} ${styles.multiline} ${mono ? styles.mono : ''}`}
          placeholder={placeholder}
        />
      ) : (
        <Input
          ref={inputRef}
          className={`${styles.input} ${mono ? styles.mono : ''}`}
          placeholder={placeholder}
          onFocus={selectOnFocus ? (event) => event.currentTarget.select() : undefined}
        />
      )}
      {description && (
        <Text slot="description" className={styles.description}>
          {description}
        </Text>
      )}
      <FieldError className={styles.error} />
    </AriaTextField>
  );
}
