import { useId, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { Notice } from '@/ui';
import { AriaButton, Form, TextField } from '@/ui/aria';
import styles from './Backup.module.css';

/**
 * Puts the page's file back into a fresh file field, so a tab change does not show it empty while
 * the page still holds it. A browser without a constructible DataTransfer keeps the field empty;
 * the button still knows the file.
 */
function keepFile(input: HTMLInputElement | null, file: File | null) {
  if (!input || !file || input.files?.[0] === file) return;
  try {
    const transfer = new DataTransfer();
    transfer.items.add(file);
    input.files = transfer.files;
  } catch {
    // No DataTransfer here (jsdom, old browsers): the field stays empty, the page has the file.
  }
}

/**
 * The tab «Aus Datei wiederherstellen» (SIC-2): the warning, the archive, the passphrase of an
 * encrypted one; the button hands both to the page, which asks its confirmation and uploads. The
 * file is the page's (not a secret, it survives a tab change), and so is the answer to the last
 * upload. The passphrase lives only here, never in a draft or the mutation cache, and goes with
 * the tab.
 */
export function UploadTab({
  file,
  onFileChange,
  error,
  isPending,
  onRestore,
}: {
  file: File | null;
  onFileChange: (file: File | null) => void;
  /** The answer to the last upload, or null. */
  error: unknown;
  isPending: boolean;
  onRestore: (file: File, passphrase: string) => void;
}) {
  const { t } = useTranslation();
  const fileId = useId();
  const [passphrase, setPassphrase] = useState('');
  const errors = error ? formErrors(t, error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (file && !isPending) onRestore(file, passphrase);
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
          ref={(input) => keepFile(input, file)}
          id={fileId}
          name="file"
          type="file"
          accept=".gz,.age"
          aria-describedby={`${fileId}-hint`}
          className={styles.fileInput}
          onChange={(event) => onFileChange(event.currentTarget.files?.[0] ?? null)}
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
        <AriaButton type="submit" variant="danger" isDisabled={file === null} isPending={isPending}>
          {isPending ? t('admin.backup.restore.running') : t('admin.backup.restore.action')}
        </AriaButton>
      </div>
    </Form>
  );
}
