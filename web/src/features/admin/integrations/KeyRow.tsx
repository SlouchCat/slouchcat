import type { KeyState } from '@slouchcat/api-client';
import { type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { paths } from '@/app/paths';
import { TextLink } from '@/ui';
import { Form, SettingRow, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import styles from '../IntegrationsPage.module.css';
import { Actions, keyDraft, Outcome, SourceChip, useSecretUpdate, type KeyName } from './shared';

/** One provider key: write-only input, save and delete; read-only when deploy/.env sets it. */
export function KeyRow({
  keyName,
  state,
  drafts,
}: {
  keyName: KeyName;
  state: KeyState;
  drafts: Drafts<string>;
}) {
  const { t } = useTranslation();
  const [message, setMessage] = useStatusMessage();
  const draft = keyDraft(keyName);
  const value = drafts.valueOf(draft) ?? '';
  const name = t(`admin.integrations.keys.${keyName}.name`);
  const field = `keys.${keyName}`;
  const save = useSecretUpdate((body) => {
    drafts.discard(draft);
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const errors = save.error ? formErrors(t, save.error, { tmdbApiKey: field }) : null;
  const env = state.source === 'env';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    void save.run({ keys: { [keyName]: value.trim() } });
  };

  return (
    <SettingRow label={name} help={t(`admin.integrations.keys.${keyName}.description`)}>
      <div className={styles.stack}>
        <Form className={styles.stack} onSubmit={onSubmit} validationErrors={errors?.fields}>
          <TextField
            name={field}
            type="password"
            label={t(`admin.integrations.keys.${keyName}.label`)}
            description={
              env
                ? t('admin.integrations.envHint')
                : state.source === 'stored'
                  ? t('admin.integrations.keys.newValue')
                  : undefined
            }
            autoComplete="off"
            spellCheck="false"
            mono
            isDisabled={env}
            isRequired={!env}
            value={value}
            onChange={(text) => drafts.set(draft, text)}
            validate={(text) => (env || text.trim() ? null : t('errors.field.required'))}
          />
          {env ? (
            <SourceChip source={state.source} />
          ) : (
            <Actions
              name={name}
              saveLabel={t('admin.integrations.keys.saveLabel', { name })}
              stored={state.source === 'stored'}
              isPending={save.isPending}
              onClear={() => {
                setMessage(null);
                void save.run({ clear: [keyName] });
              }}
            >
              <SourceChip source={state.source} hint={state.hint} />
            </Actions>
          )}
        </Form>
        <Outcome error={errors?.form ? save.error : null} message={message} />
        {keyName === 'trakt' && (
          <TextLink href={paths.adminSpotlight}>
            {t('admin.integrations.keys.traktStatus')}
          </TextLink>
        )}
      </div>
    </SettingRow>
  );
}
