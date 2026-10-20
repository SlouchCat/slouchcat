import type { LibraryRequest } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { RadioGroup, TextField } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from './choices';

export type LibraryKind = LibraryRequest['kind'];

const KINDS = ['movies', 'series', 'audiobooks', 'podcasts'] as const satisfies LibraryKind[];

/** The kind from a radio value; unknown values fall back to films. */
export const libraryKindOf = (value: string): LibraryKind =>
  KINDS.find((kind) => kind === value) ?? 'movies';

/** Audiobooks and podcasts have a default age rating of their own (AN-2). */
export const isAudioLibrary = (kind: LibraryKind) => kind === 'audiobooks' || kind === 'podcasts';

/**
 * Body of createLibrary (and the library of the setup wizard): a podcast library has no folder,
 * only an audio library sends its default age rating.
 */
export function libraryRequest(
  name: string,
  kind: LibraryKind,
  path: string,
  rating: string,
): LibraryRequest {
  const defaultRating = isAudioLibrary(kind) ? titleRatingOf(rating) : undefined;
  return {
    name,
    kind,
    ...(kind === 'podcasts' ? {} : { path }),
    ...(defaultRating !== undefined ? { defaultRating } : {}),
  };
}

interface LibraryFieldsProps {
  /** Prefix of the input names: none on the admin page, `library.` in the setup wizard. */
  prefix?: string;
  kind: LibraryKind;
  onKindChange: (kind: LibraryKind) => void;
  /** Default age rating as a radio value ("0" … "18"). */
  rating: string;
  onRatingChange: (rating: string) => void;
}

/**
 * Kind, folder and default age rating of a new library (admin page and setup wizard): four kinds,
 * the folder for every kind but podcasts (their titles come from feeds), the default rating
 * (FSK 0 … 18) for audiobooks and podcasts only.
 */
export function LibraryFields({
  prefix = '',
  kind,
  onKindChange,
  rating,
  onRatingChange,
}: LibraryFieldsProps) {
  const { t } = useTranslation();
  return (
    <>
      <RadioGroup
        name={`${prefix}kind`}
        label={t('setup.library.kind')}
        value={kind}
        onChange={(value) => onKindChange(libraryKindOf(value))}
        options={KINDS.map((value) => ({ value, label: t(`library.kind.${value}`) }))}
      />
      {kind === 'podcasts' ? (
        <Text tone="secondary" variant="caption">
          {t('library.podcastsHint')}
        </Text>
      ) : (
        <TextField
          name={`${prefix}path`}
          label={t('setup.library.path')}
          description={t('setup.library.pathHint')}
          autoComplete="off"
          spellCheck="false"
          mono
          isRequired
          validate={(value) => {
            if (!value.trim()) return t('errors.field.required');
            return value.trim().startsWith('/') ? null : t('errors.field.not_absolute');
          }}
        />
      )}
      {isAudioLibrary(kind) && (
        <RadioGroup
          name={`${prefix}defaultRating`}
          label={t('library.defaultRating')}
          description={t('library.defaultRatingHint')}
          value={rating}
          onChange={onRatingChange}
          options={ratingOptions(t)}
        />
      )}
    </>
  );
}
