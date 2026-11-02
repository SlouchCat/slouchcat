import type { RestoreResult } from '@slouchcat/api-client';
import { useId, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice } from '@/ui';
import { AriaButton, Form, TextField } from '@/ui/aria';
import { restoreUpload, type Confirmation } from './api';
import styles from './Backup.module.css';

/**
 * The tab «Aus Datei wiederherstellen» (SIC-2): the warning, the archive, the passphrase of an
 * encrypted one, and the page's confirmation before the upload. File and passphrase live only
 * here, never in a draft or the mutation cache; they go with the tab.
 */
export function UploadTab({
  ask,
  onAccepted,
}: {
  ask: (confirmation: Confirmation) => void;
  onAccepted: (result: RestoreResult) => void;
}) {
  const { t } = useTranslation();
  const fileId = useId();
  const [file, setFile] = useState<File | null>(null);
  const [passphrase, setPassphrase] = useState('');
  const restore = useTransientMutation(
    (upload: File): Promise<RestoreResult> => restoreUpload(upload, passphrase),
    onAccepted,
  );
  const errors = restore.error ? formErrors(t, restore.error) : null;

  const askRestore = () => {
    if (!file || restore.isPending) return;
    ask({
      title: t('admin.backup.restore.confirmTitle'),
      body: t('admin.backup.restore.confirmBody'),
      confirmLabel: t('admin.backup.restore.confirm'),
      onConfirm: () => void restore.run(file),
    });
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    askRestore();
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <Notice tone="warning" live={false}>
        {t('admin.backup.restore.warning')}
      </Notice>
      <div className={styles.field}>
        <label htmlFor={fileId} className={styles.label}>
          {t('admin.backup.restore.file')}
        </label>
        <input
          id={fileId}
          name="file"
          type="file"
          accept=".gz,.age"
          aria-describedby={`${fileId}-hint`}
          className={styles.fileInput}
          onChange={(event) => setFile(event.currentTarget.files?.[0] ?? null)}
        />
        <p id={`${fileId}-hint`} className={styles.hint}>
          {t('admin.backup.restore.fileHint')}
        </p>
      </div>
      <TextField
        name="passphrase"
        type="password"
        label={t('admin.backup.restore.passphrase')}
        description={t('admin.backup.restore.passphraseUploadHint')}
        autoComplete="off"
        value={passphrase}
        onChange={setPassphrase}
      />
      <div>
        <AriaButton
          type="submit"
          variant="danger"
          isDisabled={file === null}
          isPending={restore.isPending}
        >
          {restore.isPending ? t('admin.backup.restore.running') : t('admin.backup.restore.action')}
        </AriaButton>
      </div>
    </Form>
  );
}
