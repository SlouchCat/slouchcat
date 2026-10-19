import type { TFunction } from 'i18next';
import { isApiError } from './client';

// Problem codes of the contract that the UI translates. Unknown codes fall back to `unknown`.
const problemKeys = {
  invalid_credentials: 'errors.code.invalid_credentials',
  library_path_invalid: 'errors.code.library_path_invalid',
  tmdb_key_invalid: 'errors.code.tmdb_key_invalid',
  tmdb_unreachable: 'errors.code.tmdb_unreachable',
  setup_done: 'errors.code.setup_done',
  not_found: 'errors.code.not_found',
  unauthenticated: 'errors.code.unauthenticated',
  forbidden: 'errors.code.forbidden',
  csrf_header_missing: 'errors.code.csrf_header_missing',
  validation_failed: 'errors.code.validation_failed',
  transcode_busy: 'errors.code.transcode_busy',
  pin_required: 'errors.code.pin_required',
  pin_invalid: 'errors.code.pin_invalid',
  pin_locked: 'errors.code.pin_locked',
  profile_required: 'errors.code.profile_required',
  primary_profile_required: 'errors.code.primary_profile_required',
  primary_profile: 'errors.code.primary_profile',
  invitation_gone: 'errors.code.invitation_gone',
  too_many_attempts: 'errors.code.too_many_attempts',
} as const;

// Field error codes (`Problem.errors[].code`); unknown codes read as "invalid value".
const fieldKeys = {
  required: 'errors.field.required',
  too_short: 'errors.field.too_short',
  too_long: 'errors.field.too_long',
  invalid_email: 'errors.field.invalid_email',
  above_account_limit: 'errors.field.above_account_limit',
  unknown_library: 'errors.field.unknown_library',
  too_many_profiles: 'errors.field.too_many_profiles',
  already_exists: 'errors.field.already_exists',
  invalid_value: 'errors.field.invalid',
  invalid_pin: 'errors.field.pin_format',
  not_absolute: 'errors.field.not_absolute',
  library_path_invalid: 'errors.code.library_path_invalid',
  tmdb_key_invalid: 'errors.code.tmdb_key_invalid',
  tmdb_unreachable: 'errors.code.tmdb_unreachable',
} as const;

const has = <K extends string>(map: Record<K, string>, key: string): key is K =>
  Object.prototype.hasOwnProperty.call(map, key);

/** Translated message for any error thrown by an API call or by fetch itself. */
export function problemMessage(t: TFunction, error: unknown): string {
  if (!isApiError(error)) return t('errors.code.network');
  const code = error.code;
  return has(problemKeys, code) ? t(problemKeys[code]) : t('errors.code.unknown');
}

/** Translated message for one field error code. */
export function fieldMessage(t: TFunction, code: string): string {
  return has(fieldKeys, code) ? t(fieldKeys[code]) : t('errors.field.invalid');
}

/**
 * Problem codes that belong to one input. Forms show them inline at that input instead of in the
 * form-level notice.
 */
const fieldOfCode: Record<string, string> = {
  library_path_invalid: 'library.path',
  tmdb_key_invalid: 'tmdbApiKey',
  tmdb_unreachable: 'tmdbApiKey',
};

export interface FormErrors {
  /** Messages per input name (`email`, `library.path`, …), as react-aria `validationErrors`. */
  fields: Record<string, string>;
  /** Message for the form as a whole (a summary for `validation_failed`), or null. */
  form: string | null;
}

/**
 * Splits an API error into inline field messages and a form-level message.
 * `fieldNames` maps the contract's field names to input names when they differ (for example
 * `path` → `library.path` on the admin form).
 */
export function formErrors(
  t: TFunction,
  error: unknown,
  fieldNames: Record<string, string> = {},
): FormErrors {
  const fields: Record<string, string> = {};
  if (!isApiError(error)) return { fields, form: problemMessage(t, error) };
  const rename = (field: string) => fieldNames[field] ?? field;

  const target = fieldOfCode[error.code];
  if (target) {
    fields[rename(target)] = t(problemKeys[error.code as keyof typeof problemKeys]);
    return { fields, form: null };
  }
  for (const fieldError of error.problem?.errors ?? []) {
    fields[rename(fieldError.field)] ??= fieldMessage(t, fieldError.code);
  }
  return { fields, form: problemMessage(t, error) };
}

/**
 * Whole seconds the server asked the client to wait: `retryAfterSeconds` of a `pin_locked`
 * problem, else the `Retry-After` header. Null when the server named none.
 */
export function retryAfterSeconds(error: unknown): number | null {
  if (!isApiError(error)) return null;
  const fromProblem = error.problem?.retryAfterSeconds;
  if (fromProblem !== undefined) return Math.max(0, Math.ceil(fromProblem));
  if (error.retryAfterMs !== null) return Math.max(0, Math.ceil(error.retryAfterMs / 1000));
  return null;
}
