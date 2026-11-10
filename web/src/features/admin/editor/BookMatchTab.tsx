import type { AdminItem, MetadataCandidate } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useMemo, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formatBookLength } from '@/lib/format';
import { formText } from '@/lib/form';
import { CandidateCard, Chip, Notice, SelectField, Skeleton, Text } from '@/ui';
import { AriaButton, DataTable, Form, SelectCheck, TextField, type DataColumn } from '@/ui/aria';
import { providerName } from '../item/facts';
import {
  audiobookSourcesQuery,
  isBookProvider,
  matchPreviewQuery,
  metadataProvidersQuery,
  type AudibleRegion,
  type BookMatchField,
  type BookProvider,
  type MatchPreview,
} from '../queries';
import styles from './MetadataEditor.module.css';

/** The Audible marketplaces the source select offers by name; the setting adds its own. */
const REGION_LABEL = {
  de: 'admin.editor.bookMatch.audible.de',
  com: 'admin.editor.bookMatch.audible.com',
  'co.uk': 'admin.editor.bookMatch.audible.couk',
} as const satisfies Partial<Record<AudibleRegion, string>>;
const NAMED_REGIONS = Object.keys(REGION_LABEL) as (keyof typeof REGION_LABEL)[];
/** Sources that are always offered when the server can ask them; the others need the folder. */
const ALWAYS = ['dnb', 'applebooks'] as const satisfies readonly BookProvider[];
const FOLDER_ONLY = ['openlibrary', 'googlebooks'] as const satisfies readonly BookProvider[];

const ASIN = /^[A-Za-z0-9]{10}$/;
const ISBN = /^[0-9][0-9-]{8,15}[0-9Xx]$/;

/** The labels of the fields of the comparison, from the editor's own field names. */
const FIELD_LABEL = {
  title: 'admin.editor.metadata.field.title',
  subtitle: 'admin.editor.metadata.field.subtitle',
  author: 'admin.editor.metadata.field.author',
  narrator: 'admin.editor.metadata.field.narrator',
  seriesName: 'admin.editor.metadata.field.seriesName',
  seriesIndex: 'admin.editor.metadata.field.seriesIndex',
  year: 'admin.editor.metadata.field.year',
  publisher: 'admin.editor.metadata.field.publisher',
  originalLanguage: 'admin.editor.metadata.field.language',
  genres: 'admin.editor.metadata.field.genres',
  overview: 'admin.editor.metadata.field.overview',
  isbn: 'admin.editor.metadata.field.isbn',
  asin: 'admin.editor.metadata.field.asin',
  cover: 'admin.editor.tabs.cover',
} as const satisfies Record<BookMatchField, string>;

/** A choice of the source select: a book source, for Audible with its marketplace. */
interface Source {
  key: string;
  provider: BookProvider;
  region?: AudibleRegion;
}

interface Search {
  provider: BookProvider;
  region?: AudibleRegion;
  query?: string;
  author?: string;
  asin?: string;
  isbn?: string;
}

/** The name of a source choice: «Audible Deutschland», «Deutsche Nationalbibliothek». */
function sourceLabel(t: TFunction, source: Source): string {
  if (source.provider !== 'audible' || !source.region) return providerName(t, source.provider);
  const region = source.region;
  return region in REGION_LABEL
    ? t(REGION_LABEL[region as keyof typeof REGION_LABEL])
    : t('admin.editor.bookMatch.audible.other', { domain: `audible.${source.region}` });
}

/**
 * The book source of a matched audiobook (`AdminItem.bookMatch`, state `matched`); undefined while
 * the book is not matched (never tried, `review`, `none`, `error`).
 */
export function bookMatchSource(item: AdminItem): BookProvider | undefined {
  const match = item.bookMatch;
  return match?.state === 'matched' && match.provider && isBookProvider(match.provider)
    ? match.provider
    : undefined;
}

/** «Zugeordnet: Audible» or «Nicht zugeordnet: Angaben nur aus den Dateien». */
export function BookMatchNote({ item }: { item: AdminItem }) {
  const { t } = useTranslation();
  const source = bookMatchSource(item);
  return (
    <Text tone="secondary" variant="caption">
      {source
        ? t('admin.editor.bookMatch.matched', { source: providerName(t, source) })
        : t('admin.editor.bookMatch.unmatched')}
    </Text>
  );
}

/** The first message of a failed request: the field's if the server named one. */
function failure(t: TFunction, error: unknown): string {
  const errors = formErrors(t, error);
  return Object.values(errors.fields)[0] ?? errors.form ?? '';
}

/**
 * The tab «Zuordnen» of an audiobook (concept 19 D5.2, mockup `Editor`): search one book source by
 * title and author or by ASIN or ISBN; the hits show cover, people, facts and length, the length
 * within the tolerance marked by the server (`durationMatches`, rule 12). A picked hit opens the
 * comparison «Was übernommen wird»; «n Felder übernehmen» stores the match with the ticked
 * fields, which the server locks.
 */
export function BookMatchTab({
  item,
  onChanged,
}: {
  item: AdminItem;
  onChanged: (next: AdminItem, message: string) => void;
}) {
  const { t } = useTranslation();
  const providers = useQuery(metadataProvidersQuery);
  const settings = useQuery({ ...audiobookSourcesQuery, retry: false });
  const filesLength = item.durationMs;

  const sources = useMemo((): Source[] => {
    const configured = (id: BookProvider) =>
      providers.data?.find((info) => info.id === id)?.configured ?? true;
    const regions: AudibleRegion[] = [...NAMED_REGIONS];
    const setting = settings.data?.audibleRegion;
    if (setting && !regions.includes(setting)) regions.unshift(setting);
    else if (setting) regions.sort((a, b) => Number(b === setting) - Number(a === setting));
    const list: Source[] = configured('audible')
      ? regions.map((region) => ({ key: `audible:${region}`, provider: 'audible', region }))
      : [];
    for (const id of ALWAYS) if (configured(id)) list.push({ key: id, provider: id });
    // Open Library and Google Books only where the folder of the book lists them.
    for (const id of FOLDER_ONLY) {
      if (item.providers.includes(id) && providers.data?.find((info) => info.id === id)?.configured)
        list.push({ key: id, provider: id });
    }
    // The folder's order decides the first choice: its first book source.
    const first = item.providers.find(isBookProvider);
    const index = list.findIndex((source) => source.provider === first);
    return index > 0 ? [list[index]!, ...list.filter((_, i) => i !== index)] : list;
  }, [providers.data, settings.data?.audibleRegion, item.providers]);

  const [picked, setSource] = useState<string | null>(null);
  const source = sources.find((entry) => entry.key === picked) ?? sources[0];
  const [search, setSearch] = useState<Search | null>(null);
  const [needQuery, setNeedQuery] = useState(false);
  const [chosen, setChosen] = useState<MetadataCandidate | null>(null);

  const results = useQuery({
    queryKey: ['admin', 'metadata-search', 'audiobook', item.id, search],
    queryFn: ({ signal }): Promise<MetadataCandidate[]> =>
      request(
        api.GET('/admin/metadata/search', {
          params: {
            query: {
              provider: search?.provider ?? 'audible',
              kind: 'audiobook',
              query: search?.query,
              author: search?.author,
              asin: search?.asin,
              isbn: search?.isbn,
              region: search?.region,
              itemId: item.id,
            },
          },
          signal,
        }),
      ),
    enabled: search !== null,
    retry: false,
  });

  const lengthOf = (ms: number | undefined) =>
    ms ? formatBookLength(t, ms) : t('admin.editor.bookMatch.lengthUnknown');

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!source) return;
    const data = new FormData(event.currentTarget);
    const query = formText(data, 'query');
    const asin = formText(data, 'asin');
    const isbn = formText(data, 'isbn');
    if (!query && !asin && !isbn) {
      setNeedQuery(true);
      return;
    }
    if ((asin && !ASIN.test(asin)) || (isbn && !ISBN.test(isbn))) return;
    setNeedQuery(false);
    const author = formText(data, 'author');
    const next: Search = {
      provider: source.provider,
      ...(source.region ? { region: source.region } : {}),
      ...(query ? { query } : {}),
      ...(author ? { author } : {}),
      ...(asin ? { asin } : {}),
      ...(isbn ? { isbn } : {}),
    };
    setChosen(null);
    // The same search again (after «nicht erreichbar»): same query key, so ask anew.
    if (search !== null && JSON.stringify(next) === JSON.stringify(search)) void results.refetch();
    else setSearch(next);
  };

  const candidateLines = (candidate: MetadataCandidate): string[] => {
    const authors = (candidate.authors ?? []).join(t('format.listSeparator'));
    const narrators = (candidate.narrators ?? []).join(t('format.listSeparator'));
    const people = narrators ? t('admin.editor.bookMatch.people', { authors, narrators }) : authors;
    const id = candidate.asin
      ? t('admin.editor.bookMatch.asinId', { id: candidate.asin })
      : candidate.isbn
        ? t('admin.editor.bookMatch.isbnId', { id: candidate.isbn })
        : candidate.providerId;
    const facts = [candidate.publisher, candidate.year?.toString(), id]
      .filter(Boolean)
      .join(t('format.separator'));
    return [people, facts].filter(Boolean);
  };
  const candidateTitle = (candidate: MetadataCandidate) =>
    candidate.subtitle ? `${candidate.title} – ${candidate.subtitle}` : candidate.title;
  const lengthBadge = (candidate: MetadataCandidate) =>
    candidate.durationMs ? (
      candidate.durationMatches ? (
        <Chip tone="success" icon="check">
          {lengthOf(candidate.durationMs)}
        </Chip>
      ) : (
        <Chip tone="outline">{lengthOf(candidate.durationMs)}</Chip>
      )
    ) : (
      <Chip tone="outline">{lengthOf(undefined)}</Chip>
    );
  const lengthWords = (candidate: MetadataCandidate) =>
    candidate.durationMatches && candidate.durationMs
      ? t('admin.editor.bookMatch.lengthFits', { length: lengthOf(candidate.durationMs) })
      : lengthOf(candidate.durationMs);

  const metadata = item.metadata;
  return (
    <div className={styles.stack}>
      <Text tone="secondary">{t('admin.editor.bookMatch.intro')}</Text>
      <Form
        className={styles.stack}
        onSubmit={onSubmit}
        validationErrors={needQuery ? { query: t('admin.editor.bookMatch.needQuery') } : undefined}
      >
        {source && (
          <SelectField
            label={t('admin.editor.bookMatch.source')}
            value={source.key}
            onChange={(key) => {
              setSource(key);
              setChosen(null);
            }}
            options={sources.map((entry) => ({ value: entry.key, label: sourceLabel(t, entry) }))}
          />
        )}
        <div className={styles.fields}>
          <TextField
            name="query"
            label={t('admin.editor.bookMatch.title')}
            defaultValue={metadata?.title ?? item.title}
            maxLength={200}
          />
          <TextField
            name="author"
            label={t('admin.editor.bookMatch.author')}
            defaultValue={metadata?.author ?? ''}
            maxLength={200}
          />
          <TextField
            name="asin"
            label={t('admin.editor.bookMatch.asin')}
            description={t('admin.editor.bookMatch.idHint')}
            mono
            validate={(value) =>
              value.trim() && !ASIN.test(value.trim())
                ? t('admin.editor.bookMatch.asinInvalid')
                : null
            }
          />
          <TextField
            name="isbn"
            label={t('admin.editor.bookMatch.isbn')}
            mono
            validate={(value) =>
              value.trim() && !ISBN.test(value.trim())
                ? t('admin.editor.bookMatch.isbnInvalid')
                : null
            }
          />
        </div>
        <div className={styles.actions}>
          <AriaButton
            type="submit"
            variant="primary"
            icon="search"
            isPending={results.isFetching}
            isDisabled={!source}
          >
            {results.isFetching ? t('admin.match.searching') : t('admin.match.search')}
          </AriaButton>
          {filesLength !== undefined && (
            <Text tone="secondary" variant="caption">
              {t('admin.editor.bookMatch.filesLength', { length: lengthOf(filesLength) })}
            </Text>
          )}
        </div>
      </Form>

      {results.isError && <Notice tone="danger">{failure(t, results.error)}</Notice>}
      {results.isSuccess && results.data.length === 0 && <Notice>{t('admin.match.none')}</Notice>}
      {results.isSuccess && results.data.length > 0 && (
        <section className={styles.stack} aria-labelledby="book-match-results">
          <h3 id="book-match-results" className={styles.sectionTitle}>
            {t('admin.match.results')}
          </h3>
          <Text tone="secondary" variant="caption">
            {t('admin.editor.bookMatch.sorted')}
          </Text>
          <ul className={styles.candidates}>
            {results.data.map((candidate) => (
              <li key={`${candidate.provider}-${candidate.providerId}`}>
                <CandidateCard
                  title={candidateTitle(candidate)}
                  details={candidateLines(candidate)}
                  posterUrl={candidate.coverUrl ?? candidate.posterUrl}
                  shape="square"
                  badge={lengthBadge(candidate)}
                  isSelected={
                    chosen?.provider === candidate.provider &&
                    chosen.providerId === candidate.providerId
                  }
                  aria-label={t('admin.editor.bookMatch.choose', {
                    title: candidateTitle(candidate),
                    length: lengthWords(candidate),
                  })}
                  onPress={() => setChosen(candidate)}
                />
              </li>
            ))}
          </ul>
        </section>
      )}
      {chosen && isBookProvider(chosen.provider) && (
        <MatchComparison
          // A new pick starts with its own ticks.
          key={`${chosen.provider}-${chosen.providerId}`}
          item={item}
          provider={chosen.provider}
          providerId={chosen.providerId}
          region={chosen.region ?? search?.region}
          onApplied={(next, count) => {
            setChosen(null);
            onChanged(
              next,
              t('admin.editor.bookMatch.preview.applied', {
                count,
                source: providerName(t, chosen.provider),
              }),
            );
          }}
        />
      )}
    </div>
  );
}

/** The display text of a value of the comparison; a language code reads as its name. */
function useDisplayValue() {
  const { t, i18n } = useTranslation();
  const names = useMemo(
    () => new Intl.DisplayNames([i18n.language], { type: 'language' }),
    [i18n.language],
  );
  return (field: BookMatchField, value: string, side: 'current' | 'proposed') => {
    if (!value) return t('admin.editor.bookMatch.preview.empty');
    if (field === 'cover')
      return side === 'current'
        ? t('admin.editor.bookMatch.preview.coverToday')
        : t('admin.editor.bookMatch.preview.cover');
    if (field === 'originalLanguage' && /^[a-z]{2,3}$/.test(value)) return names.of(value) ?? value;
    return value;
  };
}

type PreviewRow = MatchPreview['fields'][number];

/**
 * «Was übernommen wird»: every field the entry changes, today and new, with a tick; a locked field
 * is greyed and cannot be ticked. Every differing free field starts ticked. The button stores the
 * match with the ticked fields (`setItemMatch.fields`).
 */
function MatchComparison({
  item,
  provider,
  providerId,
  region,
  onApplied,
}: {
  item: AdminItem;
  provider: BookProvider;
  providerId: string;
  region?: AudibleRegion;
  onApplied: (next: AdminItem, count: number) => void;
}) {
  const { t } = useTranslation();
  const preview = useQuery(
    matchPreviewQuery(item.id, provider, providerId, provider === 'audible' ? region : undefined),
  );
  const [untaken, setUntaken] = useState<ReadonlySet<BookMatchField>>(() => new Set());
  const display = useDisplayValue();
  const rows = (preview.data?.fields ?? []).filter((row) => row.differs);
  const taken = rows
    .filter((row) => !row.locked && !untaken.has(row.field))
    .map((row) => row.field);

  const apply = useMutation({
    mutationFn: (): Promise<AdminItem> =>
      request(
        api.PUT('/admin/items/{id}/match', {
          params: { path: { id: item.id } },
          body: {
            provider,
            providerId,
            ...(provider === 'audible' && region ? { region } : {}),
            fields: taken,
          },
        }),
      ),
    onSuccess: (next) => onApplied(next, taken.length),
  });

  const label = (field: BookMatchField) => t(FIELD_LABEL[field]);
  const columns: DataColumn<PreviewRow>[] = [
    {
      key: 'field',
      header: t('admin.editor.bookMatch.preview.field'),
      rowHeader: true,
      cell: (row) => (
        <span className={row.locked ? styles.muted : undefined}>{label(row.field)}</span>
      ),
    },
    {
      key: 'today',
      header: t('admin.editor.bookMatch.preview.today'),
      cell: (row) => (
        <span className={styles.previewValue}>{display(row.field, row.current, 'current')}</span>
      ),
    },
    {
      key: 'new',
      header: t('admin.editor.bookMatch.preview.new'),
      cell: (row) =>
        row.field === 'cover' && row.proposed ? (
          <img
            className={styles.previewCover}
            src={row.proposed}
            alt={display(row.field, row.proposed, 'proposed')}
            width={48}
            height={48}
            loading="lazy"
            referrerPolicy="no-referrer"
          />
        ) : (
          <span className={row.locked ? styles.muted : styles.previewValue}>
            {display(row.field, row.proposed, 'proposed')}
          </span>
        ),
    },
    {
      key: 'take',
      header: t('admin.editor.bookMatch.preview.take'),
      align: 'end',
      cell: (row) =>
        row.locked ? (
          <Chip tone="outline" icon="lock">
            {t('admin.editor.bookMatch.preview.locked')}
          </Chip>
        ) : (
          <SelectCheck
            label={t('admin.editor.bookMatch.preview.takeLabel', { field: label(row.field) })}
            isSelected={!untaken.has(row.field)}
            onChange={(on) =>
              setUntaken((current) => {
                const next = new Set(current);
                if (on) next.delete(row.field);
                else next.add(row.field);
                return next;
              })
            }
          />
        ),
    },
  ];

  return (
    <section className={styles.stack} aria-labelledby="book-match-preview">
      <h3 id="book-match-preview" className={styles.sectionTitle}>
        {t('admin.editor.bookMatch.preview.title')}
      </h3>
      {preview.isPending && <Skeleton shape="block" />}
      {preview.isError && <Notice tone="danger">{failure(t, preview.error)}</Notice>}
      {preview.data && (
        <>
          <Text tone="secondary">
            {t('admin.editor.bookMatch.preview.entry', {
              title: preview.data.title,
              source: providerName(t, provider),
            })}
          </Text>
          {rows.length === 0 ? (
            <Notice>{t('admin.editor.bookMatch.preview.same')}</Notice>
          ) : (
            <DataTable
              caption={t('admin.editor.bookMatch.preview.caption', {
                source: providerName(t, provider),
              })}
              columns={columns}
              rows={rows}
              rowKey={(row) => row.field}
            />
          )}
          <div className={styles.actions}>
            <AriaButton
              variant="primary"
              isDisabled={taken.length === 0}
              isPending={apply.isPending}
              onPress={() => apply.mutate()}
            >
              {t('admin.editor.bookMatch.preview.apply', { count: taken.length })}
            </AriaButton>
            <Text tone="secondary" variant="caption">
              {t('admin.editor.bookMatch.preview.hint')}
            </Text>
          </div>
          {apply.isError && <Notice tone="danger">{failure(t, apply.error)}</Notice>}
        </>
      )}
    </section>
  );
}
