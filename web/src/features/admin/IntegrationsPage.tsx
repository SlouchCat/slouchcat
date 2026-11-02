import type {
  AdminSettings,
  AdminSettingsUpdate,
  KeyState,
  SettingSource,
  SMTPTLS,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { sessionQuery, useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  Form,
  RadioGroup,
  SettingRow,
  TextField,
  useDrafts,
  useUnsavedChanges,
  type Drafts,
} from '@/ui/aria';
import {
  dashboardQuery,
  integrationsQuery,
  rotatePushKeys,
  sendTestEmail,
  settingsQuery,
  updateSettings,
} from './queries';
import { WebhooksSection, type PathMapDraft } from './WebhooksSection';
import styles from './IntegrationsPage.module.css';
import table from './AdminTablePage.module.css';

type KeyName = keyof AdminSettings['keys'];

const KEYS = [
  'tmdb',
  'tvdb',
  'tvdbPin',
  'youtube',
  'fanart',
  'fanartClient',
  'trakt',
] as const satisfies readonly KeyName[];

const TLS_MODES = ['starttls', 'tls', 'none'] as const satisfies readonly SMTPTLS[];

const sourceTone = { env: 'info', stored: 'success', none: 'outline' } as const;

/** Draft ids of the single text values: a key, the bot token, the SMTP password, the address. */
const keyDraft = (keyName: KeyName) => `keys.${keyName}`;
const TELEGRAM_DRAFT = 'telegram';
const PASSWORD_DRAFT = 'email.password';
const PUBLIC_URL_DRAFT = 'publicUrl';
const EMAIL_DRAFT = 'email';

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

/** The band's state: where the block's value comes from, and whether it holds unsaved input. */
function BandMeta({ source, unsaved }: { source?: SettingSource; unsaved: boolean }) {
  const { t } = useTranslation();
  if (!source && !unsaved) return null;
  return (
    <>
      {unsaved && <Chip tone="warning">{t('admin.integrations.unsaved')}</Chip>}
      {source && <SourceChip source={source} />}
    </>
  );
}

/** The outcome of a block's last action: an error notice or a polite status line. */
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
function KeyRow({
  keyName,
  state,
  drafts,
}: {
  keyName: KeyName;
  state: KeyState;
  drafts: Drafts<string>;
}) {
  const { t } = useTranslation();
  const [message, setMessage] = useState<string | null>(null);
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
function EmailForm({
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
  const [message, setMessage] = useState<string | null>(null);
  const values = drafts.valueOf(EMAIL_DRAFT) ?? emailValues(email);
  const password = fields.valueOf(PASSWORD_DRAFT) ?? '';
  const name = t('admin.integrations.email.name');
  const save = useSecretUpdate((body) => {
    drafts.discard(EMAIL_DRAFT);
    fields.discard(PASSWORD_DRAFT);
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
    <>
      <Text tone="secondary">{t('admin.integrations.email.intro')}</Text>
      {/* The channels each profile uses are set on their own page; it links back here (BEN-1). */}
      <TextLink href={paths.notificationSettings}>
        {t('admin.integrations.notificationsLink')}
      </TextLink>
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

/** The Telegram bot: write-only token (checked by the server with getMe) and its name. */
function TelegramForm({
  telegram,
  fields,
}: {
  telegram: AdminSettings['telegram'];
  fields: Drafts<string>;
}) {
  const { t } = useTranslation();
  const [message, setMessage] = useState<string | null>(null);
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

/** Web Push: whether a VAPID pair exists, its public key, and a new pair after a confirmation. */
function PushBlock({ push }: { push: AdminSettings['push'] }) {
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
    <>
      <Text tone="secondary">{t('admin.integrations.push.intro')}</Text>
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
    </>
  );
}

/** The daily update check (A-5): a switch that is stored at once. */
function UpdateRow({ enabled }: { enabled: boolean }) {
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
  const [message, setMessage] = useState<string | null>(null);
  const value = fields.valueOf(PUBLIC_URL_DRAFT) ?? '';
  const name = t('admin.integrations.publicUrl.name');
  const save = useSettingsUpdate((body) => {
    fields.discard(PUBLIC_URL_DRAFT);
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
                save.mutate({ clear: ['publicUrl'] });
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

/**
 * Integrations (A-2, M7's key page; concept 12, variant A): six blocks that fold away by their
 * band (provider keys, the incoming webhooks of Radarr and Sonarr, the e-mail server with a test
 * mail, the Telegram bot, the Web Push keys, and the server's update check and public address).
 * Every block starts open; unsaved input lives in the page (useDrafts), so folding a block or a
 * webhook away keeps it, and leaving the page with it asks first (S-13, S-17). Secrets are
 * write-only: a stored key shows its last four characters at most; a value from deploy/.env
 * cannot be changed here.
 */
export function IntegrationsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.integrations.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const settings = useQuery({ ...settingsQuery, enabled: allowed });
  const integrations = useQuery({ ...integrationsQuery, enabled: allowed });
  const titleRef = useRef<HTMLHeadingElement>(null);

  // The originals of the drafts: secrets are write-only, so their original is empty.
  const data = settings.data;
  const fields = useDrafts<string>(
    useCallback(
      (id: string) => (data ? (id === PUBLIC_URL_DRAFT ? (data.publicUrl ?? '') : '') : undefined),
      [data],
    ),
  );
  const email = useDrafts<EmailValues>(
    useCallback(
      (id: string) => (data && id === EMAIL_DRAFT ? emailValues(data.email) : undefined),
      [data],
    ),
  );
  const list = integrations.data;
  const pathMaps = useDrafts<PathMapDraft>(
    useCallback(
      (id: string) => {
        const integration = list?.find((entry) => String(entry.id) === id);
        return integration && { base: integration.pathMap, rows: integration.pathMap };
      },
      [list],
    ),
  );
  const unsaved = useUnsavedChanges(fields.dirty || email.dirty || pathMaps.dirty);
  const keysDirty = KEYS.some((keyName) => fields.isDirty(keyDraft(keyName)));

  return (
    <AdminPage
      title={t('admin.integrations.title')}
      help={t('admin.integrations.intro')}
      titleRef={titleRef}
    >
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.blocks} aria-busy={settings.isPending}>
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
          {data && (
            <>
              <BandPanel
                collapsible
                icon="lock"
                title={t('admin.integrations.keys.title')}
                meta={<BandMeta unsaved={keysDirty} />}
              >
                <Text tone="secondary">{t('admin.integrations.keys.intro')}</Text>
                <div>
                  {KEYS.map((keyName) => (
                    <KeyRow
                      key={keyName}
                      keyName={keyName}
                      state={data.keys[keyName]}
                      drafts={fields}
                    />
                  ))}
                </div>
              </BandPanel>
              <BandPanel
                collapsible
                icon="plug"
                title={t('admin.integrations.webhooks.title')}
                meta={<BandMeta unsaved={pathMaps.dirty} />}
              >
                <WebhooksSection drafts={pathMaps} onRemoved={() => titleRef.current?.focus()} />
              </BandPanel>
              <BandPanel
                collapsible
                icon="megaphone"
                title={t('admin.integrations.email.title')}
                meta={
                  <BandMeta
                    source={data.email.source}
                    unsaved={email.dirty || fields.isDirty(PASSWORD_DRAFT)}
                  />
                }
              >
                <EmailForm email={data.email} drafts={email} fields={fields} />
              </BandPanel>
              <BandPanel
                collapsible
                icon="external"
                title={t('admin.integrations.telegram.title')}
                meta={
                  <BandMeta
                    source={data.telegram.source}
                    unsaved={fields.isDirty(TELEGRAM_DRAFT)}
                  />
                }
              >
                <TelegramForm telegram={data.telegram} fields={fields} />
              </BandPanel>
              <BandPanel collapsible icon="bell" title={t('admin.integrations.push.title')}>
                <PushBlock push={data.push} />
              </BandPanel>
              <BandPanel
                collapsible
                icon="server"
                title={t('admin.integrations.server.title')}
                meta={<BandMeta unsaved={fields.isDirty(PUBLIC_URL_DRAFT)} />}
              >
                <div>
                  <UpdateRow enabled={data.updateCheckEnabled} />
                  <PublicUrlRow settings={data} fields={fields} />
                </div>
              </BandPanel>
            </>
          )}
        </div>
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
