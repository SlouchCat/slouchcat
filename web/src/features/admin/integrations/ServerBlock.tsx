import type { AdminSettings, AdminSettingsUpdate } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { sessionQuery } from '@/app/session';
import { Form, SettingRow, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import styles from './Integrations.module.css';
import { updateSettings } from '../queries';
import {
  Actions,
  Outcome,
  PUBLIC_URL_DRAFT,
  SourceChip,
  useSettingsSaved,
  useSettingsUpdate,
} from './shared';

/** The daily update check (A-5): a switch that is stored at once. */
function UpdateRow({ enabled }: { enabled: boolean }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const save = useSettingsUpdate((body) => {
    setMessage(
      t(body.updateCheckEnabled ? 'admin.integrations.update.on' : 'admin.integrations.update.off'),
    );
    // Switched off, the header loses its update badge (SessionInfo.updateAvailable).
    void queryClient.invalidateQueries({ queryKey: sessionQuery.queryKey });
  });
  const checked = save.isPending ? (save.variables.updateCheckEnabled ?? enabled) : enabled;
  return (
    <SettingRow
      label={t('admin.integrations.update.title')}
      help={t('admin.integrations.update.intro')}
    >
      <div className={styles.stack}>
        <label className={styles.toggle}>
          <input
            type="checkbox"
            checked={checked}
            onChange={(event) => {
              setMessage(null);
              save.mutate({ updateCheckEnabled: event.target.checked });
            }}
          />
          {t('admin.integrations.update.toggle')}
        </label>
        <Outcome error={save.error} message={message} />
      </div>
    </SettingRow>
  );
}

/** The public address for links in e-mails and messages. */
function PublicUrlRow({ settings, fields }: { settings: AdminSettings; fields: Drafts<string> }) {
  const { t } = useTranslation();
  const [message, setMessage] = useStatusMessage();
  const value = fields.valueOf(PUBLIC_URL_DRAFT) ?? '';
  const name = t('admin.integrations.publicUrl.name');
  const settingsSaved = useSettingsSaved((body) =>
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name })),
  );
  const save = useMutation({
    // `sent` is the draft as it was sent (null for «Löschen»).
    mutationFn: ({ body }: { body: AdminSettingsUpdate; sent: string | null }) =>
      updateSettings(body),
    onSuccess: async (saved, { body, sent }) => {
      // A cleared address takes the typed input with it; a saved one only what was sent, so
      // what was typed while the save ran stays a draft (decision 18).
      if (sent === null) fields.discard(PUBLIC_URL_DRAFT);
      else
        fields.set(PUBLIC_URL_DRAFT, (current) =>
          current === sent ? (saved.publicUrl ?? '') : current,
        );
      await settingsSaved(saved, body);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const env = settings.publicUrlSource === 'env';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
    if (!fields.isDirty(PUBLIC_URL_DRAFT)) return;
    setMessage(null);
    save.mutate({ body: { publicUrl: value.trim() }, sent: value });
  };

  return (
    <SettingRow
      label={t('admin.integrations.publicUrl.title')}
      help={t('admin.integrations.publicUrl.intro')}
    >
      <div className={styles.stack}>
        <Form className={styles.stack} onSubmit={onSubmit} validationErrors={errors?.fields}>
          <TextField
            name="publicUrl"
            type="url"
            label={t('admin.integrations.publicUrl.label')}
            description={
              env ? t('admin.integrations.envHint') : t('admin.integrations.publicUrl.hint')
            }
            autoComplete="off"
            spellCheck="false"
            mono
            isDisabled={env}
            isRequired={!env}
            value={value}
            onChange={(text) => fields.set(PUBLIC_URL_DRAFT, text)}
            validate={(text) => {
              if (env) return null;
              if (!text.trim()) return t('errors.field.required');
              return /^https?:\/\/[^\s?#]+$/i.test(text.trim())
                ? null
                : t('errors.field.invalid_url');
            }}
          />
          {env ? (
            <SourceChip source={settings.publicUrlSource} />
          ) : (
            <Actions
              name={name}
              stored={settings.publicUrlSource === 'stored'}
              isPending={save.isPending}
              onClear={() => {
                setMessage(null);
                save.mutate({ body: { clear: ['publicUrl'] }, sent: null });
              }}
            >
              <SourceChip source={settings.publicUrlSource} />
            </Actions>
          )}
        </Form>
        <Outcome error={errors?.form ? save.error : null} message={message} />
      </div>
    </SettingRow>
  );
}

/** The server's own settings: the update check and the public address, one row each. */
export function ServerBlock({
  settings,
  fields,
}: {
  settings: AdminSettings;
  fields: Drafts<string>;
}) {
  return (
    <div>
      <UpdateRow enabled={settings.updateCheckEnabled} />
      <PublicUrlRow settings={settings} fields={fields} />
    </div>
  );
}
