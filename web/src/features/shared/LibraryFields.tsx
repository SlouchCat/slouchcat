import type { LibraryRequest, MetadataProviderInfo } from '@slouchcat/api-client';
import type { ReactNode, RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { FolderList, RadioGroup, newFolderEntry, type FolderEntry } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from './choices';

export type LibraryKind = LibraryRequest['kind'];

const KINDS = ['video', 'audiobooks', 'podcasts'] as const satisfies LibraryKind[];

/** The kind from a radio value; unknown values fall back to video. */
export const libraryKindOf = (value: string): LibraryKind =>
  KINDS.find((kind) => kind === value) ?? 'video';

/** Audiobooks and podcasts have a default age rating of their own (AN-2). */
export const isAudioLibrary = (kind: LibraryKind) => kind === 'audiobooks' || kind === 'podcasts';

/** How many folders the form of a new library keeps at least: podcasts may do without (F-13). */
export const minFolders = (kind: LibraryKind) => (kind === 'podcasts' ? 0 : 1);

/** The cards a new library of this kind starts with: one, none for podcasts. */
export function initialFolders(kind: LibraryKind): FolderEntry[] {
  return kind === 'podcasts' ? [] : [newFolderEntry(kind, [])];
}

/**
 * Body of createLibrary (and the library of the setup wizard): every card of the folder list as
 * one root with its content and provider list; only an audio library sends its default rating.
 */
export function libraryRequest(
  name: string,
  kind: LibraryKind,
  folders: readonly FolderEntry[],
  rating: string,
): LibraryRequest {
  const defaultRating = isAudioLibrary(kind) ? titleRatingOf(rating) : undefined;
  return {
    name,
    kind,
    roots: folders.map((folder) => ({
      path: folder.path.trim(),
      content: folder.content,
      providers: folder.providers,
    })),
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
  folders: readonly FolderEntry[];
  onFoldersChange: (folders: FolderEntry[]) => void;
  /** What `listMetadataProviders` said; the wizard has no session and shows the defaults fixed. */
  providerInfos?: readonly MetadataProviderInfo[];
  /** Rendered right after the path field of a card (the browse button, the wizard's check). */
  pathAddon?: (folder: FolderEntry, inputRef: RefObject<HTMLInputElement | null>) => ReactNode;
  onPathBlur?: (folder: FolderEntry) => void;
  /** The server's field messages by input name, for the parts of a card without an input. */
  fieldErrors?: Readonly<Record<string, string>>;
}

/**
 * Kind, folders and default age rating of a new library (admin page and setup wizard, ADR 0015):
 * the three families «Video», «Hörbücher», «Podcasts»; the folder list with content and
 * metadata sources per card, which a podcast library may leave empty (its titles come from
 * feeds); the default rating (FSK 0 … 18) for audiobooks and podcasts only.
 */
export function LibraryFields({
  prefix = '',
  kind,
  onKindChange,
  rating,
  onRatingChange,
  folders,
  onFoldersChange,
  providerInfos,
  pathAddon,
  onPathBlur,
  fieldErrors,
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
      <Text tone="secondary" variant="caption">
        {kind === 'podcasts' ? t('library.podcastsHint') : t('library.videoHint')}
      </Text>
      <FolderList
        kind={kind}
        entries={folders}
        onChange={onFoldersChange}
        infos={providerInfos}
        min={minFolders(kind)}
        namePrefix={`${prefix}roots`}
        pathAddon={pathAddon}
        onPathBlur={onPathBlur}
        fieldErrors={fieldErrors}
      />
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
