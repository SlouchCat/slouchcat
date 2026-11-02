import type { AdminSettings } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useId, useRef, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { Notice, Text, TextLink } from '@/ui';
import {
  AriaButton,
  Form,
  RadioGroup,
  SettingRow,
  TextField,
  useStatusMessage,
  type Drafts,
} from '@/ui/aria';
import { sendTestEmail } from '../queries';
import styles from './Integrations.module.css';
import {
  ClearButton,
  EMAIL_DRAFT,
  emailValues,
  Outcome,
  PASSWORD_DRAFT,
  TLS_MODES,
  useSecretUpdate,
  type EmailValues,
} from './shared';

/**
 * The e-mail server as setting rows (Int-A): server and port, user, write-only password, sender,
 * TLS; below them one line with the test mail on the left and «Löschen …», «Speichern» on the
 * right. The six values save together.
 */
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
  const saveRef = useRef<HTMLButtonElement>(null);
  const passwordHelpId = useId();
  const values = drafts.valueOf(EMAIL_DRAFT) ?? emailValues(email);
  const password = fields.valueOf(PASSWORD_DRAFT) ?? '';
  const dirty = drafts.isDirty(EMAIL_DRAFT) || password !== '';
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
    if (!dirty) return;
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

  // «Verwerfen» disappears with the change it discards: the focus moves on to «Speichern».
  const onDiscard = () => {
    setMessage(null);
    drafts.discard(EMAIL_DRAFT);
    fields.discard(PASSWORD_DRAFT);
    saveRef.current?.focus();
  };

  // `email_send_failed` names why (without secrets) in the problem's detail.
  const testError =
    test.error && isApiError(test.error, 502) && test.error.problem?.detail
      ? t('admin.integrations.email.testFailed', { detail: test.error.problem.detail })
      : null;
  const testButton = (
    <AriaButton icon="check" isPending={test.isPending} onPress={() => test.mutate()}>
      {t('admin.integrations.email.test')}
    </AriaButton>
  );

  return (
    <div className={styles.stack}>
      {env && <Text tone="secondary">{t('admin.integrations.envHint')}</Text>}
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <SettingRow
          label={t('admin.integrations.email.hostPort')}
          help={
            <>
              {t('admin.integrations.email.intro')}{' '}
              {/* The channels each profile uses are set on their own page; it links back here
                  (BEN-1). */}
              <TextLink href={paths.notificationSettings}>
                {t('admin.integrations.notificationsLink')}
              </TextLink>
            </>
          }
        >
          {/* The host is named by the visible row label, the port by its word in it (WCAG 2.5.3). */}
          <div className={styles.hostPort}>
            <TextField
              name="email.host"
              label={t('admin.integrations.email.hostPort')}
              hideLabel
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
              hideLabel
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
          </div>
        </SettingRow>
        <SettingRow
          label={t('admin.integrations.email.user')}
          help={t('admin.integrations.email.userHint')}
        >
          <div className={styles.stack}>
            <TextField
              name="email.user"
              label={t('admin.integrations.email.user')}
              hideLabel
              autoComplete="off"
              spellCheck="false"
              isDisabled={env}
              value={values.user}
              onChange={change('user')}
            />
          </div>
        </SettingRow>
        <SettingRow
          label={t('admin.integrations.email.password')}
          help={
            // The help is the field's description too: how a stored password is kept.
            <span id={passwordHelpId}>
              {email.source === 'stored'
                ? t('admin.integrations.email.passwordKeep')
                : t('admin.integrations.email.passwordHint')}
            </span>
          }
        >
          <div className={styles.stack}>
            <TextField
              name="email.password"
              type="password"
              label={t('admin.integrations.email.password')}
              hideLabel
              aria-describedby={passwordHelpId}
              autoComplete="new-password"
              isDisabled={env}
              value={password}
              onChange={(text) => fields.set(PASSWORD_DRAFT, text)}
            />
          </div>
        </SettingRow>
        <SettingRow
          label={t('admin.integrations.email.from')}
          help={t('admin.integrations.email.fromHint')}
        >
          <div className={styles.stack}>
            <TextField
              name="email.from"
              label={t('admin.integrations.email.from')}
              hideLabel
              autoComplete="off"
              spellCheck="false"
              isDisabled={env}
              isRequired={!env}
              value={values.from}
              onChange={change('from')}
              validate={required}
            />
          </div>
        </SettingRow>
        <SettingRow
          label={t('admin.integrations.email.tls')}
          help={t('admin.integrations.email.tlsHint')}
        >
          <RadioGroup
            name="email.tls"
            label={t('admin.integrations.email.tls')}
            hideLabel
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
        </SettingRow>
        <div className={styles.footer}>
          {(env || email.source !== 'none') && (
            <>
              <Text variant="caption" tone="secondary" as="span">
                {t('admin.integrations.email.testTo', { email: session.account.email })}
              </Text>
              {testButton}
            </>
          )}
          {!env && (
            <div className={styles.footerEnd}>
              {dirty && (
                <AriaButton variant="ghost" isDisabled={save.isPending} onPress={onDiscard}>
                  {t('admin.integrations.discard')}
                </AriaButton>
              )}
              {email.source === 'stored' && (
                <ClearButton
                  name={name}
                  onClear={() => {
                    setMessage(null);
                    void save.run({ clear: ['email'] });
                  }}
                />
              )}
              <AriaButton ref={saveRef} type="submit" variant="primary" isPending={save.isPending}>
                {save.isPending ? t('admin.integrations.saving') : t('admin.integrations.save')}
              </AriaButton>
            </div>
          )}
        </div>
      </Form>
      {testError ? (
        <Notice tone="danger">{testError}</Notice>
      ) : (
        test.error && <Notice tone="danger">{problemMessage(t, test.error)}</Notice>
      )}
      <Outcome error={errors?.form ? save.error : null} message={message} />
    </div>
  );
}
