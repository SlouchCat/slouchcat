import type { AdminSettings } from '@slouchcat/api-client';
import { useId, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { Text, VisuallyHidden } from '@/ui';
import { Form, SettingRow, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import styles from './Integrations.module.css';
import { Actions, Outcome, TELEGRAM_DRAFT, useSecretUpdate } from './shared';

/**
 * The Telegram bot as two setting rows (Int-A): the bot Slouchcat found, and the write-only token
 * (checked by the server with getMe) with «Speichern» and «Löschen …» beside the field.
 */
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
    // A cleared token takes the typed input with it; a saved one only what was sent.
    if (body.clear) fields.discard(TELEGRAM_DRAFT);
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const env = telegram.source === 'env';
  const keepHintId = useId();

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Empty means «keep what is stored»: nothing to send, the focus stays on «Speichern»
    // (decision 18).
    if (token === '') return;
    setMessage(null);
    const sent = token;
    void save.run({ telegram: { botToken: token.trim() } }).then((outcome) => {
      // What was typed while the save ran stays a draft (decision 18, addendum 3).
      if (outcome.ok) fields.set(TELEGRAM_DRAFT, (current) => (current === sent ? '' : current));
    });
  };

  // A stored token: the field is empty, its placeholder says a token is set; that empty keeps it is
  // its accessible description (like a provider key). From deploy/.env the note stays visible.
  const keepHint = telegram.source === 'stored' ? t('admin.integrations.keys.newValue') : undefined;

  return (
    <div>
      {/* The bot's name comes only with a token saved here (getMe); for a token from deploy/.env
          the server names none, and then the row is left out rather than saying «no bot». */}
      {(telegram.botUsername || telegram.source === 'none') && (
        <SettingRow
          label={t('admin.integrations.telegram.botLabel')}
          help={t('admin.integrations.telegram.botHelp')}
        >
          <Text tone={telegram.botUsername ? 'primary' : 'secondary'}>
            {telegram.botUsername
              ? t('admin.integrations.telegram.bot', { name: telegram.botUsername })
              : t('admin.integrations.telegram.noBot')}
          </Text>
        </SettingRow>
      )}
      <SettingRow
        label={t('admin.integrations.telegram.token')}
        help={
          // The purpose is said here: the «Bot» row is left out for a token from deploy/.env.
          telegram.source === 'stored'
            ? `${t('admin.integrations.telegram.tokenHelp')} ${t('admin.integrations.telegram.tokenReplace')}`
            : t('admin.integrations.telegram.tokenHelp')
        }
      >
        <div className={styles.stack}>
          <Form className={styles.keyForm} onSubmit={onSubmit} validationErrors={errors?.fields}>
            <TextField
              name="telegram.botToken"
              type="password"
              label={t('admin.integrations.telegram.token')}
              hideLabel
              description={env ? t('admin.integrations.envHint') : undefined}
              placeholder={keepHint && t('admin.integrations.telegram.setPlaceholder')}
              aria-describedby={keepHint && keepHintId}
              autoComplete="off"
              spellCheck="false"
              mono
              isDisabled={env}
              value={token}
              onChange={(text) => fields.set(TELEGRAM_DRAFT, text)}
              // Empty keeps the stored token; only blanks alone are no token.
              validate={(text) =>
                env || text === '' || text.trim() ? null : t('errors.field.required')
              }
            />
            {!env && (
              <Actions
                name={name}
                saveLabel={t('admin.integrations.saveLabel', { name })}
                stored={telegram.source === 'stored'}
                isPending={save.isPending}
                onClear={() => {
                  setMessage(null);
                  return save.run({ clear: ['telegram'] });
                }}
              />
            )}
          </Form>
          {keepHint && <VisuallyHidden id={keepHintId}>{keepHint}</VisuallyHidden>}
          <Outcome error={errors?.form ? save.error : null} message={message} />
        </div>
      </SettingRow>
    </div>
  );
}
