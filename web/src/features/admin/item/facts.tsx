import type { AdminItem, MetadataProvider } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { formatDateTime } from '@/lib/format';
import { TextLink } from '@/ui';
import { providerChain } from '@/ui/aria';

/** The fields of "Herkunft je Feld" in display order; the server may send more later. */
const SOURCE_FIELDS = [
  'title',
  'overview',
  'tagline',
  'original_title',
  'genres',
  'runtime_minutes',
  'release_date',
  'own_rating',
  'poster',
  'backdrop',
] as const;
type SourceField = (typeof SOURCE_FIELDS)[number];
const PROVIDERS: readonly MetadataProvider[] = ['tmdb', 'tvdb', 'anilist', 'local'];

const isSourceField = (field: string): field is SourceField =>
  (SOURCE_FIELDS as readonly string[]).includes(field);
const isProvider = (provider: string): provider is MetadataProvider =>
  (PROVIDERS as readonly string[]).includes(provider);
const anilistUrl = (id: number) => `https://anilist.co/anime/${id}`;

/** A provider's name; `union` stands for the genres merged from every provider. */
function providerName(t: TFunction, provider: string): string {
  if (provider === 'union') return t('admin.item.sources.union');
  return isProvider(provider) ? t(`library.provider.${provider}`) : provider;
}

/** "Herkunft je Feld" (TIT-3): field and language, then the provider that won it. */
export function sourceFacts(t: TFunction, sources: AdminItem['fieldSources']): [string, string][] {
  // Unknown fields go last, under their own name, instead of vanishing.
  const rank = (field: string) =>
    isSourceField(field) ? SOURCE_FIELDS.indexOf(field) : SOURCE_FIELDS.length;
  return [...sources]
    .sort((a, b) => rank(a.field) - rank(b.field) || a.lang.localeCompare(b.lang))
    .map((source): [string, string] => {
      const field = isSourceField(source.field)
        ? t(`admin.item.sources.field.${source.field}`)
        : source.field;
      return [
        source.lang ? t('admin.item.sources.fieldLang', { field, lang: source.lang }) : field,
        providerName(t, source.provider),
      ];
    });
}

/** The header line of «Herkunft je Feld»: how many fields each provider won, in list order. */
export function sourceSummary(t: TFunction, sources: AdminItem['fieldSources']): string {
  const counts = new Map<string, number>();
  for (const [, provider] of sourceFacts(t, sources)) {
    counts.set(provider, (counts.get(provider) ?? 0) + 1);
  }
  return [...counts]
    .map(([provider, count]) => t('admin.item.sources.providerCount', { provider, count }))
    .join(t('format.listSeparator'));
}

/** The metadata facts of a title (TIT-2): kind, ids, state, providers, locks, children. */
export function itemFacts(t: TFunction, item: AdminItem, language: string): [string, ReactNode][] {
  const none = t('admin.item.facts.none');
  const rows: [string, ReactNode][] = [
    [t('admin.item.facts.kind'), t(`kind.${item.kind}`)],
    [t('admin.item.facts.id'), item.id],
    [t('admin.item.facts.library'), String(item.libraryId)],
  ];
  if (item.seasonNumber !== undefined) {
    rows.push([t('admin.item.facts.season'), String(item.seasonNumber)]);
  }
  if (item.episodeNumber !== undefined) {
    rows.push([
      t('admin.item.facts.episode'),
      item.episodeEnd !== undefined && item.episodeEnd > item.episodeNumber
        ? t('admin.item.facts.episodeRange', { from: item.episodeNumber, to: item.episodeEnd })
        : String(item.episodeNumber),
    ]);
  }
  rows.push([t('admin.item.facts.metadata'), t(`admin.item.state.${item.metadataState}`)]);
  if (item.metadataError) rows.push([t('admin.item.facts.metadataError'), item.metadataError]);
  rows.push(
    [
      t('admin.item.facts.matchedAt'),
      item.matchedAt ? formatDateTime(item.matchedAt, language) : none,
    ],
    [t('admin.item.facts.tmdb'), item.tmdbId !== undefined ? String(item.tmdbId) : none],
    [t('admin.item.facts.tvdb'), item.tvdbId !== undefined ? String(item.tvdbId) : none],
    [t('admin.item.facts.imdb'), item.imdbId ?? none],
    [
      t('admin.item.facts.providers'),
      item.providers.length > 0 ? providerChain(t, item.providers) : none,
    ],
    // 0 means "looked up, no entry"; a found id links to the entry.
    [
      t('library.provider.anilist'),
      item.anilistId ? (
        <TextLink href={anilistUrl(item.anilistId)} external>
          {String(item.anilistId)}
        </TextLink>
      ) : (
        none
      ),
    ],
    [
      t('admin.item.facts.locked'),
      item.lockedFields.length > 0 ? item.lockedFields.join(t('format.listSeparator')) : none,
    ],
  );
  if (item.kind === 'series' || item.kind === 'season' || item.kind === 'movie') {
    rows.push([t('admin.item.facts.children'), t('admin.item.facts.childCounts', item.children)]);
  }
  return rows;
}
