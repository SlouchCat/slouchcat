import type { AdminSettings } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { Notice, Text, TextLink } from '@/ui';
import { AriaButton, Form, RadioGroup, TextField, useStatusMessage, type Drafts } from '@/ui/aria';
import { sendTestEmail } from '../queries';
import styles from '../IntegrationsPage.module.css';
import {
  Actions,
  EMAIL_DRAFT,
  emailValues,
  Outcome,
  PASSWORD_DRAFT,
  TLS_MODES,
  useSecretUpdate,
  type EmailValues,
} from './shared';

/** The e-mail server: host, port, user, write-only password, sender, TLS; and a test mail. */
export function EmailForm({
  email,
  drafts,
  fields,
}: {
  email: AdminSettings['email'];
  drafts: Drafts<EmailValues>;
  fields: Drafts<string>;
}) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const [message, setMessage] = useStatusMessage();
  const values = drafts.valueOf(EMAIL_DRAFT) ?? emailValues(email);
  const password = fields.valueOf(PASSWORD_DRAFT) ?? '';
  const name = t('admin.integrations.email.name');
  const save = useSecretUpdate((body) => {
    // A cleared server takes the typed input with it; a saved one only what was sent.
    if (body.clear) {
      drafts.discard(EMAIL_DRAFT);
      fields.discard(PASSWORD_DRAFT);
    }
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const test = useMutation({
    mutationFn: sendTestEmail,
    onMutate: () => setMessage(null),
    onSuccess: () =>
      setMessage(t('admin.integrations.email.testSent', { email: session.account.email })),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const env = email.source === 'env';
  const change = (field: keyof EmailValues) => (value: string) =>
    drafts.set(EMAIL_DRAFT, (current) => ({ ...current, [field]: value }));
  const required = (text: string) => (env || text.trim() ? null : t('errors.field.required'));

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
    if (!drafts.isDirty(EMAIL_DRAFT) && !password) return;
    const sent = values;
    const sentPassword = password;
    void save
      .run({
        email: {
          host: values.host.trim(),
          port: Number(values.port),
          ...(values.user.trim() ? { user: values.user.trim() } : {}),
          ...(password ? { password } : {}),
          from: values.from.trim(),
          tls: values.tls,
        },
      })
      .then((outcome) => {
        if (!outcome.ok) return;
        // What was typed while the save ran stays a draft (decision 18, addendum 3).
        drafts.set(EMAIL_DRAFT, (current) =>
          JSON.stringify(current) === JSON.stringify(sent)
            ? emailValues(outcome.data.email)
            : current,
        );
        fields.set(PASSWORD_DRAFT, (current) => (current === sentPassword ? '' : current));
      });
  };

  // `email_send_failed` names why (without secrets) in the problem's detail.
  const testError =
    test.error && isApiError(test.error, 502) && test.error.problem?.detail
      ? t('admin.integrations.email.testFailed', { detail: test.error.problem.detail })
      : null;

  return (
    <>
      <div className={styles.stack}>
        <Text tone="secondary">{t('admin.integrations.email.intro')}</Text>
        {/* The channels each profile uses are set on their own page; it links back here (BEN-1). */}
        <TextLink href={paths.notificationSettings}>
          {t('admin.integrations.notificationsLink')}
        </TextLink>
      </div>
      {env && <Text tone="secondary">{t('admin.integrations.envHint')}</Text>}
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.fields}>
          <TextField
            name="email.host"
            label={t('admin.integrations.email.host')}
            autoComplete="off"
            spellCheck="false"
            isDisabled={env}
            isRequired={!env}
            value={values.host}
            onChange={change('host')}
            validate={required}
          />
          <TextField
            name="email.port"
            label={t('admin.integrations.email.port')}
            inputMode="numeric"
            autoComplete="off"
            isDisabled={env}
            isRequired={!env}
            value={values.port}
            onChange={change('port')}
            validate={(text) => {
              if (env) return null;
              const port = Number(text);
              return Number.isInteger(port) && port > 0 && port < 65536
                ? null
                : t('errors.field.invalid');
            }}
          />
          <TextField
            name="email.user"
            label={t('admin.integrations.email.user')}
            autoComplete="off"
            spellCheck="false"
            isDisabled={env}
            value={values.user}
            onChange={change('user')}
          />
          <TextField
            name="email.password"
            type="password"
            label={t('admin.integrations.email.password')}
            description={
              email.source === 'stored'
                ? t('admin.integrations.email.passwordKeep')
                : t('admin.integrations.email.passwordHint')
            }
            autoComplete="new-password"
            isDisabled={env}
            value={password}
            onChange={(text) => fields.set(PASSWORD_DRAFT, text)}
          />
        </div>
        <TextField
          name="email.from"
          label={t('admin.integrations.email.from')}
          description={t('admin.integrations.email.fromHint')}
          autoComplete="off"
          spellCheck="false"
          isDisabled={env}
          isRequired={!env}
          value={values.from}
          onChange={change('from')}
          validate={required}
        />
        <RadioGroup
          name="email.tls"
          label={t('admin.integrations.email.tls')}
          isDisabled={env}
          value={values.tls}
          onChange={(value) =>
            drafts.set(EMAIL_DRAFT, (current) => ({
              ...current,
              tls: TLS_MODES.find((mode) => mode === value) ?? 'starttls',
            }))
          }
          options={TLS_MODES.map((mode) => ({
            value: mode,
            label: t(`admin.integrations.email.tlsOptions.${mode}`),
          }))}
        />
        {env ? (
          <div className={styles.actions}>
            <AriaButton icon="check" isPending={test.isPending} onPress={() => test.mutate()}>
              {t('admin.integrations.email.test')}
            </AriaButton>
          </div>
        ) : (
          <Actions
            name={name}
            stored={email.source === 'stored'}
            isPending={save.isPending}
            onClear={() => {
              setMessage(null);
              void save.run({ clear: ['email'] });
            }}
          >
            {email.source !== 'none' && (
              <AriaButton icon="check" isPending={test.isPending} onPress={() => test.mutate()}>
                {t('admin.integrations.email.test')}
              </AriaButton>
            )}
          </Actions>
        )}
      </Form>
      {testError ? (
        <Notice tone="danger">{testError}</Notice>
      ) : (
        test.error && <Notice tone="danger">{problemMessage(t, test.error)}</Notice>
      )}
      <Outcome error={errors?.form ? save.error : null} message={message} />
    </>
  );
}
