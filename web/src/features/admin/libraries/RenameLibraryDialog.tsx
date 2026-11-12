import type { Library } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, TextField } from '@/ui/aria';
import styles from './Libraries.module.css';

/** Longest library name the server takes (`LibraryUpdate.name`). */
const MAX_NAME = 80;

interface RenameLibraryDialogProps {
  library: Library;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onSaved: (library: Library) => void;
}

/**
 * «Umbenennen …» of a library: one name field, filled with the name and selected, so typing
 * replaces it. The server checks the name (422 `required`, `too_long`); its answer sits at the
 * field. The page opens it from the row's menu and mounts it afresh each time, like the
 * collection's NameDialog.
 */
export function RenameLibraryDialog({
  library,
  isOpen,
  onOpenChange,
  onSaved,
}: RenameLibraryDialogProps) {
  const { t } = useTranslation();
  const save = useMutation({
    mutationFn: (name: string): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}', {
          params: { path: { id: library.id } },
          body: { name },
        }),
      ),
    onSuccess: (updated) => {
      onOpenChange(false);
      onSaved(updated);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (save.isPending) return;
    save.mutate(formText(new FormData(event.currentTarget), 'name'));
  };
  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title={t('admin.libraries.rename.title', { name: library.name })}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('admin.libraries.rename.label')}
          defaultValue={library.name}
          maxLength={MAX_NAME}
          autoComplete="off"
          isRequired
          selectOnFocus
          // A server error sits on this field until the name changes (as in NameDialog).
          onChange={() => {
            if (save.error) save.reset();
          }}
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
