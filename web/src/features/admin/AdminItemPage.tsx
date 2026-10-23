import type { AdminItem, Trailer, TrailerRequest } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formText } from '@/lib/form';
import { formatBytes, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, FactList, Notice, Page, Panel, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from '../shared/choices';
import { MatchDialog } from './MatchDialog';
import { adminItemQuery, maintenanceKey } from './queries';
import styles from './AdminItemPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/items/$itemId');

/** A bare YouTube video id; anything else goes to the server as a URL. */
const YOUTUBE_ID = /^[A-Za-z0-9_-]{8,16}$/;
const LANG_CODE = /^[a-z]{2,3}$/;
const FROM_METADATA = 'metadata';

function itemFacts(t: TFunction, item: AdminItem, language: string): [string, string][] {
  const none = t('admin.item.facts.none');
  const rows: [string, string][] = [
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
        ? `${item.episodeNumber}–${item.episodeEnd}`
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
      t('admin.item.facts.locked'),
      item.lockedFields.length > 0 ? item.lockedFields.join(', ') : none,
    ],
  );
  if (item.kind === 'series' || item.kind === 'season' || item.kind === 'movie') {
    rows.push([t('admin.item.facts.children'), t('admin.item.facts.childCounts', item.children)]);
  }
  return rows;
}

function RatingPanel({ item, onSaved }: { item: AdminItem; onSaved: () => Promise<void> }) {
  const { t } = useTranslation();
  const none = t('admin.item.facts.none');
  const [value, setValue] = useState(
    item.ownRating !== undefined ? String(item.ownRating) : FROM_METADATA,
  );
  const save = useMutation({
    mutationFn: () =>
      request(
        api.PUT('/admin/items/{id}/rating', {
          params: { path: { id: item.id } },
          body: { rating: value === FROM_METADATA ? null : (titleRatingOf(value) ?? null) },
        }),
      ),
    onSuccess: onSaved,
  });
  const fsk = (age: number | undefined) => (age !== undefined ? t('rating.fsk', { age }) : none);

  return (
    <Panel as="section" aria-labelledby="admin-item-rating">
      <h2 id="admin-item-rating" className={styles.panelTitle}>
        {t('admin.item.rating.title')}
      </h2>
      <FactList
        facts={[
          [t('admin.item.rating.own'), fsk(item.ownRating)],
          [t('admin.item.rating.effective'), fsk(item.effectiveRating)],
          [t('admin.item.rating.source'), item.ratingSource || none],
        ]}
      />
      <div className={styles.inline}>
        <RadioGroup
          label={t('admin.item.rating.label')}
          value={value}
          onChange={setValue}
          options={[
            { value: FROM_METADATA, label: t('admin.item.rating.fromMetadata') },
            ...ratingOptions(t),
          ]}
        />
        <AriaButton variant="primary" isPending={save.isPending} onPress={() => save.mutate()}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </AriaButton>
      </div>
      {save.isError && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
    </Panel>
  );
}

function TrailerForm({
  itemId,
  onClose,
  onAdded,
}: {
  itemId: string;
  onClose: () => void;
  onAdded: () => Promise<void>;
}) {
  const { t, i18n } = useTranslation();
  const [lang, setLang] = useState(i18n.language === 'en' ? 'en' : 'de');
  const add = useMutation({
    mutationFn: (body: TrailerRequest): Promise<Trailer> =>
      request(api.POST('/admin/items/{id}/trailers', { params: { path: { id: itemId } }, body })),
    onSuccess: async () => {
      await onAdded();
      onClose();
    },
  });
  const errors = add.error
    ? formErrors(t, add.error, { youtubeId: 'source', url: 'source', lang: 'langCode' })
    : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const source = formText(data, 'source');
    const code = lang === 'other' ? formText(data, 'langCode') : lang;
    add.mutate({
      lang: code,
      ...(YOUTUBE_ID.test(source) ? { youtubeId: source } : { url: source }),
    });
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <TextField
        name="source"
        label={t('admin.item.trailers.url')}
        description={t('admin.item.trailers.urlHint')}
        autoComplete="off"
        spellCheck="false"
        mono
        isRequired
        validate={(value) => (value.trim() ? null : t('errors.field.required'))}
      />
      <RadioGroup
        label={t('admin.item.trailers.lang')}
        value={lang}
        onChange={setLang}
        options={[
          { value: 'de', label: t('language.de') },
          { value: 'en', label: t('language.en') },
          { value: 'other', label: t('admin.item.trailers.langOther') },
        ]}
      />
      {lang === 'other' && (
        <TextField
          name="langCode"
          label={t('admin.item.trailers.langCode')}
          description={t('admin.item.trailers.langCodeHint')}
          autoComplete="off"
          isRequired
          validate={(value) => (LANG_CODE.test(value.trim()) ? null : t('errors.field.invalid'))}
        />
      )}
      <div className={styles.dialogActions}>
        <AriaButton variant="ghost" onPress={onClose}>
          {t('common.cancel')}
        </AriaButton>
        <AriaButton type="submit" variant="primary" isPending={add.isPending}>
          {add.isPending ? t('admin.item.trailers.submitting') : t('admin.item.trailers.submit')}
        </AriaButton>
      </div>
    </Form>
  );
}

function TrailersPanel({ item, onChanged }: { item: AdminItem; onChanged: () => Promise<void> }) {
  const { t } = useTranslation();
  const [adding, setAdding] = useState(false);
  const remove = useMutation({
    mutationFn: (trailerId: number) =>
      request(
        api.DELETE('/admin/items/{id}/trailers/{trailerId}', {
          params: { path: { id: item.id, trailerId } },
        }),
      ),
    onSuccess: onChanged,
  });

  return (
    <Panel as="section" aria-labelledby="admin-item-trailers">
      <div className={styles.panelHead}>
        <h2 id="admin-item-trailers" className={styles.panelTitle}>
          {t('admin.item.trailers.title')}
        </h2>
        <Chip tone="info">{t(`admin.item.trailers.state.${item.trailerState}`)}</Chip>
      </div>
      {remove.isError && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
      {item.trailers.length === 0 ? (
        <Text tone="secondary">{t('admin.item.trailers.empty')}</Text>
      ) : (
        <ul className={styles.list} aria-label={t('admin.item.trailers.title')}>
          {item.trailers.map((trailer) => {
            const labelId = `trailer-${trailer.id}`;
            return (
              <li key={trailer.id} className={styles.row}>
                <div className={styles.rowMain}>
                  <span id={labelId} className={styles.rowHead}>
                    <Chip tone="outline">{trailer.lang.toUpperCase()}</Chip>
                    <code className={styles.path}>{trailer.youtubeId}</code>
                    {trailer.title && <span>{trailer.title}</span>}
                  </span>
                  <span className={styles.rowHead}>
                    <span className={styles.muted}>
                      {t(`admin.item.trailers.source.${trailer.source}`)}
                    </span>
                    {trailer.official && (
                      <Chip tone="info">{t('admin.item.trailers.official')}</Chip>
                    )}
                  </span>
                </div>
                <ConfirmDialog
                  trigger={
                    <AriaButton variant="ghost" aria-describedby={labelId}>
                      {t('admin.item.trailers.delete.action')}
                    </AriaButton>
                  }
                  title={t('admin.item.trailers.delete.title')}
                  body={t('admin.item.trailers.delete.body')}
                  confirmLabel={t('admin.item.trailers.delete.confirm')}
                  cancelLabel={t('common.cancel')}
                  tone="danger"
                  onConfirm={() => remove.mutate(trailer.id)}
                />
              </li>
            );
          })}
        </ul>
      )}
      <div>
        <AriaButton icon="plus" onPress={() => setAdding(true)}>
          {t('admin.item.trailers.add')}
        </AriaButton>
      </div>
      {adding && (
        <FormDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) setAdding(false);
          }}
          title={t('admin.item.trailers.addTitle')}
        >
          <TrailerForm itemId={item.id} onClose={() => setAdding(false)} onAdded={onChanged} />
        </FormDialog>
      )}
    </Panel>
  );
}

function FilesPanel({ item }: { item: AdminItem }) {
  const { t, i18n } = useTranslation();
  return (
    <Panel as="section" aria-labelledby="admin-item-files">
      <h2 id="admin-item-files" className={styles.panelTitle}>
        {t('admin.item.files.title')}
      </h2>
      {item.mediaFiles.length === 0 ? (
        <Text tone="secondary">{t('admin.item.files.none')}</Text>
      ) : (
        <ul className={styles.list} aria-label={t('admin.item.files.title')}>
          {item.mediaFiles.map((file) => (
            <li key={file.id} className={styles.fileRow}>
              <code className={styles.path}>{file.path}</code>
              <span className={styles.rowHead}>
                {file.versionLabel && <Chip tone="outline">{file.versionLabel}</Chip>}
                {!file.available && (
                  <Chip tone="warning" icon="warning">
                    {t('item.file.unavailable')}
                  </Chip>
                )}
                <span className={styles.muted}>{formatBytes(file.sizeBytes, i18n.language)}</span>
                {file.probedAt && (
                  <span className={styles.muted}>
                    {t('admin.item.files.probedAt', {
                      date: formatDateTime(file.probedAt, i18n.language),
                    })}
                  </span>
                )}
              </span>
              {file.probeError && (
                <Notice tone="warning" title={t('admin.item.files.probeError')}>
                  {file.probeError}
                </Notice>
              )}
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}

function AdminItemView({ itemId }: { itemId: string }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const item = useQuery(adminItemQuery(itemId));
  useDocumentTitle(item.data?.title ?? t('admin.item.title'));
  const [matching, setMatching] = useState(false);
  const [notice, setNotice] = useState<string | null>(null);

  const reload = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: adminItemQuery(itemId).queryKey }),
      queryClient.invalidateQueries({ queryKey: maintenanceKey }),
      queryClient.invalidateQueries({ queryKey: ['item', itemId] }),
    ]);
  };
  const adopt = async (next: AdminItem, message: string) => {
    queryClient.setQueryData(adminItemQuery(itemId).queryKey, next);
    setNotice(message);
    await reload();
  };
  const refresh = useMutation({
    mutationFn: (): Promise<AdminItem> =>
      request(api.POST('/admin/items/{id}/refresh', { params: { path: { id: itemId } } })),
    onSuccess: (next) => adopt(next, t('admin.item.refreshed')),
  });

  if (item.isPending) return <Skeleton shape="block" />;
  if (item.isError) {
    return (
      <Notice tone={isApiError(item.error, 404) ? 'info' : 'danger'}>
        {isApiError(item.error, 404) ? t('admin.item.notFound') : problemMessage(t, item.error)}
      </Notice>
    );
  }

  const data = item.data;
  const matchable = data.kind === 'movie' || data.kind === 'series';
  return (
    <div className={styles.layout}>
      <div className={styles.header}>
        <span className={styles.rowHead}>
          <Chip tone="info">{t(`kind.${data.kind}`)}</Chip>
          {data.year !== undefined && <span className={styles.muted}>{data.year}</span>}
        </span>
        <Text as="h1" variant="display">
          {data.title}
        </Text>
        <div className={styles.actions}>
          {matchable && (
            <AriaButton variant="primary" onPress={() => setMatching(true)}>
              {t('admin.maintenance.action.match')}
            </AriaButton>
          )}
          <AriaButton icon="refresh" isPending={refresh.isPending} onPress={() => refresh.mutate()}>
            {t('admin.maintenance.action.refresh')}
          </AriaButton>
          <ButtonLink href={paths.item(data.id)} variant="ghost" icon="chevronRight">
            {t('admin.item.view')}
          </ButtonLink>
        </div>
        {refresh.isError && <Notice tone="danger">{problemMessage(t, refresh.error)}</Notice>}
        {notice && <Notice>{notice}</Notice>}
      </div>

      <Panel as="section" aria-labelledby="admin-item-facts">
        <h2 id="admin-item-facts" className={styles.panelTitle}>
          {t('admin.item.facts.metadata')}
        </h2>
        <FactList facts={itemFacts(t, data, i18n.language)} />
      </Panel>
      {/* Remount on a new rating so the choice starts from the stored value; the confirmation
          lives here, so it survives that remount. */}
      <RatingPanel
        key={`${data.ownRating ?? 'none'}`}
        item={data}
        onSaved={async () => {
          setNotice(t('admin.item.rating.saved'));
          await reload();
        }}
      />
      <FilesPanel item={data} />
      <TrailersPanel item={data} onChanged={reload} />

      {matching && (
        <MatchDialog
          itemId={data.id}
          title={data.title}
          year={data.year}
          kind={data.kind === 'series' ? 'series' : 'movie'}
          onClose={() => setMatching(false)}
          onMatched={(next) => void adopt(next, t('admin.maintenance.done.match'))}
        />
      )}
    </div>
  );
}

/** Administration view of one title (B-6, B-8, B-9): state, ids, rating, files and trailers. */
export function AdminItemPage() {
  const { t } = useTranslation();
  const { itemId } = route.useParams();
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  return (
    <Page>
      {allowed ? (
        <AdminItemView key={itemId} itemId={itemId} />
      ) : (
        <Notice title={t('admin.item.notAllowed.title')}>{t('admin.item.notAllowed.body')}</Notice>
      )}
    </Page>
  );
}
