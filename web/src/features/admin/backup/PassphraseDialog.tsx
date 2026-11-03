import type { Backup, RestoreResult } from '@slouchcat/api-client';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, TextField } from '@/ui/aria';
import { restoreBackup, type Confirmation } from './api';
import styles from './Backup.module.css';

/**
 * «Wiederherstellen» of an encrypted backup of the list (SIC-3): the warning and its passphrase,
 * then the page's confirmation. The passphrase lives only in this dialog, never in a draft or the
 * mutation cache; it goes when the dialog closes or the restore is accepted.
 */
export function PassphraseDialog({
  backup,
  ask,
  onClose,
  onAccepted,
}: {
  backup: Backup;
  ask: (confirmation: Confirmation) => void;
  onClose: () => void;
  onAccepted: (result: RestoreResult) => void;
}) {
  const { t } = useTranslation();
  const [passphrase, setPassphrase] = useState('');
  const restore = useTransientMutation(
    (): Promise<RestoreResult> => restoreBackup(backup.name, passphrase),
    onAccepted,
  );
  const errors = restore.error ? formErrors(t, restore.error) : null;
  const ready = passphrase !== '' && !restore.isPending;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!ready) return;
    ask({
      title: t('admin.backup.restore.confirmTitle'),
      body: t('admin.backup.restore.confirmBodyNamed', { name: backup.name }),
      confirmLabel: t('admin.backup.restore.confirm'),
      defaultFocus: 'cancel',
      onConfirm: () => void restore.run(undefined),
    });
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.backup.restore.titleNamed', { name: backup.name })}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <Notice tone="warning" live={false}>
          {t('admin.backup.restore.warning')}
        </Notice>
        <TextField
          name="passphrase"
          type="password"
          label={t('admin.backup.restore.passphrase')}
          description={t('admin.backup.restore.passphraseHint')}
          autoComplete="off"
          isRequired
          value={passphrase}
          onChange={setPassphrase}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="danger"
            isDisabled={passphrase === ''}
            isPending={restore.isPending}
          >
            {restore.isPending
              ? t('admin.backup.restore.running')
              : t('admin.backup.restore.action')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
