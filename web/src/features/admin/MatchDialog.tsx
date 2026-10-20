import type { AdminItem, MetadataCandidate, MetadataProvider } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formText } from '@/lib/form';
import { CandidateCard, Notice } from '@/ui';
import { AriaButton, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import styles from './MaintenancePage.module.css';

type MatchKind = 'movie' | 'series';

interface Search {
  provider: MetadataProvider;
  kind: MatchKind;
  query: string;
  year?: number;
}

interface MatchDialogProps {
  itemId: string;
  /** Starting values of the search: the parsed title, its year and kind. */
  title: string;
  year?: number;
  kind: MatchKind;
  onClose: () => void;
  /** After the assignment was stored (202). */
  onMatched: (item: AdminItem) => void;
}

function parseYear(value: string): number | undefined | null {
  if (!value) return undefined;
  const year = Number(value);
  return Number.isInteger(year) && year >= 1800 && year <= 2100 ? year : null;
}

/**
 * Manual assignment of a title to a provider entry (B-6): search TMDB or TVDB, pick a candidate,
 * the server stores its id and refreshes the title. Shared by the maintenance and the item page.
 */
export function MatchDialog({ itemId, title, year, kind, onClose, onMatched }: MatchDialogProps) {
  const { t } = useTranslation();
  const [provider, setProvider] = useState<MetadataProvider>('tmdb');
  const candidateTitle = (candidate: MetadataCandidate) =>
    candidate.year !== undefined
      ? t('admin.titleYear', { title: candidate.title, year: candidate.year })
      : candidate.title;
  const [searchKind, setSearchKind] = useState<MatchKind>(kind);
  const [search, setSearch] = useState<Search | null>(null);

  const results = useQuery({
    queryKey: ['admin', 'metadata-search', search],
    queryFn: ({ signal }): Promise<MetadataCandidate[]> =>
      request(
        api.GET('/admin/metadata/search', {
          params: {
            query: {
              provider: search?.provider ?? 'tmdb',
              kind: search?.kind ?? 'movie',
              query: search?.query ?? '',
              year: search?.year,
            },
          },
          signal,
        }),
      ),
    enabled: search !== null,
    retry: false,
  });

  const match = useMutation({
    mutationFn: (candidate: MetadataCandidate): Promise<AdminItem> =>
      request(
        api.PUT('/admin/items/{id}/match', {
          params: { path: { id: itemId } },
          body: { provider: candidate.provider, providerId: candidate.providerId },
        }),
      ),
    onSuccess: (item) => {
      onMatched(item);
      onClose();
    },
  });

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const parsedYear = parseYear(formText(data, 'year'));
    if (parsedYear === null) return;
    const next: Search = {
      provider,
      // TVDB knows series only.
      kind: provider === 'tvdb' ? 'series' : searchKind,
      query: formText(data, 'query'),
      year: parsedYear,
    };
    // The same search again (after "TMDB nicht erreichbar"): same query key, so ask anew.
    if (search !== null && JSON.stringify(next) === JSON.stringify(search)) void results.refetch();
    else setSearch(next);
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.match.title')}
      description={t('admin.match.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit}>
        <TextField
          name="query"
          label={t('admin.match.query')}
          defaultValue={title}
          isRequired
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <TextField
          name="year"
          label={t('admin.match.year')}
          defaultValue={year !== undefined ? String(year) : ''}
          inputMode="numeric"
          validate={(value) =>
            parseYear(value.trim()) === null ? t('admin.match.yearInvalid') : null
          }
        />
        <RadioGroup
          label={t('admin.match.provider')}
          value={provider}
          onChange={(value) => setProvider(value === 'tvdb' ? 'tvdb' : 'tmdb')}
          options={[
            { value: 'tmdb', label: t('admin.item.facts.tmdb') },
            { value: 'tvdb', label: t('admin.item.facts.tvdb') },
          ]}
        />
        <RadioGroup
          label={t('admin.match.kind')}
          value={provider === 'tvdb' ? 'series' : searchKind}
          onChange={(value) => setSearchKind(value === 'series' ? 'series' : 'movie')}
          options={[
            { value: 'movie', label: t('kind.movie'), isDisabled: provider === 'tvdb' },
            { value: 'series', label: t('kind.series') },
          ]}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={results.isFetching}>
            {results.isFetching ? t('admin.match.searching') : t('admin.match.search')}
          </AriaButton>
        </div>
      </Form>

      {results.isError && <Notice tone="danger">{problemMessage(t, results.error)}</Notice>}
      {match.isError && <Notice tone="danger">{problemMessage(t, match.error)}</Notice>}
      {results.isSuccess && results.data.length === 0 && <Notice>{t('admin.match.none')}</Notice>}
      {results.isSuccess && results.data.length > 0 && (
        <section className={styles.candidates} aria-labelledby="match-results">
          <h3 id="match-results" className={styles.listTitle}>
            {t('admin.match.results')}
          </h3>
          <ul className={styles.candidateList}>
            {results.data.map((candidate) => (
              <li key={`${candidate.provider}-${candidate.providerId}`}>
                <CandidateCard
                  title={candidateTitle(candidate)}
                  subtitle={
                    candidate.originalTitle !== candidate.title
                      ? candidate.originalTitle
                      : undefined
                  }
                  overview={candidate.overview}
                  posterUrl={candidate.posterUrl}
                  aria-label={t('admin.match.choose', { title: candidateTitle(candidate) })}
                  isDisabled={match.isPending}
                  onPress={() => match.mutate(candidate)}
                />
              </li>
            ))}
          </ul>
        </section>
      )}
    </FormDialog>
  );
}
