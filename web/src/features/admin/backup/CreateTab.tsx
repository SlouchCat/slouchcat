import type { Backup, BackupRequest } from '@slouchcat/api-client';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice, Text } from '@/ui';
import { AriaButton, Form, RadioGroup, TextField } from '@/ui/aria';
import { MIN_PASSPHRASE } from './api';
import styles from './Backup.module.css';

export type CreateMode = 'without' | 'with';

/**
 * The tab «Neue Sicherung» (SIC-1): without secrets (the default) or with them behind a passphrase
 * typed twice. The scope is the page's (it stays when the tab changes); the passphrases live only
 * here, never in a draft or the mutation cache, and go with the tab.
 */
export function CreateTab({
  mode,
  onModeChange,
  onCreated,
}: {
  mode: CreateMode;
  onModeChange: (mode: CreateMode) => void;
  onCreated: (backup: Backup) => Promise<void> | void;
}) {
  const { t } = useTranslation();
  const [passphrase, setPassphrase] = useState('');
  const [repeat, setRepeat] = useState('');
  const create = useTransientMutation(
    (body: BackupRequest): Promise<Backup> => request(api.POST('/admin/backups', { body })),
    onCreated,
  );
  const errors = create.error ? formErrors(t, create.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void create.run(
      mode === 'with' ? { includeSecrets: true, passphrase } : { includeSecrets: false },
    );
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      <Text tone="secondary">{t('admin.backup.create.intro')}</Text>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <RadioGroup
        name="includeSecrets"
        label={t('admin.backup.create.scope')}
        value={mode}
        onChange={(value) => {
          onModeChange(value === 'with' ? 'with' : 'without');
          // Nothing typed for a scope that is no longer chosen stays around.
          setPassphrase('');
          setRepeat('');
        }}
        options={[
          { value: 'without', label: t('admin.backup.create.without') },
          { value: 'with', label: t('admin.backup.create.with') },
        ]}
        description={
          mode === 'with'
            ? t('admin.backup.create.withHint', { min: MIN_PASSPHRASE })
            : t('admin.backup.create.withoutHint')
        }
      />
      {mode === 'with' && (
        <div className={styles.passphrases}>
          <TextField
            name="passphrase"
            type="password"
            label={t('admin.backup.create.passphrase')}
            description={t('admin.backup.create.passphraseHint', { min: MIN_PASSPHRASE })}
            autoComplete="new-password"
            isRequired
            value={passphrase}
            onChange={setPassphrase}
            validate={(value) =>
              value.length >= MIN_PASSPHRASE
                ? null
                : t('admin.backup.create.passphraseTooShort', { min: MIN_PASSPHRASE })
            }
          />
          <TextField
            name="passphraseRepeat"
            type="password"
            label={t('admin.backup.create.passphraseRepeat')}
            autoComplete="new-password"
            isRequired
            value={repeat}
            onChange={setRepeat}
            validate={(value) =>
              value === passphrase ? null : t('admin.backup.create.passphraseMismatch')
            }
          />
        </div>
      )}
      <div>
        <AriaButton type="submit" variant="primary" isPending={create.isPending}>
          {create.isPending ? t('admin.backup.create.creating') : t('admin.backup.create.submit')}
        </AriaButton>
      </div>
    </Form>
  );
}
