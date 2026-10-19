/** Trimmed text value of a form field, or '' when the field is missing or a file. */
export function formText(data: FormData, name: string, trim = true): string {
  const value = data.get(name);
  if (typeof value !== 'string') return '';
  return trim ? value.trim() : value;
}
