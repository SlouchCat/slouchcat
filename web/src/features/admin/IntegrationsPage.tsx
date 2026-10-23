import type {
  AdminSettings,
  AdminSettingsUpdate,
  KeyState,
  SettingSource,
  SMTPTLS,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { sessionQuery, useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, Notice, Page, Panel, Skeleton, Text, TextLink } from '@/ui';
import { AriaButton, ConfirmDialog, Form, RadioGroup, TextField } from '@/ui/aria';
import {
  dashboardQuery,
  rotatePushKeys,
  sendTestEmail,
  settingsQuery,
  updateSettings,
} from './queries';
import { WebhooksSection } from './WebhooksSection';
import styles from './IntegrationsPage.module.css';
import table from './AdminTablePage.module.css';

type KeyName = keyof AdminSettings['keys'];

const KEYS = ['tmdb', 'tvdb', 'tvdbPin', 'youtube', 'trakt'] as const satisfies readonly KeyName[];

const TLS_MODES = ['starttls', 'tls', 'none'] as const satisfies readonly SMTPTLS[];

const sourceTone = { env: 'info', stored: 'success', none: 'outline' } as const;

/** After a part of the settings is stored: the answer is the whole new state of the page. */
function useSettingsSaved(onSaved: (body: AdminSettingsUpdate) => void) {
  const queryClient = useQueryClient();
  return async (settings: AdminSettings, body: AdminSettingsUpdate) => {
    queryClient.setQueryData(settingsQuery.queryKey, settings);
    onSaved(body);
    // The dashboard shows the integrations' state.
    await queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
  };
}

/** Stores a part of the settings without a secret (update check, public address). */
function useSettingsUpdate(onSaved: (body: AdminSettingsUpdate) => void) {
  return useMutation({ mutationFn: updateSettings, onSuccess: useSettingsSaved(onSaved) });
}

/**
 * Stores a part of the settings that may carry a secret (a key, the SMTP password, the bot
 * token): the value never stays in the query client (useTransientMutation).
 */
function useSecretUpdate(onSaved: (body: AdminSettingsUpdate) => void) {
  return useTransientMutation(updateSettings, useSettingsSaved(onSaved));
}

/** Where a value comes from: deploy/.env, stored here (with its last four characters), or none. */
function SourceChip({ source, hint }: { source: SettingSource; hint?: string }) {
  const { t } = useTranslation();
  return (
    <Chip tone={sourceTone[source]}>
      {source === 'stored' && hint
        ? t('admin.integrations.source.storedHint', { hint })
        : t(`admin.integrations.source.${source}`)}
    </Chip>
  );
}

function Section({
  title,
  intro,
  source,
  children,
}: {
  title: string;
  intro: string;
  source?: ReactNode;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <Panel as="section" aria-labelledby={id}>
      <div className={styles.head}>
        <div className={styles.headText}>
          <h2 id={id} className={styles.title}>
            {title}
          </h2>
          <Text tone="secondary">{intro}</Text>
        </div>
        {source}
      </div>
      {children}
    </Panel>
  );
}

/** The outcome of a section's last action: an error notice or a polite status line. */
function Outcome({ error, message }: { error: unknown; message: string | null }) {
  const { t } = useTranslation();
  return (
    <>
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
      <div role="status" className={styles.live}>
        {message && !error && <Notice live={false}>{message}</Notice>}
      </div>
    </>
  );
}

/** The buttons of a stored value: save, and delete after a confirmation. */
function Actions({
  name,
  saveLabel,
  stored,
  isPending,
  onClear,
  children,
}: {
  name: string;
  saveLabel?: string;
  stored: boolean;
  isPending: boolean;
  onClear: () => void;
  children?: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <div className={styles.actions}>
      <AriaButton type="submit" variant="primary" isPending={isPending} aria-label={saveLabel}>
        {isPending ? t('admin.integrations.saving') : t('admin.integrations.save')}
      </AriaButton>
      {stored && (
        <ConfirmDialog
          trigger={
            <AriaButton variant="ghost" aria-label={t('admin.integrations.clear.label', { name })}>
              {t('admin.integrations.clear.action')}
            </AriaButton>
          }
          title={t('admin.integrations.clear.title', { name })}
          body={t('admin.integrations.clear.body')}
          confirmLabel={t('admin.integrations.clear.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={onClear}
        />
      )}
      {children}
    </div>
  );
}

/** One provider key: write-only input, save and delete; read-only when deploy/.env sets it. */
function KeyForm({ keyName, state }: { keyName: KeyName; state: KeyState }) {
  const { t } = useTranslation();
  const [value, setValue] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const name = t(`admin.integrations.keys.${keyName}.name`);
  const field = `keys.${keyName}`;
  const save = useSecretUpdate((body) => {
    setValue('');
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
    <div className={styles.item}>
      <div className={styles.head}>
        <h3 className={styles.itemTitle}>{name}</h3>
        <SourceChip source={state.source} hint={state.hint} />
      </div>
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <TextField
          name={field}
          type="password"
          label={t(`admin.integrations.keys.${keyName}.label`)}
          description={
            env
              ? t('admin.integrations.envHint')
              : state.source === 'stored'
                ? t('admin.integrations.keys.newValue')
                : t(`admin.integrations.keys.${keyName}.description`)
          }
          autoComplete="off"
          spellCheck="false"
          mono
          isDisabled={env}
          isRequired={!env}
          value={value}
          onChange={setValue}
          validate={(text) => (env || text.trim() ? null : t('errors.field.required'))}
        />
        {!env && (
          <Actions
            name={name}
            saveLabel={t('admin.integrations.keys.saveLabel', { name })}
            stored={state.source === 'stored'}
            isPending={save.isPending}
            onClear={() => {
              setMessage(null);
              void save.run({ clear: [keyName] });
            }}
          />
        )}
      </Form>
      <Outcome error={errors?.form ? save.error : null} message={message} />
      {keyName === 'trakt' && (
        <TextLink href={paths.adminHomeRows}>{t('admin.integrations.keys.traktStatus')}</TextLink>
      )}
    </div>
  );
}

function KeysSection({ keys }: { keys: AdminSettings['keys'] }) {
  const { t } = useTranslation();
  return (
    <Section title={t('admin.integrations.keys.title')} intro={t('admin.integrations.keys.intro')}>
      <div className={styles.items}>
        {KEYS.map((keyName) => (
          <KeyForm key={keyName} keyName={keyName} state={keys[keyName]} />
        ))}
      </div>
    </Section>
  );
}

interface EmailValues {
  host: string;
  port: string;
  user: string;
  from: string;
  tls: SMTPTLS;
}

const emailValues = (email: AdminSettings['email']): EmailValues => ({
  host: email.host ?? '',
  port: email.port ? String(email.port) : '587',
  user: email.user ?? '',
  from: email.from ?? '',
  tls: email.tls ?? 'starttls',
});

/** The e-mail server: host, port, user, write-only password, sender, TLS; and a test mail. */
function EmailSection({ email }: { email: AdminSettings['email'] }) {
  const { t } = useTranslation();
  const session = useActiveSession();
  const [values, setValues] = useState(() => emailValues(email));
  const [password, setPassword] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const name = t('admin.integrations.email.name');
  const save = useSecretUpdate((body) => {
    setPassword('');
    if (body.clear) setValues(emailValues({ source: 'none' }));
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
    setValues((current) => ({ ...current, [field]: value }));
  const required = (text: string) => (env || text.trim() ? null : t('errors.field.required'));

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    void save.run({
      email: {
        host: values.host.trim(),
        port: Number(values.port),
        ...(values.user.trim() ? { user: values.user.trim() } : {}),
        ...(password ? { password } : {}),
        from: values.from.trim(),
        tls: values.tls,
      },
    });
  };

  // `email_send_failed` names why (without secrets) in the problem's detail.
  const testError =
    test.error && isApiError(test.error, 502) && test.error.problem?.detail
      ? t('admin.integrations.email.testFailed', { detail: test.error.problem.detail })
      : null;

  return (
    <Section
      title={t('admin.integrations.email.title')}
      intro={t('admin.integrations.email.intro')}
      source={<SourceChip source={email.source} />}
    >
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
            onChange={setPassword}
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
            setValues((current) => ({
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
    </Section>
  );
}

/** The Telegram bot: write-only token (checked by the server with getMe) and its name. */
function TelegramSection({ telegram }: { telegram: AdminSettings['telegram'] }) {
  const { t } = useTranslation();
  const [token, setToken] = useState('');
  const [message, setMessage] = useState<string | null>(null);
  const name = t('admin.integrations.telegram.name');
  const save = useSecretUpdate((body) => {
    setToken('');
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
    <Section
      title={t('admin.integrations.telegram.title')}
      intro={t('admin.integrations.telegram.intro')}
      source={<SourceChip source={telegram.source} />}
    >
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
          onChange={setToken}
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
    </Section>
  );
}

/** Web Push: whether a VAPID pair exists, its public key, and a new pair after a confirmation. */
function PushSection({ push }: { push: AdminSettings['push'] }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);
  const rotate = useMutation({
    mutationFn: rotatePushKeys,
    onMutate: () => setMessage(null),
    onSuccess: (settings) => {
      queryClient.setQueryData(settingsQuery.queryKey, settings);
      setMessage(t('admin.integrations.push.rotated'));
    },
  });
  return (
    <Section title={t('admin.integrations.push.title')} intro={t('admin.integrations.push.intro')}>
      <Text tone="secondary">
        {push.configured
          ? t('admin.integrations.push.configured')
          : t('admin.integrations.push.notConfigured')}
      </Text>
      {push.publicKey && (
        <div className={styles.publicKey}>
          <Text as="span" variant="caption" tone="muted">
            {t('admin.integrations.push.publicKey')}
          </Text>
          <code className={table.url}>{push.publicKey}</code>
        </div>
      )}
      <div className={styles.actions}>
        <ConfirmDialog
          trigger={
            <AriaButton icon="refresh" isPending={rotate.isPending}>
              {t('admin.integrations.push.rotate.action')}
            </AriaButton>
          }
          title={t('admin.integrations.push.rotate.title')}
          body={t('admin.integrations.push.rotate.body')}
          confirmLabel={t('admin.integrations.push.rotate.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => rotate.mutate()}
        />
      </div>
      <Outcome error={rotate.error} message={message} />
    </Section>
  );
}

/** The daily update check (A-5): a switch that is stored at once. */
function UpdateSection({ enabled }: { enabled: boolean }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);
  const save = useSettingsUpdate((body) => {
    setMessage(
      t(body.updateCheckEnabled ? 'admin.integrations.update.on' : 'admin.integrations.update.off'),
    );
    // Switched off, the header loses its update badge (SessionInfo.updateAvailable).
    void queryClient.invalidateQueries({ queryKey: sessionQuery.queryKey });
  });
  const checked = save.isPending ? (save.variables.updateCheckEnabled ?? enabled) : enabled;
  return (
    <Section
      title={t('admin.integrations.update.title')}
      intro={t('admin.integrations.update.intro')}
    >
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
    </Section>
  );
}

/** The public address for links in e-mails and messages. */
function PublicUrlSection({ settings }: { settings: AdminSettings }) {
  const { t } = useTranslation();
  const [value, setValue] = useState(settings.publicUrl ?? '');
  const [message, setMessage] = useState<string | null>(null);
  const name = t('admin.integrations.publicUrl.name');
  const save = useSettingsUpdate((body) => {
    if (body.clear) setValue('');
    setMessage(t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }));
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const env = settings.publicUrlSource === 'env';

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    save.mutate({ publicUrl: value.trim() });
  };

  return (
    <Section
      title={t('admin.integrations.publicUrl.title')}
      intro={t('admin.integrations.publicUrl.intro')}
      source={<SourceChip source={settings.publicUrlSource} />}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
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
          onChange={setValue}
          validate={(text) => {
            if (env) return null;
            if (!text.trim()) return t('errors.field.required');
            return /^https?:\/\/[^\s?#]+$/i.test(text.trim())
              ? null
              : t('errors.field.invalid_url');
          }}
        />
        {!env && (
          <Actions
            name={name}
            stored={settings.publicUrlSource === 'stored'}
            isPending={save.isPending}
            onClear={() => {
              setMessage(null);
              save.mutate({ clear: ['publicUrl'] });
            }}
          />
        )}
      </Form>
      <Outcome error={errors?.form ? save.error : null} message={message} />
    </Section>
  );
}

/**
 * Integrations (A-2, M7's key page): provider keys, the incoming webhooks of Radarr and Sonarr
 * (M9, I-8), the e-mail server with a test mail, the Telegram bot, the Web Push keys, the update
 * check and the public address. Secrets are write-only: a stored key
 * shows its last four characters at most; a value from deploy/.env cannot be changed here.
 */
export function IntegrationsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.integrations.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const settings = useQuery({ ...settingsQuery, enabled: allowed });

  return (
    <Page>
      <div className={table.header}>
        <Text as="h1" variant="display">
          {t('admin.integrations.title')}
        </Text>
        <Text tone="secondary">{t('admin.integrations.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.sections} aria-busy={settings.isPending}>
          {settings.isPending && <Skeleton shape="block" />}
          {settings.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, settings.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {settings.data && (
            <>
              <KeysSection keys={settings.data.keys} />
              <WebhooksSection />
              <EmailSection email={settings.data.email} />
              <TelegramSection telegram={settings.data.telegram} />
              <PushSection push={settings.data.push} />
              <UpdateSection enabled={settings.data.updateCheckEnabled} />
              <PublicUrlSection settings={settings.data} />
            </>
          )}
        </div>
      )}
    </Page>
  );
}
