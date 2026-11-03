import type { AdminPick, PickRequest } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { manageProfilesQuery } from '@/features/profiles/queries';
import { Notice } from '@/ui';
import { AriaButton, CheckboxGroup, Form, FormDialog } from '@/ui/aria';
import { fromLocalInput, titleMeta, TitleSearch } from '../CurationParts';
import { adminItemQuery, createPicks } from '../queries';
import { WindowFields } from './WindowFields';
import styles from './Picks.module.css';

/** The value of the target group that means "every profile" (a global pick). */
const ALL = 'all';

/** A title chosen for a new pick: from the search, or the one `?add=` names. */
interface Choice {
  id: string;
  title: string;
  kind: AdminPick['kind'];
  year?: number;
}

/**
 * «Hinzufügen» (PIC-1): a title from the search (or the one of `?add=`), for every profile or for
 * single ones, with an optional note and time window. New picks go to the end of the row.
 */
export function AddPickDialog({
  presetId,
  onClose,
  onAdded,
}: {
  presetId?: string;
  onClose: () => void;
  onAdded: (title: string) => void;
}) {
  const { t } = useTranslation();
  const profiles = useQuery(manageProfilesQuery);
  // The title the link named, from the admin view: no profile's visibility hides it here.
  const preset = useQuery({ ...adminItemQuery(presetId ?? ''), enabled: Boolean(presetId) });
  const [picked, setPicked] = useState<Choice | null>(null);
  const chosen: Choice | null =
    picked ??
    (preset.data
      ? {
          id: preset.data.id,
          title: preset.data.title,
          kind: preset.data.kind,
          year: preset.data.year,
        }
      : null);
  const [missingTitle, setMissingTitle] = useState(false);
  const [targets, setTargets] = useState<string[]>([ALL]);
  const [note, setNote] = useState('');
  const [from, setFrom] = useState('');
  const [to, setTo] = useState('');
  const create = useMutation({
    mutationFn: (body: PickRequest) => createPicks(body),
    onSuccess: () => onAdded(chosen?.title ?? ''),
  });
  const errors = create.error ? formErrors(t, create.error) : null;

  // "Alle Profile" and single profiles exclude each other: the last choice wins.
  const onTargets = (next: string[]) => {
    if (next.includes(ALL) && !targets.includes(ALL)) setTargets([ALL]);
    else setTargets(next.length > 1 ? next.filter((value) => value !== ALL) : next);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!chosen) {
      setMissingTitle(true);
      return;
    }
    const text = note.trim();
    create.mutate({
      itemId: chosen.id,
      ...(targets.includes(ALL) ? {} : { profileIds: targets.map(Number) }),
      ...(text ? { note: text } : {}),
      ...(fromLocalInput(from) ? { startsAt: fromLocalInput(from) } : {}),
      ...(fromLocalInput(to) ? { endsAt: fromLocalInput(to) } : {}),
    });
  };
  const itemError = errors?.fields.itemId;

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.picks.add.title')}
      description={t('admin.picks.add.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TitleSearch
          label={t('admin.picks.search.label')}
          hint={t('admin.picks.search.hint')}
          emptyText={t('admin.picks.search.empty')}
          resultsLabel={t('admin.picks.search.results')}
          actionText={t('admin.picks.search.select')}
          actionLabel={(hit) => t('admin.picks.search.selectLabel', { title: hit.title })}
          onPick={(hit) => {
            setPicked(hit);
            setMissingTitle(false);
            if (create.error) create.reset();
          }}
          exclude={chosen ? new Set([chosen.id]) : undefined}
        />
        <div aria-live="polite">
          {chosen ? (
            <p className={styles.chosen}>
              {t('admin.picks.search.chosen', { title: chosen.title })}
              <span className={styles.muted}>
                {t('format.separator')}
                {titleMeta(t, chosen)}
              </span>
            </p>
          ) : presetId && preset.isPending ? (
            <p className={styles.muted}>{t('common.loading')}</p>
          ) : (
            <p className={styles.muted}>{t('admin.picks.search.none')}</p>
          )}
          {presetId && preset.isError && !picked && (
            <Notice tone="warning">{t('admin.picks.add.presetMissing')}</Notice>
          )}
          {missingTitle && !chosen && (
            <Notice tone="danger">{t('admin.picks.add.titleRequired')}</Notice>
          )}
          {itemError && <Notice tone="danger">{itemError}</Notice>}
        </div>
        <CheckboxGroup
          name="profileIds"
          label={t('admin.picks.targets.label')}
          description={t('admin.picks.targets.description')}
          value={targets}
          onChange={onTargets}
          validate={(value) => (value.length === 0 ? t('admin.picks.targets.required') : null)}
          options={[
            { value: ALL, label: t('admin.picks.targets.all') },
            ...(profiles.data ?? []).map((profile) => ({
              value: String(profile.id),
              label: profile.name,
            })),
          ]}
        />
        <WindowFields
          note={note}
          from={from}
          to={to}
          onNote={setNote}
          onFrom={setFrom}
          onTo={setTo}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending ? t('admin.picks.add.submitting') : t('admin.picks.add.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
