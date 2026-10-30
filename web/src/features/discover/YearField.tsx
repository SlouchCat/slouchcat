import { useState } from 'react';
import { SearchField } from '@/ui';

const YEAR = /^\d{4}$/;

/** A year filter of the browsing pages (search, library): four digits, applied once complete. */
export function YearField({
  label,
  value,
  onChange,
  focusKey,
}: {
  label: string;
  value: number | undefined;
  onChange: (year: number | undefined) => void;
  focusKey: string;
}) {
  const [text, setText] = useState(value !== undefined ? String(value) : '');
  // The URL changed from elsewhere ("Filter zurücksetzen", a link): follow it, but keep a year that
  // is still being typed.
  const [seen, setSeen] = useState(value);
  if (value !== seen) {
    setSeen(value);
    setText(value !== undefined ? String(value) : YEAR.test(text) ? '' : text);
  }
  return (
    <SearchField
      label={label}
      variant="year"
      focusKey={focusKey}
      value={text}
      onChange={(next) => {
        const digits = next.replace(/\D/g, '').slice(0, 4);
        setText(digits);
        // Only a full year filters; a partial one lifts the filter until it is complete.
        onChange(YEAR.test(digits) ? Number(digits) : undefined);
      }}
    />
  );
}
