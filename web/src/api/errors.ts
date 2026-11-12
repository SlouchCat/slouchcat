import type { TFunction } from 'i18next';
import { isApiError } from './client';

// Problem codes of the contract that the UI translates. Unknown codes fall back to `unknown`.
const problemKeys = {
  invalid_credentials: 'errors.code.invalid_credentials',
  // Administration (M8): account, backup and announcements.
  email_taken: 'errors.code.email_taken',
  passphrase_required: 'errors.code.passphrase_required',
  passphrase_too_short: 'errors.code.passphrase_too_short',
  passphrase_wrong: 'errors.code.passphrase_wrong',
  backup_invalid: 'errors.code.backup_invalid',
  backup_newer_than_server: 'errors.code.backup_newer_than_server',
  // Concept 11: restart and restore.
  restart_pending: 'errors.code.restart_pending',
  backup_busy: 'errors.code.backup_busy',
  // Concept 15 P-8: an install of an update runs.
  update_running: 'errors.code.update_running',
  announcement_published: 'errors.code.announcement_published',
  library_path_invalid: 'errors.code.library_path_invalid',
  // The folder browser of the library form (idea 02).
  path_not_browsable: 'errors.code.path_not_browsable',
  path_unreachable: 'errors.code.path_unreachable',
  browse_busy: 'errors.code.browse_busy',
  // Removing a folder from a library (idea 02, E12).
  scan_running: 'errors.code.scan_running',
  confirm_mismatch: 'errors.code.confirm_mismatch',
  tmdb_key_invalid: 'errors.code.tmdb_key_invalid',
  tmdb_unreachable: 'errors.code.tmdb_unreachable',
  provider_not_configured: 'errors.code.provider_not_configured',
  setup_done: 'errors.code.setup_done',
  not_found: 'errors.code.not_found',
  unauthenticated: 'errors.code.unauthenticated',
  forbidden: 'errors.code.forbidden',
  csrf_header_missing: 'errors.code.csrf_header_missing',
  validation_failed: 'errors.code.validation_failed',
  // Notification channels (M8, A-6).
  channel_unavailable: 'errors.code.channel_unavailable',
  channel_disabled: 'errors.code.channel_disabled',
  event_not_allowed: 'errors.code.event_not_allowed',
  send_failed: 'errors.code.send_failed',
  push_unavailable: 'errors.code.push_unavailable',
  transcode_busy: 'errors.code.transcode_busy',
  pin_required: 'errors.code.pin_required',
  pin_invalid: 'errors.code.pin_invalid',
  pin_locked: 'errors.code.pin_locked',
  profile_required: 'errors.code.profile_required',
  primary_profile_required: 'errors.code.primary_profile_required',
  primary_profile: 'errors.code.primary_profile',
  // User management (ADR 0014): the owner level and the two conflicts of the role change.
  owner_required: 'errors.code.owner_required',
  owner_protected: 'errors.code.owner_protected',
  last_admin: 'errors.code.last_admin',
  admin_limits_fixed: 'errors.code.admin_limits_fixed',
  invitation_gone: 'errors.code.invitation_gone',
  too_many_attempts: 'errors.code.too_many_attempts',
  too_many_requests: 'errors.code.too_many_requests',
  watch_limit_reached: 'errors.code.watch_limit_reached',
  stream_limit_reached: 'errors.code.stream_limit_reached',
  device_required: 'errors.code.device_required',
  not_downloadable: 'errors.code.not_downloadable',
  download_not_ready: 'errors.code.download_not_ready',
  download_evicted: 'errors.code.download_evicted',
  feed_unreachable: 'errors.code.feed_unreachable',
  feed_invalid: 'errors.code.feed_invalid',
  feed_exists: 'errors.code.feed_exists',
  // Stored podcast episodes (concept 19 part A, contract 1.27.0): «Speichern» needs a folder that
  // passes the folder check, and a podcast folder at all.
  root_not_writable: 'errors.code.root_not_writable',
  root_required: 'errors.code.root_required',
  rating_below_library_default: 'errors.code.rating_below_library_default',
  not_listable: 'errors.code.not_listable',
  not_editable: 'errors.code.not_editable',
  // Administration (M8).
  env_wins: 'errors.code.env_wins',
  telegram_token_invalid: 'errors.code.telegram_token_invalid',
  telegram_unreachable: 'errors.code.telegram_unreachable',
  email_not_configured: 'errors.code.email_not_configured',
  email_send_failed: 'errors.code.email_send_failed',
  job_not_failed: 'errors.code.job_not_failed',
  update_check_disabled: 'errors.code.update_check_disabled',
  job_type_not_runnable: 'errors.code.job_type_not_runnable',
  job_type_paused: 'errors.code.job_type_paused',
  job_type_not_scheduled: 'errors.code.job_type_not_scheduled',
  job_type_not_windowable: 'errors.code.job_type_not_windowable',
  // The spotlight list (M11): another entry holds the place an entry is pinned to.
  rank_taken: 'errors.code.rank_taken',
  // Webradio (concept 16, contract 1.17.0): the relays are full, the station does not answer or
  // sends no audio, the catalog is not synced yet.
  radio_capacity: 'errors.code.radio_capacity',
  radio_unreachable: 'errors.code.radio_unreachable',
  radio_not_audio: 'errors.code.radio_not_audio',
  radio_catalog_loading: 'errors.code.radio_catalog_loading',
  // The radio offer (contract 1.28.0): it changed since the version the draft is based on.
  offer_conflict: 'errors.code.offer_conflict',
  // Deleting a library or a title (concept 20 stage 5, O-13): a profile plays one of its titles.
  playing: 'errors.code.playing',
  not_deletable: 'errors.code.not_deletable',
  files_not_deletable: 'errors.code.files_not_deletable',
  deleting: 'errors.code.deleting',
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
  // Metadata settings (concept 10, ADR 0018): the language lists, the chains, the country.
  langs_empty: 'admin.metadata.errors.langs_empty',
  lang_unknown: 'admin.metadata.errors.lang_unknown',
  ui_lang_required: 'admin.metadata.errors.ui_lang_required',
  chain_duplicate: 'admin.metadata.errors.chain_duplicate',
  country_unknown: 'admin.metadata.errors.country_unknown',
  not_burnable: 'errors.field.not_burnable',
  not_absolute: 'errors.field.not_absolute',
  library_path_invalid: 'errors.code.library_path_invalid',
  // The folders of a library (ADR 0015): `roots[n].path`, `.content`, `.providers` and `roots`.
  root_overlap: 'errors.field.root_overlap',
  path_with_content: 'errors.field.path_with_content',
  root_files_missing: 'errors.field.root_files_missing',
  content_not_allowed: 'errors.field.content_not_allowed',
  providers_not_allowed: 'errors.field.providers_not_allowed',
  roots_required: 'errors.field.roots_required',
  provider_not_searchable: 'errors.field.provider_not_searchable',
  // The book sources (concept 19 D6, 422 validation_failed): a source that failed (field
  // `provider`), an entry it does not know (`providerId`), a refetch of a book without a match
  // (`fromProvider`).
  provider_unreachable: 'errors.code.provider_unreachable',
  provider_paused: 'errors.code.provider_paused',
  provider_not_configured: 'errors.code.provider_not_configured',
  not_found: 'errors.field.not_found',
  not_matched: 'errors.field.not_matched',
  // The folder browser of the library form (idea 02).
  path_not_browsable: 'errors.code.path_not_browsable',
  path_unreachable: 'errors.code.path_unreachable',
  browse_busy: 'errors.code.browse_busy',
  tmdb_key_invalid: 'errors.code.tmdb_key_invalid',
  tmdb_unreachable: 'errors.code.tmdb_unreachable',
  invalid_url: 'errors.field.invalid_url',
  library_kind: 'errors.field.library_kind',
  unknown_item: 'errors.field.unknown_item',
  env_wins: 'errors.code.env_wins',
  // Own radio stations (concept 16, 5.6, F-15): an address in the local network, a field that
  // only an own station has.
  local_address: 'errors.field.local_address',
  not_own: 'errors.field.not_own',
} as const;

const has = <K extends string>(map: Record<K, string>, key: string): key is K =>
  Object.prototype.hasOwnProperty.call(map, key);

/** Translated message for any error thrown by an API call or by fetch itself. */
export function problemMessage(t: TFunction, error: unknown): string {
  if (!isApiError(error)) return t('errors.code.network');
  const code = error.code;
  // `playing` names the profiles that play a title of what was about to be deleted.
  const playing = error.problem?.playingProfiles ?? [];
  if (code === 'playing' && playing.length > 0) {
    return t('errors.code.playing_named', { names: playing.join(t('format.listSeparator')) });
  }
  return has(problemKeys, code) ? t(problemKeys[code]) : t('errors.code.unknown');
}

/** Translated message for a problem code the server stored as text, or undefined if unknown. */
export function codeMessage(t: TFunction, code: string): string | undefined {
  return has(problemKeys, code) ? t(problemKeys[code]) : undefined;
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
  feed_unreachable: 'feedUrl',
  feed_invalid: 'feedUrl',
  feed_exists: 'feedUrl',
  telegram_token_invalid: 'telegram.botToken',
  telegram_unreachable: 'telegram.botToken',
  email_taken: 'email',
  passphrase_required: 'passphrase',
  passphrase_too_short: 'passphrase',
  passphrase_wrong: 'passphrase',
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
 * The one message of a failed request shown without a form: the first field's message if the
 * server named a field (`provider_paused`, `not_matched`), else the form's.
 */
export function firstErrorMessage(t: TFunction, error: unknown): string {
  const errors = formErrors(t, error);
  return Object.values(errors.fields)[0] ?? errors.form ?? '';
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

/**
 * A whole-number member of the problem's `extensions` object (RFC 9457 extension members, for
 * example `limitMinutes` of `watch_limit_reached`), or undefined when the server sent none.
 */
export function problemExtension(error: unknown, name: string): number | undefined {
  if (!isApiError(error)) return undefined;
  const extensions = (error.problem as { extensions?: unknown } | null)?.extensions;
  if (typeof extensions !== 'object' || extensions === null) return undefined;
  const value = (extensions as Record<string, unknown>)[name];
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}
