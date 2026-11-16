import type {
  LibraryRequest,
  LibraryRoot,
  LibraryRootUpdate,
  MetadataSettings,
  RootImages,
} from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, CheckboxGroup, RadioGroup, SettingRow } from '@/ui/aria';
import { metadataProvidersForQuery, metadataSettingsQuery } from '../queries';
import {
  LanguageRanks,
  chainText,
  unknownLanguages,
  useLanguageName,
  useListOf,
  youtubeTitles,
} from './languages';
import styles from './FolderMetadataFields.module.css';

/** One folder of `createLibrary` and `addLibraryRoot`. */
type LibraryRootRequest = LibraryRequest['roots'][number];

/** The chains of a folder's own images, in the order of the page «Metadaten». */
const CHAIN_KINDS = ['poster', 'backdrop', 'logo'] as const;
type ChainKind = (typeof CHAIN_KINDS)[number];

/** The switchable image kinds (poster and backdrop are always on, E-1). */
const SWITCHABLE = ['logo', 'season_poster', 'episode_still'] as const;
type Switchable = (typeof SWITCHABLE)[number];

const switchable = (kinds: readonly string[]): Switchable[] =>
  SWITCHABLE.filter((kind) => kinds.includes(kind));

/**
 * What the form holds for one folder's languages and images (concept 24 4.2). `own*` is the
 * choice «Eigene»; the list beside it stays while «Wie Standard» is chosen, so switching back and
 * forth loses nothing before the save. A list is undefined until «Eigene» was chosen once.
 */
export interface FolderMetadataDraft {
  ownLangs: boolean;
  textLangs?: string[];
  ownImages: boolean;
  images?: RootImages;
}

/** The draft of a folder as the server holds it; a new folder follows the default. */
export function folderMetadataDraft(
  root?: Pick<LibraryRoot, 'textLangs' | 'images'>,
): FolderMetadataDraft {
  return {
    ownLangs: Boolean(root?.textLangs),
    ...(root?.textLangs ? { textLangs: [...root.textLangs] } : {}),
    ownImages: Boolean(root?.images),
    ...(root?.images ? { images: copyImages(root.images) } : {}),
  };
}

const copyImages = (images: RootImages): RootImages => ({
  langs: Object.fromEntries(
    Object.entries(images.langs).map(([kind, chain]) => [kind, [...chain]]),
  ),
  kinds: [...images.kinds],
});

/** The draft's values: `null` = like the default (the whole state, as `saveLibrary` takes it). */
export function folderMetadataValues(draft: FolderMetadataDraft): {
  textLangs: string[] | null;
  images: RootImages | null;
} {
  return {
    textLangs: draft.ownLangs ? (draft.textLangs ?? null) : null,
    images: draft.ownImages ? (draft.images ?? null) : null,
  };
}

/** The fields of a new folder (createLibrary, addLibraryRoot): only its own values are sent. */
export function folderMetadataRequest(
  draft: FolderMetadataDraft | undefined,
): Partial<Pick<LibraryRootRequest, 'textLangs' | 'images'>> {
  if (!draft) return {};
  const { textLangs, images } = folderMetadataValues(draft);
  return { ...(textLangs ? { textLangs } : {}), ...(images ? { images } : {}) };
}

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

/**
 * The fields of a changed folder (updateLibraryRoot, concept 24 3.6): left out when unchanged,
 * `null` back to the default, the list when it is the folder's own.
 */
export function folderMetadataUpdate(
  draft: FolderMetadataDraft,
  root: Pick<LibraryRoot, 'textLangs' | 'images'>,
): Partial<Pick<LibraryRootUpdate, 'textLangs' | 'images'>> {
  const { textLangs, images } = folderMetadataValues(draft);
  return {
    ...(same(textLangs, root.textLangs) ? {} : { textLangs }),
    ...(same(images, root.images) ? {} : { images }),
  };
}

/** Whether a new folder's draft has something of its own. */
export const hasOwnMetadata = (draft: FolderMetadataDraft | undefined) =>
  draft !== undefined && (draft.ownLangs || draft.ownImages);

/** The default's images as a folder's own: every chain and the switchable kinds. */
const defaultImages = (settings: MetadataSettings): RootImages => ({
  langs: {
    poster: [...settings.imageLangs.poster],
    backdrop: [...settings.imageLangs.backdrop],
    logo: [...settings.imageLangs.logo],
  },
  kinds: switchable(settings.imageKinds),
});

interface FolderMetadataFieldsProps {
  value: FolderMetadataDraft;
  onChange: (value: FolderMetadataDraft) => void;
  /** Unique per form: the radio groups of two cards must not share a name. */
  name: string;
  /**
   * Prefix of the server's field names: `''` for a changed folder (`textLangs`), `roots[1].` for
   * a new one of a list (`roots[1].images.langs.poster`).
   */
  fieldPrefix?: string;
  /** The server's field messages by name. */
  fieldErrors?: Readonly<Record<string, string>>;
}

/**
 * «Sprachen und Bilder eines Ordners» (concept 24 4.2, ADR 0036): the rows «Sprachen» and
 * «Bilder», each «Wie Standard» (the default's value beside it) or «Eigene». «Eigene» starts from
 * the default; the place list of the languages, the switchable image kinds and the language per
 * image kind as on the page «Metadaten», with the hints which source does not know a language
 * (for this list), what a language costs and the YouTube quota. Only video folders have it. The
 * form that holds the folder sends the draft (folderMetadataRequest, folderMetadataUpdate).
 */
export function FolderMetadataFields({
  value,
  onChange,
  name,
  fieldPrefix = '',
  fieldErrors,
}: FolderMetadataFieldsProps) {
  const { t } = useTranslation();
  const nameOf = useLanguageName();
  const listOf = useListOf();
  const settings = useQuery(metadataSettingsQuery);
  const langs = value.ownLangs ? value.textLangs : undefined;
  const providers = useQuery({
    ...metadataProvidersForQuery(langs ?? []),
    enabled: langs !== undefined && langs.length > 0,
  });
  // The previous answer belongs to the previous list: no hint until this list's answer is there.
  const providersOfList = providers.isPlaceholderData ? undefined : providers.data;
  // The rows start from the default and name it: without it there is nothing to choose yet.
  if (settings.isPending) return <Skeleton shape="block" />;
  if (!settings.data)
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, settings.error)}
        actions={
          <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
            {t('common.retry')}
          </AriaButton>
        }
      />
    );
  const defaults = settings.data;
  const known = defaults.knownLanguages;
  const error = (field: string) => fieldErrors?.[`${fieldPrefix}${field}`];
  const textLangsError = error('textLangs');
  const kindsError = error('images.kinds');

  const images = value.ownImages ? (value.images ?? defaultImages(defaults)) : undefined;
  const chainOf = (kind: ChainKind) => images?.langs[kind] ?? defaults.imageLangs[kind];
  const setImages = (next: RootImages) => onChange({ ...value, images: next });
  const defaultKinds = switchable(defaults.imageKinds);
  // «Wie Standard» names the default's kinds and the language of each kind it fetches (4.2).
  const defaultImagesText = t('admin.metadata.folder.imagesDefaultIs', {
    kinds: listOf([
      t('admin.metadata.images.kind.poster'),
      t('admin.metadata.images.kind.backdrop'),
      ...defaultKinds.map((kind) => t(`admin.metadata.images.kind.${kind}`)),
    ]),
    chains: listOf(
      CHAIN_KINDS.filter((kind) => kind !== 'logo' || defaultKinds.includes('logo')).map((kind) =>
        t('admin.metadata.folder.imageChain', {
          kind: t(`admin.metadata.images.kind.${kind}`),
          chain: chainText(t, nameOf, defaults.imageLangs[kind]),
        }),
      ),
    ),
  });
  const choices = [
    { value: 'default', label: t('admin.metadata.folder.likeDefault') },
    { value: 'own', label: t('admin.metadata.folder.own') },
  ];
  // A message of the whole image setting; the kinds and each chain show theirs at the field.
  const imageErrors = [error('images'), error('images.langs')].filter(
    (message): message is string => Boolean(message),
  );

  return (
    <div className={styles.fields}>
      <RadioGroup
        name={`${name}.langsMode`}
        label={t('admin.metadata.languages.title')}
        description={
          value.ownLangs
            ? t('admin.metadata.folder.onlyHere')
            : t('admin.metadata.folder.defaultIs', {
                value: chainText(t, nameOf, defaults.textLangs),
              })
        }
        value={value.ownLangs ? 'own' : 'default'}
        onChange={(mode) =>
          onChange({
            ...value,
            ownLangs: mode === 'own',
            // «Eigene» starts from the default; a list chosen before comes back.
            textLangs: value.textLangs ?? [...defaults.textLangs],
          })
        }
        options={choices}
      />
      {langs && (
        <div className={styles.own}>
          <SettingRow label={t('admin.metadata.languages.list')}>
            <LanguageRanks
              value={langs}
              onChange={(next) => onChange({ ...value, textLangs: next })}
              known={known}
            />
          </SettingRow>
          <Text tone="secondary" variant="caption">
            {[
              ...(providersOfList
                ? unknownLanguages(t, nameOf, listOf, providersOfList, langs)
                : []),
              t('admin.metadata.folder.cost'),
              t('admin.metadata.languages.youtube', {
                count: langs.length,
                titles: youtubeTitles(langs),
              }),
            ].join(' ')}
          </Text>
        </div>
      )}
      {textLangsError && <Notice tone="danger">{textLangsError}</Notice>}

      <RadioGroup
        name={`${name}.imagesMode`}
        label={t('admin.metadata.images.title')}
        description={value.ownImages ? t('admin.metadata.folder.onlyHere') : defaultImagesText}
        value={value.ownImages ? 'own' : 'default'}
        onChange={(mode) =>
          onChange({
            ...value,
            ownImages: mode === 'own',
            images: value.images ?? defaultImages(defaults),
          })
        }
        options={choices}
      />
      {images && (
        <div className={styles.own}>
          <CheckboxGroup
            label={t('admin.metadata.images.kinds')}
            description={t('admin.metadata.folder.always')}
            value={images.kinds}
            onChange={(kinds) => setImages({ ...images, kinds: switchable(kinds) })}
            options={SWITCHABLE.map((kind) => ({
              value: kind,
              label: t(`admin.metadata.images.kind.${kind}`),
              description: t(`admin.metadata.images.kindHint.${kind}`),
            }))}
          />
          {kindsError && <Notice tone="danger">{kindsError}</Notice>}
          {CHAIN_KINDS.map((kind) => (
            <div key={kind} className={styles.own}>
              <SettingRow
                label={t('admin.metadata.images.chainOf', {
                  kind: t(`admin.metadata.images.kind.${kind}`),
                })}
              >
                <LanguageRanks
                  value={chainOf(kind)}
                  onChange={(chain) =>
                    setImages({ ...images, langs: { ...images.langs, [kind]: chain } })
                  }
                  known={known}
                  allowNoText
                />
              </SettingRow>
              {error(`images.langs.${kind}`) && (
                <Notice tone="danger">{error(`images.langs.${kind}`)}</Notice>
              )}
            </div>
          ))}
        </div>
      )}
      {imageErrors.map((message) => (
        <Notice key={message} tone="danger">
          {message}
        </Notice>
      ))}
    </div>
  );
}
