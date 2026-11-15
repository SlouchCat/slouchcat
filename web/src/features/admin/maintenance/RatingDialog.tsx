import type { Rating } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useRatingSystem } from '@/app/session';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, RadioGroup } from '@/ui/aria';
import { ratingLabel, ratingOptions, titleRatingOf } from '../../shared/choices';
import type { Outcome } from './shared';
import styles from './Maintenance.module.css';

/** «Freigabe zuweisen» for the selected titles (PFL-3). */
export function RatingDialog({
  itemIds,
  onClose,
  onDone,
}: {
  itemIds: string[];
  onClose: () => void;
  onDone: (outcome: Outcome) => void;
}) {
  const { t } = useTranslation();
  const ratingSystem = useRatingSystem();
  // No preselection: an unrated title counts as FSK 18, and a wrong default (FSK 0) would open it
  // to every profile with two key presses.
  const [rating, setRating] = useState('');
  const chosen = titleRatingOf(rating);
  const assign = useMutation({
    mutationFn: (value: Rating) =>
      request(api.POST('/admin/items/rating', { body: { itemIds, rating: value } })),
    onSuccess: (result) => {
      onClose();
      onDone({ tone: 'info', text: t('admin.maintenance.done.rating', { count: result.updated }) });
    },
  });
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (chosen !== undefined && !assign.isPending) assign.mutate(chosen);
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.maintenance.rating.title')}
      description={t('admin.maintenance.rating.description', { count: itemIds.length })}
    >
      <Form className={styles.form} onSubmit={onSubmit}>
        {assign.isError && <Notice tone="danger">{problemMessage(t, assign.error)}</Notice>}
        <RadioGroup
          name="rating"
          label={t('admin.maintenance.rating.label')}
          value={rating}
          onChange={setRating}
          options={ratingOptions(t, ratingSystem)}
          isRequired
        />
        <p className={styles.muted} aria-live="polite">
          {chosen !== undefined &&
            t('admin.maintenance.rating.summary', {
              count: itemIds.length,
              rating: ratingLabel(t, chosen, ratingSystem),
            })}
        </p>
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            isDisabled={chosen === undefined}
            isPending={assign.isPending}
          >
            {assign.isPending
              ? t('admin.maintenance.rating.submitting')
              : t('admin.maintenance.rating.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
