import type { Profile } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, PinField } from '@/ui/aria';
import styles from './ProfileForm.module.css';

interface PinSetDialogProps {
  profile: Profile;
  onClose: () => void;
}

/** Sets or replaces the PIN of a profile (K-2). */
export function PinSetDialog({ profile, onClose }: PinSetDialogProps) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [pin, setPin] = useState('');
  const save = useTransientMutation(
    (value: string) =>
      request(
        api.PUT('/profiles/{id}/pin', {
          params: { path: { id: profile.id } },
          body: { pin: value },
        }),
      ),
    async () => {
      await queryClient.invalidateQueries({ queryKey: ['profiles'] });
      await queryClient.invalidateQueries({ queryKey: ['auth'] });
      onClose();
    },
  );
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void save.run(pin);
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('profiles.settings.pin.title', { name: profile.name })}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <PinField
          name="pin"
          label={t('profiles.settings.pin.label')}
          description={t('profiles.settings.pin.hint')}
          invalidMessage={t('errors.field.pin_format')}
          value={pin}
          onChange={setPin}
          autoFocus
        />
        <div className={styles.actions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {save.isPending ? t('profiles.settings.pin.saving') : t('profiles.settings.pin.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
