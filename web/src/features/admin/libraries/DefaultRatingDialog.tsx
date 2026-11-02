import type { Library, Rating } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, RadioGroup } from '@/ui/aria';
import { ratingOptions, titleRatingOf } from '../../shared/choices';
import styles from './Libraries.module.css';

interface DefaultRatingDialogProps {
  library: Library;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  onSaved: (library: Library) => void;
}

/**
 * «Standardstufe ändern» of an audio library (AN-2, BIB-4): every title whose rating comes from
 * the library follows; titles rated by hand keep theirs (the server decides). The page opens it
 * from the row's menu and mounts it afresh each time, so it starts at the library's value.
 */
export function DefaultRatingDialog({
  library,
  isOpen,
  onOpenChange,
  onSaved,
}: DefaultRatingDialogProps) {
  const { t } = useTranslation();
  const [rating, setRating] = useState(String(library.defaultRating ?? 0));
  const save = useMutation({
    mutationFn: (defaultRating: Rating): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}', {
          params: { path: { id: library.id } },
          body: { defaultRating },
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
    const value = titleRatingOf(rating);
    if (value !== undefined) save.mutate(value);
  };
  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title={t('admin.libraries.changeRating.title', { name: library.name })}
      description={t('admin.libraries.changeRating.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <RadioGroup
          name="defaultRating"
          label={t('library.defaultRating')}
          value={rating}
          onChange={setRating}
          options={ratingOptions(t)}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {t('admin.libraries.changeRating.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
