/**
 * Whether `value` may be the page to return to after the profile picker: a path of this app,
 * never another origin (`//host` and `/\host` are read as one by browsers) and never the picker
 * itself. Checked where the value is read, not only where the URL is parsed.
 */
export const isReturnTarget = (value: unknown): value is string =>
  typeof value === 'string' &&
  value.startsWith('/') &&
  !value.startsWith('//') &&
  !value.startsWith('/\\') &&
  !/^\/profiles([/?#]|$)/.test(value);
