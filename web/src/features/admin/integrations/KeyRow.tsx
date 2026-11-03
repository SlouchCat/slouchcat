import type { KeyState } from '@slouchcat/api-client';
import { useId, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { paths } from '@/app/paths';
import { TextLink, VisuallyHidden } from '@/ui';
import { Form, SettingRow, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import styles from './Integrations.module.css';
import { Actions, keyDraft, Outcome, SourceChip, useSecretUpdate, type KeyName } from './shared';

/**
 * Where a signed-in user gets each key at its provider (acceptance K12-9). Addresses, not text:
 * they stay out of the locales.
 */
const KEY_URLS = {
  tmdb: 'https://www.themoviedb.org/settings/api',
  tvdb: 'https://thetvdb.com/dashboard/account/apikey',
  tvdbPin: 'https://thetvdb.com/dashboard/account/subscription',
  youtube: 'https://console.cloud.google.com/apis/credentials',
  fanart: 'https://fanart.tv/get-an-api-key/',
  fanartClient: 'https://fanart.tv/get-an-api-key/',
  trakt: 'https://developer.trakt.tv/apps',
} as const satisfies Record<KeyName, string>;

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
    // A cleared key takes the typed input with it; a saved one only what was sent.
    if (body.clear) drafts.discard(draft);
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const errors = save.error ? formErrors(t, save.error, { tmdbApiKey: field }) : null;
  const env = state.source === 'env';
  const keepHintId = useId();

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Empty means «keep what is stored»: nothing to send, the focus stays on «Speichern»
    // (decision 18).
    if (value === '') return;
    setMessage(null);
    const sent = value;
    void save.run({ keys: { [keyName]: value.trim() } }).then((outcome) => {
      // What was typed while the save ran stays a draft (decision 18, addendum 3).
      if (outcome.ok) drafts.set(draft, (current) => (current === sent ? '' : current));
    });
  };

  // A stored key: the field is empty, its placeholder says a key is set (K12-9); that empty keeps
  // it is its accessible description. A key from deploy/.env keeps its visible note under the field.
  const keepHint = state.source === 'stored' ? t('admin.integrations.keys.newValue') : undefined;

  // One line per key (Int-A): the source next to the label, field and buttons side by side.
  return (
    <SettingRow
      label={name}
      labelAddon={<SourceChip source={state.source} hint={state.hint} />}
      help={
        <>
          {t(`admin.integrations.keys.${keyName}.description`)}{' '}
          <TextLink href={KEY_URLS[keyName]} external>
            {t(`admin.integrations.keys.${keyName}.link`)}
          </TextLink>
          {keyName === 'trakt' && (
            <>
              {' '}
              <TextLink href={paths.adminSpotlight}>
                {t('admin.integrations.keys.traktStatus')}
              </TextLink>
            </>
          )}
        </>
      }
    >
      <div className={styles.stack}>
        <Form className={styles.keyForm} onSubmit={onSubmit} validationErrors={errors?.fields}>
          <TextField
            name={field}
            type="password"
            label={name}
            hideLabel
            description={env ? t('admin.integrations.envHint') : undefined}
            placeholder={keepHint && t('admin.integrations.keys.setPlaceholder')}
            aria-describedby={keepHint && keepHintId}
            autoComplete="off"
            spellCheck="false"
            mono
            isDisabled={env}
            value={value}
            onChange={(text) => drafts.set(draft, text)}
            // Empty keeps the stored key; only blanks alone are no key.
            validate={(text) =>
              env || text === '' || text.trim() ? null : t('errors.field.required')
            }
          />
          {!env && (
            <Actions
              name={name}
              saveLabel={t('admin.integrations.saveLabel', { name })}
              stored={state.source === 'stored'}
              isPending={save.isPending}
              onClear={() => {
                setMessage(null);
                return save.run({ clear: [keyName] });
              }}
            />
          )}
        </Form>
        {keepHint && <VisuallyHidden id={keepHintId}>{keepHint}</VisuallyHidden>}
        <Outcome error={errors?.form ? save.error : null} message={message} />
      </div>
    </SettingRow>
  );
}
