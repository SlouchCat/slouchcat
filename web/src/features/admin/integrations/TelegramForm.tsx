import type { AdminSettings } from '@slouchcat/api-client';
import { type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { Text } from '@/ui';
import { Form, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import styles from '../IntegrationsPage.module.css';
import { Actions, Outcome, TELEGRAM_DRAFT, useSecretUpdate } from './shared';

/** The Telegram bot: write-only token (checked by the server with getMe) and its name. */
export function TelegramForm({
  telegram,
  fields,
}: {
  telegram: AdminSettings['telegram'];
  fields: Drafts<string>;
}) {
  const { t } = useTranslation();
  const [message, setMessage] = useStatusMessage();
  const token = fields.valueOf(TELEGRAM_DRAFT) ?? '';
  const name = t('admin.integrations.telegram.name');
  const save = useSecretUpdate((body) => {
    fields.discard(TELEGRAM_DRAFT);
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const env = telegram.source === 'env';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    void save.run({ telegram: { botToken: token.trim() } });
  };

  return (
    <>
      <Text tone="secondary">{t('admin.integrations.telegram.intro')}</Text>
      {telegram.botUsername && (
        <Text>{t('admin.integrations.telegram.bot', { name: telegram.botUsername })}</Text>
      )}
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <TextField
          name="telegram.botToken"
          type="password"
          label={t('admin.integrations.telegram.token')}
          description={
            env ? t('admin.integrations.envHint') : t('admin.integrations.telegram.tokenKeep')
          }
          autoComplete="off"
          spellCheck="false"
          mono
          isDisabled={env}
          isRequired={!env}
          value={token}
          onChange={(text) => fields.set(TELEGRAM_DRAFT, text)}
          validate={(text) => (env || text.trim() ? null : t('errors.field.required'))}
        />
        {!env && (
          <Actions
            name={name}
            stored={telegram.source === 'stored'}
            isPending={save.isPending}
            onClear={() => {
              setMessage(null);
              void save.run({ clear: ['telegram'] });
            }}
          />
        )}
      </Form>
      <Outcome error={errors?.form ? save.error : null} message={message} />
    </>
  );
}
