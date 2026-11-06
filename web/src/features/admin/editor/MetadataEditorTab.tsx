import type { AdminItem } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useId, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { fieldMessage, formErrors } from '@/api/errors';
import { Chip, Notice, SelectField, Text } from '@/ui';
import { AriaButton, Form, LockToggle, TextField } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from '../../shared/choices';
import { providerName } from '../item/facts';
import { FROM_METADATA, hasOriginalLanguage, originalLanguageOptions } from '../item/MetadataTab';
import { adminItemQuery, metadataSettingsQuery } from '../queries';
import {
  canToggleLock,
  editField,
  emptyDraft,
  lockedOnServer,
  saveOf,
  serverValues,
  shownLocked,
  toggleLock,
  valueOf,
  withGenre,
  type AdminItemMetadata,
  type EditorDraft,
  type EditorField,
  type EditorValues,
} from './editorModel';
import styles from './MetadataEditor.module.css';

/** The genre suggestions of the server's metadata language (listAdminGenres). */
const genresQuery = {
  queryKey: ['admin', 'genres'],
  queryFn: ({ signal }: { signal: AbortSignal }) => request(api.GET('/admin/genres', { signal })),
};

/** Where a field's value came from: «aus TMDB», «von Hand» or nothing. */
function sourceOf(
  t: TFunction,
  item: AdminItem,
  meta: AdminItemMetadata,
  field: EditorField,
): string | undefined {
  let provider: string | undefined;
  if (field === 'rating') provider = item.ownRating === undefined ? item.ratingSource : undefined;
  else if (field === 'language')
    provider = item.fieldSources.find((s) => s.field === 'original_language')?.provider;
  else provider = meta.sources.find((s) => s.field === field)?.provider;
  if (lockedOnServer(item, meta, field)) return t('admin.editor.metadata.manual');
  return provider
    ? t('admin.editor.metadata.source', { provider: providerName(t, provider) })
    : undefined;
}

interface MetadataEditorTabProps {
  item: AdminItem;
  meta: AdminItemMetadata;
  /** The reader's changes, held by the dialog so a tab switch keeps them. */
  draft: EditorDraft;
  onDraftChange: (draft: EditorDraft) => void;
  onChanged: (next: AdminItem, message: string) => void;
}

/**
 * The tab «Metadaten» of the editor (concept 15 7.4): every field with its origin and its lock.
 * A change locks the field; the lock lifts it again. «Speichern» sends all changes at once
 * (updateItemMetadata, then the rating and the original language with their own endpoints).
 * Texts belong to the server's metadata language (F-10).
 */
export function MetadataEditorTab({
  item,
  meta,
  draft,
  onDraftChange,
  onChanged,
}: MetadataEditorTabProps) {
  const { t, i18n } = useTranslation();
  const server = useMemo(() => serverValues(item, meta), [item, meta]);
  const genres = useQuery(genresQuery);
  const languages = useQuery({ ...metadataSettingsQuery, enabled: hasOriginalLanguage(item) });
  const names = useMemo(
    () => new Intl.DisplayNames([i18n.language], { type: 'language' }),
    [i18n.language],
  );
  const queryClient = useQueryClient();
  const genresId = useId();
  const genresErrorId = useId();
  const dateInput = useRef<HTMLInputElement>(null);
  const [genre, setGenre] = useState('');
  const [invalid, setInvalid] = useState<Record<string, string>>({});
  const [unchanged, setUnchanged] = useState(false);

  const save = useMutation({
    mutationFn: async (plan: ReturnType<typeof saveOf>): Promise<AdminItem> => {
      const path = { params: { path: { id: item.id } } };
      if (plan.metadata) {
        await request(api.PATCH('/admin/items/{id}/metadata', { ...path, body: plan.metadata }));
      }
      if (plan.rating !== undefined) {
        const rating = plan.rating === FROM_METADATA ? null : (titleRatingOf(plan.rating) ?? null);
        await request(api.PUT('/admin/items/{id}/rating', { ...path, body: { rating } }));
      }
      if (plan.language !== undefined) {
        const language = plan.language === FROM_METADATA ? null : plan.language;
        await request(
          api.PUT('/admin/items/{id}/original-language', { ...path, body: { language } }),
        );
      }
      return request(api.GET('/admin/items/{id}', path));
    },
    onSuccess: (next) => {
      onDraftChange(emptyDraft);
      onChanged(next, t('admin.editor.metadata.saved'));
    },
    // A step may have gone through before the one that failed (the texts, then the rating): the
    // dialog reads the title again and keeps the draft, so «Speichern» can try the rest again.
    onError: () =>
      void queryClient.invalidateQueries({ queryKey: adminItemQuery(item.id).queryKey }),
  });
  const errors: { fields: Record<string, string>; form: string | null } = save.isError
    ? formErrors(t, save.error)
    : { fields: {}, form: null };
  // Genres and the rating have no react-aria field the form could hand their message to.
  const genresError = errors.fields.genres;
  const ratingError = errors.fields.rating;

  const set = <F extends EditorField>(field: F, value: EditorValues[F]) => {
    setUnchanged(false);
    onDraftChange(editField(server, draft, field, value));
  };
  const value = <F extends EditorField>(field: F) => valueOf(server, draft, field);
  const fieldLabel = (field: EditorField) => t(`admin.editor.metadata.field.${field}`);

  /** The line under a field: where its value came from, and its lock. */
  const meta_ = (field: EditorField): ReactNode => {
    const source = sourceOf(t, item, meta, field);
    return (
      <div className={styles.fieldMeta}>
        {source && <span className={styles.muted}>{source}</span>}
        <LockToggle
          label={t('admin.editor.metadata.lock.label', { field: fieldLabel(field) })}
          isLocked={shownLocked(item, meta, server, draft, field)}
          isDisabled={!canToggleLock(item, meta, server, draft, field)}
          lockedText={t('admin.editor.metadata.lock.locked')}
          freeText={t('admin.editor.metadata.lock.free')}
          onChange={() => {
            setUnchanged(false);
            onDraftChange(toggleLock(item, meta, server, draft, field));
          }}
        />
      </div>
    );
  };

  const text = (field: 'title' | 'originalTitle' | 'sortTitle' | 'tagline', full = false) => (
    <div className={full ? styles.full : undefined}>
      <TextField
        name={field}
        label={fieldLabel(field)}
        value={value(field)}
        // A free sort title follows the title: the field stays empty and shows it.
        placeholder={field === 'sortTitle' ? value('title') : undefined}
        onChange={(next) => set(field, next)}
      />
      {meta_(field)}
    </div>
  );

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const plan = saveOf(draft);
    const local: Record<string, string> = {};
    for (const [field, code] of Object.entries(plan.invalid)) local[field] = fieldMessage(t, code);
    // A half-typed date reads as empty; sending it would clear the date.
    if (dateInput.current?.validity.badInput) local.releaseDate = fieldMessage(t, 'invalid_value');
    setInvalid(local);
    if (Object.keys(local).length > 0) return;
    if (!plan.metadata && plan.rating === undefined && plan.language === undefined) {
      setUnchanged(true);
      return;
    }
    save.mutate(plan);
  };

  const titleKind = item.kind === 'movie' || item.kind === 'series';
  const chosen = value('genres');
  const suggestions = (genres.data?.genres ?? []).filter(
    (name) => !chosen.some((g) => g.toLowerCase() === name.toLowerCase()),
  );
  const addGenre = () => {
    set('genres', withGenre(chosen, genre));
    setGenre('');
  };
  const dirty = Object.keys(draft.changed).length > 0 || draft.unlock.length > 0;

  return (
    <Form
      className={styles.stack}
      onSubmit={onSubmit}
      validationErrors={{ ...errors.fields, ...invalid }}
    >
      <Text tone="secondary" variant="caption">
        {t('admin.editor.metadata.intro')}{' '}
        {t('admin.editor.metadata.lang', { language: names.of(meta.lang) ?? meta.lang })}
      </Text>
      <div className={styles.fields}>
        {text('title', true)}
        {titleKind && text('originalTitle')}
        {text('sortTitle')}
        <div>
          <TextField
            name="year"
            label={fieldLabel('year')}
            inputMode="numeric"
            value={value('year')}
            onChange={(next) => set('year', next)}
          />
          {meta_('year')}
        </div>
        <div>
          <TextField
            name="releaseDate"
            label={fieldLabel('releaseDate')}
            type="date"
            inputRef={dateInput}
            value={value('releaseDate')}
            onChange={(next) => set('releaseDate', next)}
          />
          {meta_('releaseDate')}
        </div>
        {item.kind === 'movie' && text('tagline', true)}
        <div className={styles.full}>
          <TextField
            name="overview"
            label={fieldLabel('overview')}
            rows={5}
            value={value('overview')}
            onChange={(next) => set('overview', next)}
          />
          {meta_('overview')}
        </div>
        {titleKind && (
          <div className={styles.full}>
            <div
              role="group"
              aria-labelledby={genresId}
              aria-describedby={genresError ? genresErrorId : undefined}
              className={styles.stack}
            >
              <span id={genresId} className={styles.label}>
                {fieldLabel('genres')}
              </span>
              <ul className={styles.genres}>
                {chosen.length === 0 && (
                  <li className={styles.muted}>{t('admin.editor.metadata.genresNone')}</li>
                )}
                {chosen.map((name) => (
                  <li key={name} className={styles.genre}>
                    <Chip>{name}</Chip>
                    <AriaButton
                      variant="ghost"
                      icon="close"
                      iconOnly
                      aria-label={t('admin.editor.metadata.genreRemove', { genre: name })}
                      onPress={() =>
                        set(
                          'genres',
                          chosen.filter((g) => g !== name),
                        )
                      }
                    />
                  </li>
                ))}
              </ul>
              <div className={styles.inline}>
                <TextField
                  name="genre"
                  label={t('admin.editor.metadata.genreAdd')}
                  value={genre}
                  suggestions={suggestions}
                  onChange={setGenre}
                  onKeyDown={(event) => {
                    // Enter adds the genre instead of saving the form.
                    if (event.key !== 'Enter') return;
                    event.preventDefault();
                    addGenre();
                  }}
                />
                <AriaButton icon="plus" isDisabled={!genre.trim()} onPress={addGenre}>
                  {t('admin.editor.metadata.genreAddAction')}
                </AriaButton>
              </div>
              {genresError && (
                <p id={genresErrorId} className={styles.fieldError}>
                  {genresError}
                </p>
              )}
            </div>
            {meta_('genres')}
          </div>
        )}
        <div>
          <SelectField
            label={fieldLabel('rating')}
            value={value('rating')}
            onChange={(next) => set('rating', next)}
            options={[
              { value: FROM_METADATA, label: t('admin.item.rating.fromMetadata') },
              ...ratingOptions(t).map(({ value: age, label }) => ({ value: age, label })),
            ]}
          />
          {ratingError && <p className={styles.fieldError}>{ratingError}</p>}
          {meta_('rating')}
        </div>
        {hasOriginalLanguage(item) && (
          <div>
            <SelectField
              label={fieldLabel('language')}
              value={value('language')}
              onChange={(next) => set('language', next)}
              options={originalLanguageOptions(
                t,
                names,
                i18n.language,
                languages.data?.knownLanguages ?? [],
                value('language'),
              )}
            />
            {meta_('language')}
          </div>
        )}
      </div>
      {errors.form && <Notice tone="danger">{errors.form}</Notice>}
      {unchanged && <Notice>{t('admin.editor.metadata.unchanged')}</Notice>}
      <div className={styles.actions}>
        <AriaButton type="submit" variant="primary" isPending={save.isPending}>
          {save.isPending ? t('common.saving') : t('common.save')}
        </AriaButton>
        <AriaButton
          variant="ghost"
          isDisabled={!dirty || save.isPending}
          onPress={() => {
            setInvalid({});
            setUnchanged(false);
            save.reset();
            onDraftChange(emptyDraft);
          }}
        >
          {t('admin.editor.metadata.discard')}
        </AriaButton>
      </div>
    </Form>
  );
}
