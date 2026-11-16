import type { LibraryRoot, MetadataProviderInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { RankedChoiceList } from '@/ui/aria';
import { intlLocale } from '@/i18n';

/** The display name of a language code in the interface language (rule 9: no name in the code). */
export function useLanguageName(): (code: string) => string {
  const { i18n } = useTranslation();
  const names = useMemo(
    () => new Intl.DisplayNames([intlLocale(i18n.language)], { type: 'language' }),
    [i18n.language],
  );
  return (code: string) => (code ? (names.of(code) ?? code) : '');
}

/** A list of names joined the way the interface language writes «a, b and c». */
export function useListOf(): (items: string[]) => string {
  const { i18n } = useTranslation();
  const format = useMemo(
    () => new Intl.ListFormat(intlLocale(i18n.language), { type: 'conjunction' }),
    [i18n.language],
  );
  return (items: string[]) => format.format(items);
}

/**
 * An ordered list of language codes as fixed places (concept 12, 10.2): «1. Sprache», «2.
 * Sprache» … each with a select of every known language; picking one that sits on another place
 * swaps the two. «Weitere Sprache» appends a place, «×» removes one, except the last one; a chain
 * offers "" (no text). No list has to hold German or English (concept 22 F-7, ADR 0033).
 */
export function LanguageRanks({
  value,
  onChange,
  known,
  allowNoText,
  disabled,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  known: readonly string[];
  allowNoText?: boolean;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const nameOf = useLanguageName();
  const choices = useMemo(() => {
    const codes = [...new Set([...known, ...value.filter((code) => code !== '')])];
    const list = codes.map((code) => ({ value: code, label: nameOf(code) }));
    list.sort((a, b) => a.label.localeCompare(b.label));
    return allowNoText ? [{ value: '', label: t('admin.metadata.images.noText') }, ...list] : list;
  }, [known, value, allowNoText, nameOf, t]);
  return (
    <RankedChoiceList
      choices={choices}
      value={value}
      onChange={onChange}
      rankLabel={(index) => t('admin.metadata.languages.rank', { position: index + 1 })}
      addLabel={t('admin.metadata.languages.more')}
      isDisabled={disabled}
    />
  );
}

/** Which languages of a list a provider does not know (6.1, 12.3), one line per provider. */
export function unknownLanguages(
  t: TFunction,
  nameOf: (code: string) => string,
  listOf: (items: string[]) => string,
  providers: readonly MetadataProviderInfo[],
  langs: string[],
): string[] {
  const lines: string[] = [];
  for (const p of providers) {
    if (p.id === 'local') continue;
    const missing = langs.filter((code) => !p.languages.includes(code));
    if (missing.length > 0 && p.languages.length > 0) {
      lines.push(
        t('admin.metadata.languages.unknownAt', {
          provider: p.id.toUpperCase(),
          languages: listOf(missing.map(nameOf)),
        }),
      );
    }
  }
  return lines;
}

/** The YouTube quota: 10'000 units a day, a search costs 100 (ADR 0006). */
const YOUTUBE_SEARCHES_PER_DAY = 100;

/** How many titles a day the trailer search reaches with this many base languages. */
export function youtubeTitles(langs: readonly string[]): number {
  return Math.floor(
    YOUTUBE_SEARCHES_PER_DAY / Math.max(1, new Set(langs.map((c) => c.split('-')[0])).size),
  );
}

/** A language list in one line: «Deutsch › Englisch». */
export function chainText(
  t: TFunction,
  nameOf: (code: string) => string,
  langs: readonly string[],
): string {
  return langs
    .map((code) => (code === '' ? t('admin.metadata.images.noText') : nameOf(code)))
    .join(t('admin.metadata.areas.separator'));
}

/**
 * A folder's languages and images in one line each (concept 24 4.2, 4.4): «Sprachen: Standard»
 * or its own chain, «Bilder: Standard» or «Bilder: eigene».
 */
export function useFolderMetadataText(): {
  langs: (root: Pick<LibraryRoot, 'textLangs'>) => string;
  images: (root: Pick<LibraryRoot, 'images'>) => string;
  isOwn: (root: Pick<LibraryRoot, 'textLangs' | 'images'>) => boolean;
} {
  const { t } = useTranslation();
  const nameOf = useLanguageName();
  return {
    langs: (root) =>
      root.textLangs === null
        ? t('admin.metadata.folder.langsDefault')
        : t('admin.metadata.folder.langsOwn', { chain: chainText(t, nameOf, root.textLangs) }),
    images: (root) =>
      root.images === null
        ? t('admin.metadata.folder.imagesDefault')
        : t('admin.metadata.folder.imagesOwn'),
    isOwn: (root) => root.textLangs !== null || root.images !== null,
  };
}
