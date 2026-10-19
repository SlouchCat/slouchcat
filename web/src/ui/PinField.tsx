import type { ReactNode } from 'react';
import { TextField } from './TextField';

interface PinFieldProps {
  label: string;
  /** Form field name; the value is the PIN as typed. */
  name: string;
  value: string;
  onChange: (value: string) => void;
  description?: ReactNode;
  /** Message shown when the PIN is not 4 to 8 digits (client-side check on submit). */
  invalidMessage: string;
  /** Allows an empty field (a PIN that is optional). */
  isOptional?: boolean;
  autoFocus?: boolean;
  isDisabled?: boolean;
}

const PIN = /^\d{4,8}$/;

/**
 * Masked field for a PIN of 4 to 8 digits: numeric keypad on phones, anything but digits is
 * dropped while typing, and the browser does not offer to store it as a password (`one-time-code`).
 */
export function PinField({
  label,
  name,
  value,
  onChange,
  description,
  invalidMessage,
  isOptional = false,
  autoFocus,
  isDisabled,
}: PinFieldProps) {
  return (
    <TextField
      name={name}
      label={label}
      description={description}
      type="password"
      inputMode="numeric"
      autoComplete="one-time-code"
      autoFocus={autoFocus}
      isDisabled={isDisabled}
      isRequired={!isOptional}
      maxLength={8}
      value={value}
      onChange={(next) => onChange(next.replace(/\D/g, ''))}
      validate={(next) => (PIN.test(next) || (isOptional && next === '') ? null : invalidMessage)}
    />
  );
}
