import type { AdminPick } from '@slouchcat/api-client';
import type { FormErrors } from '@/api/errors';
import { useId, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Notice, Text } from '@/ui';
import { AriaButton, Form, RankHint, RankSelect } from '@/ui/aria';
import { WindowFields } from './WindowFields';
import styles from './Picks.module.css';

/** The unsaved note and time window of a pick, as the fields show them (S-17). */
export interface PickDraft {
  note: string;
  /** `datetime-local` values, '' for none. */
  from: string;
  to: string;
}

/**
 * The fold-out of a pick (PIC-3, mockup Pic-C): note and time window saved here (before: a
 * dialog), «Verwerfen» drops the draft; the place in the row as a choice (S-16, the others move
 * along); the target to read; «Entfernen» asks first (the page's confirmation).
 */
export function PickDetail({
  pick,
  draft,
  changed,
  saving,
  errors,
  place,
  count,
  onChange,
  onSave,
  onDiscard,
  onPlace,
  onRemove,
}: {
  pick: AdminPick;
  draft: PickDraft;
  changed: boolean;
  saving: boolean;
  /** The server's problems with the last save of this pick: per field and for the form. */
  errors: FormErrors | null;
  /** 0-based place in the whole row. */
  place: number;
  count: number;
  onChange: (pick: AdminPick, next: Partial<PickDraft>) => void;
  onSave: (pick: AdminPick) => void;
  onDiscard: (pick: AdminPick) => void;
  onPlace: (pick: AdminPick, position: number) => void;
  onRemove: (pick: AdminPick) => void;
}) {
  const { t } = useTranslation();
  const rankHint = useId();
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onSave(pick);
  };
  return (
    <div className={styles.detail}>
      <Form
        className={styles.editForm}
        aria-label={t('admin.picks.edit.label', { title: pick.title })}
        onSubmit={onSubmit}
        validationErrors={errors?.fields}
      >
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <div className={styles.fields}>
          <WindowFields
            note={draft.note}
            from={draft.from}
            to={draft.to}
            onNote={(note) => onChange(pick, { note })}
            onFrom={(from) => onChange(pick, { from })}
            onTo={(to) => onChange(pick, { to })}
          />
        </div>
        <div className={styles.actions}>
          <AriaButton
            type="submit"
            variant="primary"
            aria-label={t('admin.picks.edit.saveLabel', { title: pick.title })}
            isDisabled={!changed}
            isPending={saving}
          >
            {saving ? t('common.saving') : t('common.save')}
          </AriaButton>
          <AriaButton
            variant="ghost"
            aria-label={t('admin.picks.edit.discardLabel', { title: pick.title })}
            isDisabled={!changed || saving}
            onPress={() => onDiscard(pick)}
          >
            {t('admin.picks.edit.discard')}
          </AriaButton>
        </div>
      </Form>
      <div className={styles.side}>
        <div className={styles.rank}>
          <Text as="span" variant="caption" tone="secondary">
            {t('admin.picks.detail.place')}
          </Text>
          <RankSelect
            name={pick.title}
            position={place}
            count={count}
            describedBy={rankHint}
            onChange={(position) => onPlace(pick, position)}
          />
          <RankHint id={rankHint} />
        </div>
        <dl className={styles.facts}>
          <div>
            <dt>{t('admin.picks.detail.target')}</dt>
            <dd>{pick.profileName ?? t('admin.picks.allProfiles')}</dd>
          </div>
        </dl>
        <div className={styles.actions}>
          <AriaButton
            variant="danger"
            icon="trash"
            aria-label={t('admin.picks.remove.label', { title: pick.title })}
            onPress={() => onRemove(pick)}
          >
            {t('admin.picks.remove.action')}
          </AriaButton>
        </div>
      </div>
    </div>
  );
}
