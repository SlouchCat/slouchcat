import type { AdminCollection, CollectionRequest } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, TextField } from '@/ui/aria';
import { createCollection, renameCollection } from '../queries';
import styles from './Collections.module.css';

/** Longest collection name the server takes (`CollectionRequest.name`). */
const MAX_NAME = 120;

/**
 * «Sammlung anlegen» and «Umbenennen» (SAM-1, SAM-3): one name field. A new collection starts
 * empty; its titles are added in its row.
 */
export function NameDialog({
  collection,
  onClose,
  onSaved,
}: {
  /** The collection to rename; absent for a new one. */
  collection?: AdminCollection;
  onClose: () => void;
  onSaved: (saved: AdminCollection) => void;
}) {
  const { t } = useTranslation();
  const save = useMutation({
    mutationFn: (body: CollectionRequest) =>
      collection ? renameCollection(collection.id, body) : createCollection(body),
    onSuccess: onSaved,
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate({ name: formText(new FormData(event.currentTarget), 'name') });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={
        collection
          ? t('admin.collections.rename.title', { name: collection.name })
          : t('admin.collections.create.title')
      }
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('admin.collections.form.name')}
          defaultValue={collection?.name}
          maxLength={MAX_NAME}
          autoComplete="off"
          isRequired
          // A server error sits on this field until the name changes (as on the podcasts page).
          onChange={() => {
            if (save.error) save.reset();
          }}
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            icon={collection ? undefined : 'plus'}
            isPending={save.isPending}
          >
            {save.isPending
              ? t('common.saving')
              : collection
                ? t('common.save')
                : t('admin.collections.create.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
