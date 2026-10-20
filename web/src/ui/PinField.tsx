import type { ReactNode } from 'react';
import { TextField } from './TextField';

interface PinFieldProps {
  label: string;
  /** Form field name; the value is the PIN as typed. */
  name: string;
  value: string;
  onChange: (value: string) => void;
  description?: ReactNode;
  /** Message shown when the PIN has the wrong number of digits (client-side check on submit). */
  invalidMessage: string;
  /** Allows an empty field (a PIN that is optional). */
  isOptional?: boolean;
  autoFocus?: boolean;
  isDisabled?: boolean;
  /** Digits allowed: 4 to 8 for a profile PIN, 6 to 12 for the separate offline PIN (AN-19). */
  digits?: { min: number; max: number };
}

const PROFILE_PIN = { min: 4, max: 8 };

/**
 * Masked field for a PIN of 4 to 8 digits (or `digits`): numeric keypad on phones, anything but
 * digits is dropped while typing, and the browser does not offer to store it as a password
 * (`one-time-code`).
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
  digits = PROFILE_PIN,
}: PinFieldProps) {
  const pattern = new RegExp(`^\\d{${digits.min},${digits.max}}$`);
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
      maxLength={digits.max}
      value={value}
      onChange={(next) => onChange(next.replace(/\D/g, ''))}
      validate={(next) =>
        pattern.test(next) || (isOptional && next === '') ? null : invalidMessage
      }
    />
  );
}
