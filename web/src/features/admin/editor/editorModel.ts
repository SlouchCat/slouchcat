import type { AdminItem, components, ImageCandidate } from '@slouchcat/api-client';
import { isApiError } from '@/api/client';
import { FROM_METADATA, languageChoiceOf, ratingChoiceOf } from '../item/MetadataTab';

// The logic of the metadata editor (concept 15 7.4) without React: what the reader changed, which
// field shows its lock, what «Speichern» sends, which candidates a filter keeps and what an upload
// error means. The server decides and locks; this only prepares the requests.

export type AdminItemMetadata = components['schemas']['AdminItemMetadata'];
export type AdminMetadataField = components['schemas']['AdminMetadataField'];
type ItemMetadataUpdate = components['schemas']['ItemMetadataUpdate'];

/** A text or column of updateItemMetadata, or one of the two choices with their own endpoint. */
export type EditorField = AdminMetadataField | 'rating' | 'language';

/** Every field as the form holds it: text, the year as typed, genres as a list. */
export interface EditorValues {
  title: string;
  originalTitle: string;
  sortTitle: string;
  year: string;
  releaseDate: string;
  tagline: string;
  overview: string;
  genres: readonly string[];
  /** The fields of an audiobook; the volume as typed («2», «2.5», «2,5»). */
  author: string;
  narrator: string;
  seriesName: string;
  seriesIndex: string;
  publisher: string;
  isbn: string;
  asin: string;
  /** ratingChoiceOf: the age or FROM_METADATA. */
  rating: string;
  /** languageChoiceOf: the code, "" for «Unbekannt», or FROM_METADATA. */
  language: string;
}

/** What the reader did: changed values and the locks he lifts without a change. */
export interface EditorDraft {
  changed: Partial<EditorValues>;
  unlock: readonly AdminMetadataField[];
}

export const emptyDraft: EditorDraft = { changed: {}, unlock: [] };

/** The two choices that unlock by going back to «Aus Metadaten». */
const isChoice = (field: EditorField): field is 'rating' | 'language' =>
  field === 'rating' || field === 'language';

/** The values the server holds for the title. */
export function serverValues(item: AdminItem, meta: AdminItemMetadata): EditorValues {
  return {
    title: meta.title,
    originalTitle: meta.originalTitle ?? '',
    // A free sort title follows the title: the field is empty (the title shows as its
    // placeholder), so anything typed, the title's own text too, sets and locks it.
    sortTitle: meta.locked.includes('sortTitle') ? (meta.sortTitle ?? '') : '',
    year: meta.year !== undefined ? String(meta.year) : '',
    releaseDate: meta.releaseDate ?? '',
    tagline: meta.tagline ?? '',
    overview: meta.overview ?? '',
    genres: meta.genres,
    author: meta.author ?? '',
    narrator: meta.narrator ?? '',
    seriesName: meta.seriesName ?? '',
    seriesIndex: meta.seriesIndex !== undefined ? String(meta.seriesIndex) : '',
    publisher: meta.publisher ?? '',
    isbn: meta.isbn ?? '',
    asin: meta.asin ?? '',
    rating: ratingChoiceOf(item),
    language: languageChoiceOf(item),
  };
}

const same = (a: EditorValues[EditorField], b: EditorValues[EditorField]) =>
  Array.isArray(a) && Array.isArray(b)
    ? a.length === b.length && a.every((entry, i) => entry === b[i])
    : a === b;

/** The value the form shows: the change, else the server's. */
export const valueOf = <F extends EditorField>(
  server: EditorValues,
  draft: EditorDraft,
  field: F,
): EditorValues[F] => (draft.changed[field] as EditorValues[F] | undefined) ?? server[field];

export const isChanged = (draft: EditorDraft, field: EditorField) => field in draft.changed;

/** A change of one field: back to the server's value drops it; a change locks, so no unlock. */
export function editField<F extends EditorField>(
  server: EditorValues,
  draft: EditorDraft,
  field: F,
  value: EditorValues[F],
): EditorDraft {
  const changed = { ...draft.changed };
  if (same(value, server[field])) delete changed[field];
  else changed[field] = value;
  return { changed, unlock: draft.unlock.filter((name) => name !== field) };
}

/** Whether the field is locked on the server. */
export function lockedOnServer(item: AdminItem, meta: AdminItemMetadata, field: EditorField) {
  if (field === 'rating') return item.ownRating !== undefined;
  if (field === 'language') return languageChoiceOf(item) !== FROM_METADATA;
  return meta.locked.includes(field);
}

/** Whether the lock of the field shows closed: locked and not lifted, or changed (a change locks). */
export function shownLocked(
  item: AdminItem,
  meta: AdminItemMetadata,
  server: EditorValues,
  draft: EditorDraft,
  field: EditorField,
): boolean {
  if (isChoice(field)) return valueOf(server, draft, field) !== FROM_METADATA;
  if (isChanged(draft, field)) return true;
  return lockedOnServer(item, meta, field) && !draft.unlock.includes(field);
}

/**
 * The lock button of a field. Closed: a change goes back to the server's value (it would lock),
 * and a lock of the server is lifted; a choice goes back to «Aus Metadaten». Open after the
 * reader lifted a lock: the lock stays as the server has it. A free field has nothing to toggle.
 */
export function toggleLock(
  item: AdminItem,
  meta: AdminItemMetadata,
  server: EditorValues,
  draft: EditorDraft,
  field: EditorField,
): EditorDraft {
  if (isChoice(field)) {
    const target = shownLocked(item, meta, server, draft, field) ? FROM_METADATA : server[field];
    return editField(server, draft, field, target);
  }
  if (shownLocked(item, meta, server, draft, field)) {
    const changed = { ...draft.changed };
    delete changed[field];
    const unlock = lockedOnServer(item, meta, field) ? [...draft.unlock, field] : draft.unlock;
    return { changed, unlock: [...new Set(unlock)] };
  }
  return { ...draft, unlock: draft.unlock.filter((name) => name !== field) };
}

/** Whether the lock button can do anything: a free field without change has no lock to lift. */
export const canToggleLock = (
  item: AdminItem,
  meta: AdminItemMetadata,
  server: EditorValues,
  draft: EditorDraft,
  field: EditorField,
) =>
  shownLocked(item, meta, server, draft, field) ||
  (isChoice(field) ? isChanged(draft, field) : draft.unlock.includes(field));

const YEAR = /^\d{4}$/;
/** A volume: up to four digits, a fraction with a point or a comma (0.5 before 1). */
const VOLUME = /^\d{1,4}([.,]\d+)?$/;

/** What «Speichern» sends: the body of updateItemMetadata and the changed choices. */
export interface EditorSave {
  /** undefined: no text or column changed and no lock lifted. */
  metadata?: ItemMetadataUpdate;
  rating?: string;
  language?: string;
  /** Field errors the form shows before anything is sent (`invalid_value`, `required`). */
  invalid: Partial<Record<AdminMetadataField, string>>;
}

/**
 * Turns the draft into the requests of «Speichern» (all changed fields at once). An empty series
 * clears the volume with it on the server, so an emptied volume goes along unsent; a series that
 * stays needs its volume.
 */
export function saveOf(draft: EditorDraft, server: EditorValues): EditorSave {
  const { rating, language, year, genres, seriesName, seriesIndex, ...texts } = draft.changed;
  const body: ItemMetadataUpdate = {};
  const invalid: EditorSave['invalid'] = {};
  for (const [field, value] of Object.entries(texts) as [keyof typeof texts, string][]) {
    body[field] = value.trim();
  }
  if (year !== undefined) {
    const typed = year.trim();
    const number = Number(typed);
    if (!typed) invalid.year = 'required';
    else if (!YEAR.test(typed) || number < 1800 || number > 2200) invalid.year = 'invalid_value';
    else body.year = number;
  }
  if (genres !== undefined) body.genres = [...genres];
  if (seriesName !== undefined) body.seriesName = seriesName.trim();
  if (seriesIndex !== undefined) {
    const typed = seriesIndex.trim();
    const series = (seriesName ?? server.seriesName).trim();
    const number = Number(typed.replace(',', '.'));
    if (!typed) {
      if (series) invalid.seriesIndex = 'required';
    } else if (!series || !VOLUME.test(typed) || number > 9999)
      invalid.seriesIndex = 'invalid_value';
    else body.seriesIndex = number;
  }
  if (draft.unlock.length > 0) body.unlock = [...draft.unlock];
  return {
    metadata: Object.keys(body).length > 0 ? body : undefined,
    rating,
    language,
    invalid,
  };
}

/** Adds a genre (trimmed, once, case as typed); an empty one changes nothing. */
export function withGenre(genres: readonly string[], name: string): readonly string[] {
  const value = name.trim();
  if (!value || genres.some((genre) => genre.toLowerCase() === value.toLowerCase())) return genres;
  return [...genres, value];
}

/** The filter of the candidates: a provider or every one, a language ("" = no text) or every one. */
export interface CandidateFilter {
  provider: string | null;
  lang: string | null;
}

export const filterCandidates = (list: readonly ImageCandidate[], filter: CandidateFilter) =>
  list.filter(
    (c) =>
      (filter.provider === null || c.provider === filter.provider) &&
      (filter.lang === null || c.lang === filter.lang),
  );

/** The providers and languages the candidates carry, in their first order, for the filters. */
export function candidateFacets(list: readonly ImageCandidate[]) {
  return {
    providers: [...new Set(list.map((c) => c.provider))],
    langs: [...new Set(list.map((c) => c.lang))],
  };
}

/** The i18n key of a failed upload (uploadItemImage 422 per field and code), else undefined. */
type UploadErrorKey =
  | 'admin.editor.images.upload.error.tooLarge'
  | 'admin.editor.images.upload.error.format'
  | 'admin.editor.images.upload.error.pixels'
  | 'admin.editor.images.upload.error.missing'
  | 'admin.editor.images.upload.error.kind';

export function uploadErrorKey(error: unknown): UploadErrorKey | undefined {
  if (!isApiError(error, 422)) return undefined;
  const errors = error.problem?.errors ?? [];
  const file = errors.find((entry) => entry.field === 'file');
  if (file) {
    switch (file.code) {
      case 'too_large':
        return 'admin.editor.images.upload.error.tooLarge';
      case 'unsupported_format':
        return 'admin.editor.images.upload.error.format';
      case 'too_many_pixels':
        return 'admin.editor.images.upload.error.pixels';
      default:
        return 'admin.editor.images.upload.error.missing';
    }
  }
  if (errors.some((entry) => entry.field === 'kind'))
    return 'admin.editor.images.upload.error.kind';
  return undefined;
}

/** The largest upload the server takes (uploadItemImage): checked here only to answer at once. */
export const MAX_UPLOAD_BYTES = 20 * 1024 * 1024;

/** The formats the server takes; the picker offers them. */
export const UPLOAD_TYPES = 'image/jpeg,image/png,image/webp';
