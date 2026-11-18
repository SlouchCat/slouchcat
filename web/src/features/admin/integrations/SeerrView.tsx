import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useId, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { useTransientMutation } from '@/lib/useTransientMutation';
import {
  Chip,
  FactList,
  Notice,
  Skeleton,
  StatusDot,
  Text,
  TextLink,
  VisuallyHidden,
  type StatusDotState,
} from '@/ui';
import {
  afterDialogsClosed,
  afterRender,
  AriaButton,
  ConfirmDialog,
  DataTable,
  DisclosureGroup,
  focusLost,
  Form,
  SettingRow,
  Switch,
  StatusRegion,
  TextField,
  useStatusMessage,
  type Drafts,
} from '@/ui/aria';
import {
  confirmSeerrInstance,
  dashboardQuery,
  syncWishesNow,
  seerrKeys,
  seerrSettingsQuery,
  testSeerrConnection,
  updateSeerrSettings,
  wishKeys,
  type SeerrCheck,
  type SeerrSettings,
  type SeerrSettingsUpdate,
  type SeerrStatus,
  type SeerrTestResult,
} from '../queries';
import styles from './Integrations.module.css';
import { SeerrArrBand } from './SeerrArrBand';
import { SeerrHookBand } from './SeerrHookBand';
import {
  BandMeta,
  ClearButton,
  Outcome,
  SEERR_DRAFT,
  SEERR_KEY_DRAFT,
  seerrStateTone,
  seerrValues,
  type SeerrValues,
} from './shared';

/** States with something to fix: the band «Verbindung» opens with them (design rule 1.3). */
const OPEN_STATES: readonly SeerrStatus['state'][] = [
  'not_configured',
  'untested',
  'key_invalid',
  'unreachable',
  'instance_changed',
];

const checkDot = { ok: 'success', warn: 'warning', fail: 'danger' } as const satisfies Record<
  SeerrCheck['level'],
  StatusDotState
>;

/** The detail line of a state: the version, when Seerr last answered, or what to do. */
function stateDetail(t: TFunction, status: SeerrStatus, language: string): string | undefined {
  switch (status.state) {
    case 'connected':
      return status.version
        ? t('admin.integrations.seerr.version', { version: status.version })
        : undefined;
    case 'unreachable':
    case 'key_invalid':
      return status.lastSuccess
        ? t('admin.integrations.seerr.lastSuccess', {
            time: formatDateTime(status.lastSuccess, intlLocale(language)),
          })
        : undefined;
    default:
      return t(`admin.integrations.seerr.stateHint.${status.state}`);
  }
}

/**
 * The band «Abgleich» (concept 25, M-22): the open wishes, those waiting for the transfer, when
 * Seerr last answered, and «Jetzt abgleichen», which queues the sync at once; the job texts live
 * on «Automatisierung».
 */
function SyncBand({ status }: { status: SeerrStatus }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [message, say] = useStatusMessage();
  const sync = useMutation({
    mutationFn: syncWishesNow,
    onMutate: () => say(null),
    onSuccess: async () => {
      say(t('admin.wishes.requests.syncQueued'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: seerrKeys.all }),
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      ]);
    },
  });
  return (
    <div className={styles.stack}>
      <FactList
        facts={[
          [t('admin.integrations.seerr.sync.open'), String(status.counts.open)],
          [t('admin.integrations.seerr.sync.queued'), String(status.counts.queued)],
          [
            t('admin.integrations.seerr.sync.last'),
            status.lastSuccess
              ? formatDateTime(status.lastSuccess, intlLocale(i18n.language))
              : t('admin.integrations.seerr.sync.never'),
          ],
        ]}
      />
      <Text tone="secondary">{t('admin.integrations.seerr.sync.every')}</Text>
      <div className={styles.footer}>
        <AriaButton icon="refresh" isPending={sync.isPending} onPress={() => sync.mutate()}>
          {t('admin.wishes.requests.sync')}
        </AriaButton>
        <TextLink href={`${paths.adminWishes}?tab=anfragen`}>
          {t('admin.integrations.seerr.sync.requestsLink')}
        </TextLink>
      </div>
      <Outcome error={sync.error} message={message} />
    </div>
  );
}

/**
 * What only getSeerrStatus knows about the state, below the dashboard's state line of the view
 * frame (ServiceStatus): the version, when Seerr last answered, or what to do.
 */
function SeerrStateDetail({ status }: { status: SeerrStatus }) {
  const { t, i18n } = useTranslation();
  const detail = stateDetail(t, status, i18n.language);
  if (!detail) return null;
  return (
    <Text as="p" variant="caption" tone="secondary">
      {detail}
    </Text>
  );
}

/** What a check found, in words: the server sends a key, a level and a detail code. */
function checkText(t: TFunction, check: SeerrCheck): string {
  const { key, level, detail } = check;
  if (level === 'ok') {
    return t(`admin.integrations.seerr.checkOk.${key}`, {
      version: detail ?? '',
      count: Number(detail ?? 0),
    });
  }
  const notTested = t('admin.integrations.seerr.checkIssue.notTested');
  switch (key) {
    case 'reachable':
      if (detail === 'timeout') return t('admin.integrations.seerr.checkIssue.timeout');
      if (detail === 'refused') return t('admin.integrations.seerr.checkIssue.refused');
      if (detail === 'redirect') return t('admin.integrations.seerr.checkIssue.redirect');
      return t('admin.integrations.seerr.checkIssue.unreachable');
    case 'version':
      return t('admin.integrations.seerr.checkIssue.versionNotVerified');
    case 'key_valid':
      return detail === 'not_user_one'
        ? t('admin.integrations.seerr.checkIssue.keyNotUserOne')
        : t('admin.integrations.seerr.checkIssue.keyInvalid');
    case 'manage_users':
      return t('admin.integrations.seerr.checkIssue.manageUsers');
    case 'instance':
      return t('admin.integrations.seerr.checkIssue.instance');
    case 'act_as_user':
      if (detail === 'not_testable') return t('admin.integrations.seerr.checkIssue.notTestable');
      if (detail === 'instance_changed') return t('admin.integrations.seerr.checkIssue.waiting');
      return level === 'fail' ? t('admin.integrations.seerr.checkIssue.actAsUser') : notTested;
    case 'csrf_off':
      if (detail === 'unexpected_answer') {
        return t('admin.integrations.seerr.checkIssue.csrfUnexpected');
      }
      return level === 'fail' ? t('admin.integrations.seerr.checkIssue.csrfOn') : notTested;
    case 'radarr':
      return detail === '0' ? t('admin.integrations.seerr.checkIssue.noRadarr') : notTested;
    case 'sonarr':
      return detail === '0' ? t('admin.integrations.seerr.checkIssue.noSonarr') : notTested;
    case 'local_login':
      return detail === 'off' ? t('admin.integrations.seerr.checkIssue.localLoginOff') : notTested;
    case 'tvdb_seasons':
      if (detail === 'not_available')
        return t('admin.integrations.seerr.checkIssue.noSeasonSource');
      return detail === 'tmdb' ? t('admin.integrations.seerr.checkIssue.tmdbSeasons') : notTested;
    case 'password_transport':
      return t('admin.integrations.seerr.checkIssue.passwordTransport');
    case 'radarr_tags':
    case 'sonarr_tags':
      return detail === 'not_confirmed'
        ? t('admin.integrations.seerr.checkIssue.arrNotConfirmed')
        : t('admin.integrations.seerr.checkIssue.arrUnreachable');
  }
}

/** The check list of a test (architecture 15.1): one line per check point with its level. */
function CheckList({ result }: { result: SeerrTestResult }) {
  const { t } = useTranslation();
  return (
    <ul className={styles.checks} aria-label={t('admin.integrations.seerr.test.listLabel')}>
      {result.checks.map((check) => (
        <li key={check.key} className={styles.check}>
          <StatusDot state={checkDot[check.level]} />
          <span className={styles.checkText}>
            <span className={styles.checkLabel}>
              {t(`admin.integrations.seerr.checks.${check.key}`)}
              <VisuallyHidden>{`: ${t(`admin.integrations.seerr.level.${check.level}`)}`}</VisuallyHidden>
            </span>
            <span className={styles.checkDetail}>{checkText(t, check)}</span>
          </span>
        </li>
      ))}
    </ul>
  );
}

/** The summary above the check list: passed, passed with hints, or failed and why. */
function TestSummary({ result }: { result: SeerrTestResult }) {
  const { t } = useTranslation();
  const warnings = result.checks.filter((check) => check.level === 'warn').length;
  if (!result.ok) {
    const failed = result.checks
      .filter((check) => check.level === 'fail')
      .map((check) => t(`admin.integrations.seerr.checks.${check.key}`))
      .join(t('format.listSeparator'));
    return (
      <Notice tone="danger" title={t('admin.integrations.seerr.test.failed', { checks: failed })} />
    );
  }
  return (
    <Notice
      tone={warnings > 0 ? 'warning' : 'info'}
      title={
        warnings > 0
          ? t('admin.integrations.seerr.test.passedWithHints', { count: warnings })
          : t('admin.integrations.seerr.test.passed')
      }
    >
      {warnings > 0 ? t('admin.integrations.seerr.test.hintsHelp') : undefined}
    </Notice>
  );
}

/** The last 20 calls to Seerr: never a header, a body, a key or the host. */
function CallLog({ status }: { status: SeerrStatus }) {
  const { t, i18n } = useTranslation();
  if (status.log.length === 0) {
    return <Text tone="secondary">{t('admin.integrations.seerr.log.empty')}</Text>;
  }
  return (
    <div className={styles.stack}>
      <DataTable
        caption={t('admin.integrations.seerr.log.caption')}
        rows={status.log}
        rowKey={(entry) => `${entry.at}-${entry.method}-${entry.path}`}
        columns={[
          {
            key: 'at',
            header: t('admin.integrations.seerr.log.at'),
            rowHeader: true,
            cell: (entry) => formatDateTime(entry.at, intlLocale(i18n.language)),
          },
          {
            key: 'call',
            header: t('admin.integrations.seerr.log.call'),
            cell: (entry) => <code className={styles.call}>{`${entry.method} ${entry.path}`}</code>,
          },
          {
            key: 'result',
            header: t('admin.integrations.seerr.log.result'),
            cell: (entry) =>
              entry.status === undefined ? (
                <Chip tone="danger">{t('admin.integrations.seerr.log.noAnswer')}</Chip>
              ) : (
                <Chip tone={entry.status < 300 ? 'success' : 'danger'}>{String(entry.status)}</Chip>
              ),
          },
          {
            key: 'duration',
            header: t('admin.integrations.seerr.log.duration'),
            align: 'end',
            cell: (entry) => (
              <span className={styles.nowrap}>
                {t('admin.integrations.seerr.log.ms', { ms: entry.durationMs })}
              </span>
            ),
          },
          {
            key: 'error',
            header: t('admin.integrations.seerr.log.error'),
            cell: (entry) => entry.error ?? '–',
          },
        ]}
      />
      <Text variant="caption" tone="secondary">
        {t('admin.integrations.seerr.log.note')}
      </Text>
    </div>
  );
}

/**
 * «Andere Seerr-Instanz erkannt» (architecture 7.4): until the administrator decides, no call
 * runs on behalf of a user. «Zuordnung bestätigen» keeps every Seerr user of the accounts,
 * «Zuordnung löschen» removes them after a question.
 */
function InstanceNotice({ onDecided }: { onDecided: (message: string) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const decide = useMutation({
    mutationFn: confirmSeerrInstance,
    onSuccess: async (status, action) => {
      queryClient.setQueryData([...seerrKeys.all, 'status'], status);
      onDecided(
        t(
          action === 'confirm'
            ? 'admin.integrations.seerr.instance.confirmed'
            : 'admin.integrations.seerr.instance.resetDone',
        ),
      );
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      ]);
    },
  });
  return (
    <div className={styles.stack}>
      <Notice
        tone="danger"
        title={t('admin.integrations.seerr.instance.title')}
        actions={
          <>
            <AriaButton
              variant="primary"
              isPending={decide.isPending && decide.variables === 'confirm'}
              onPress={() => decide.mutate('confirm')}
            >
              {t('admin.integrations.seerr.instance.confirm')}
            </AriaButton>
            <ConfirmDialog
              trigger={
                <AriaButton variant="dangerQuiet">
                  {t('admin.integrations.seerr.instance.reset')}
                </AriaButton>
              }
              title={t('admin.integrations.seerr.instance.resetTitle')}
              body={t('admin.integrations.seerr.instance.resetBody')}
              confirmLabel={t('admin.integrations.seerr.instance.reset')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => decide.mutate('reset')}
            />
          </>
        }
      >
        {t('admin.integrations.seerr.instance.body')}
      </Notice>
      {decide.error ? <Notice tone="danger">{problemMessage(t, decide.error)}</Notice> : null}
    </div>
  );
}

/**
 * Integrationen › Seerr (concept 25, M-22, administration 4): the state line, the decision about
 * another instance, the band «Verbindung» (the switch «Seerr verwenden», address, write-only key;
 * «Verbindung testen» on the left, «Verwerfen», «Löschen …» and «Speichern» on the right, the
 * check list below), «Abgleich», and once an address is set the bands «Radarr und Sonarr»
 * and «Sofortmeldung» (stage 4), then «Protokoll». The bands start closed; «Verbindung» opens while
 * the connection needs something (design rule 1.3). Saving runs no test on the server, so the
 * view tests the saved values right after. Values from deploy/.env lock their field.
 */
export function SeerrView({
  settings,
  status,
  drafts,
  fields,
  arrDrafts,
  onFocusLost,
}: {
  settings: SeerrSettings;
  status: UseQueryResult<SeerrStatus>;
  drafts: Drafts<SeerrValues>;
  fields: Drafts<string>;
  /** The own addresses of Radarr and Sonarr typed in the band «Radarr und Sonarr». */
  arrDrafts: Drafts<string>;
  /** The element that held the focus is gone (the notice of another instance). */
  onFocusLost: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  // The outcome of a decision about another instance, outside the bands.
  const [decided, setDecided] = useStatusMessage();
  const saveRef = useRef<HTMLButtonElement>(null);
  const [formKey, setFormKey] = useState(0);
  const [result, setResult] = useState<SeerrTestResult | null>(null);
  const [connectionOpen, setConnectionOpen] = useState<boolean>();
  const keyHelpId = useId();
  const values = drafts.valueOf(SEERR_DRAFT) ?? seerrValues(settings);
  const apiKey = fields.valueOf(SEERR_KEY_DRAFT) ?? '';
  const dirty = drafts.isDirty(SEERR_DRAFT) || apiKey !== '';
  const name = t('admin.integrations.seerr.name');
  const urlEnv = settings.urlSource === 'env';
  const keyEnv = settings.apiKey.source === 'env';
  const stored = settings.urlSource === 'stored' || settings.apiKey.source === 'stored';

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: seerrKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: wishKeys.all }),
    ]);

  // The key goes out with a test or a save and never stays in the query client.
  const test = useTransientMutation(testSeerrConnection, async (next) => {
    setResult(next);
    await refresh();
  });
  const save = useTransientMutation(
    updateSeerrSettings,
    async (next: SeerrSettings, body: SeerrSettingsUpdate) => {
      queryClient.setQueryData(seerrSettingsQuery.queryKey, next);
      if (body.clear) {
        drafts.discard(SEERR_DRAFT);
        fields.discard(SEERR_KEY_DRAFT);
        setResult(null);
      }
      setMessage(
        t(body.clear ? 'admin.integrations.cleared' : 'admin.integrations.saved', { name }),
      );
      await refresh();
    },
  );
  const failed = save.error ?? test.error;
  const errors = failed ? formErrors(t, failed) : null;

  const runTest = (body: { url?: string; apiKey?: string }) => {
    setMessage(null);
    save.clearError();
    setResult(null);
    return test.run(body);
  };

  const onTest = () => {
    void runTest({
      ...(drafts.isDirty(SEERR_DRAFT) && values.url.trim() !== (settings.url ?? '')
        ? { url: values.url.trim() }
        : {}),
      ...(apiKey.trim() ? { apiKey: apiKey.trim() } : {}),
    });
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    test.clearError();
    // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
    if (!dirty) return;
    const sent = values;
    const sentKey = apiKey;
    const body: SeerrSettingsUpdate = {
      ...(values.enabled !== settings.enabled ? { enabled: values.enabled } : {}),
      ...(!urlEnv && values.url.trim() !== (settings.url ?? '') ? { url: values.url.trim() } : {}),
      ...(!keyEnv && apiKey.trim() ? { apiKey: apiKey.trim() } : {}),
    };
    void save.run(body).then((outcome) => {
      if (!outcome.ok) return;
      // What was typed while the save ran stays a draft (decision 18, addendum 3).
      drafts.set(SEERR_DRAFT, (current) =>
        JSON.stringify(current) === JSON.stringify(sent) ? seerrValues(outcome.data) : current,
      );
      fields.set(SEERR_KEY_DRAFT, (current) => (current === sentKey ? '' : current));
      // Saving runs no test: the saved values are tested right after, which «Wünsche aktiv» needs.
      if (outcome.data.url && outcome.data.apiKey.source !== 'none') void test.run({});
    });
  };

  const onDiscard = () => {
    setMessage(null);
    save.clearError();
    test.clearError();
    drafts.discard(SEERR_DRAFT);
    fields.discard(SEERR_KEY_DRAFT);
    setFormKey((key) => key + 1);
    void afterRender().then(() => saveRef.current?.focus());
  };

  // «Löschen …» disappears with the connection it deletes: the focus moves on to «Speichern».
  const onClear = async () => {
    setMessage(null);
    await save.run({ clear: true });
    await afterDialogsClosed();
    if (focusLost()) saveRef.current?.focus();
  };

  const state = status.data?.state;
  // Open at first while the connection needs something; later states never fold it on their own,
  // so the check list of a save stays in view.
  if (connectionOpen === undefined && state !== undefined) {
    setConnectionOpen(OPEN_STATES.includes(state));
  }
  const keyPlaceholder =
    settings.apiKey.source === 'stored' && settings.apiKey.hint
      ? t('admin.integrations.source.storedHint', { hint: settings.apiKey.hint })
      : undefined;
  const errorCount = status.data?.log.filter(
    (entry) => entry.error || (entry.status ?? 0) >= 300,
  ).length;

  return (
    <div className={styles.stack}>
      {status.isPending && <Skeleton shape="block" />}
      {status.isError && <Notice tone="danger" title={problemMessage(t, status.error)} />}
      {status.data && <SeerrStateDetail status={status.data} />}
      {status.data?.instanceChanged && (
        <InstanceNotice
          onDecided={(text) => {
            setDecided(text);
            void afterDialogsClosed().then(() => {
              if (focusLost()) onFocusLost();
            });
          }}
        />
      )}
      <StatusRegion message={decided} />
      <DisclosureGroup
        title={t('admin.integrations.seerr.connection.title')}
        isExpanded={connectionOpen ?? false}
        onExpandedChange={setConnectionOpen}
        meta={
          <>
            <BandMeta unsaved={dirty} />
            <span className={styles.clipMeta}>
              {settings.url ?? t('admin.integrations.seerr.connection.noAddress')}
            </span>
            {state && (
              <Chip tone={seerrStateTone[state]}>
                {t(`admin.integrations.seerr.state.${state}`)}
              </Chip>
            )}
          </>
        }
      >
        <div className={styles.stack}>
          <Form key={formKey} onSubmit={onSubmit} validationErrors={errors?.fields}>
            <SettingRow
              label={t('admin.integrations.seerr.enabled')}
              help={t('admin.integrations.seerr.enabledHelp')}
            >
              <Switch
                name="enabled"
                isSelected={values.enabled}
                onChange={(enabled) =>
                  drafts.set(SEERR_DRAFT, (current) => ({ ...current, enabled }))
                }
              >
                <VisuallyHidden>{t('admin.integrations.seerr.enabled')}</VisuallyHidden>
              </Switch>
            </SettingRow>
            <SettingRow
              label={t('admin.integrations.seerr.url')}
              help={t('admin.integrations.seerr.urlHelp')}
            >
              <div className={styles.stack}>
                <TextField
                  name="url"
                  type="url"
                  label={t('admin.integrations.seerr.url')}
                  hideLabel
                  inputMode="url"
                  autoComplete="off"
                  spellCheck="false"
                  placeholder={t('admin.integrations.seerr.urlPlaceholder')}
                  description={urlEnv ? t('admin.integrations.envHint') : undefined}
                  isDisabled={urlEnv}
                  value={values.url}
                  onChange={(url) => drafts.set(SEERR_DRAFT, (current) => ({ ...current, url }))}
                />
              </div>
            </SettingRow>
            <SettingRow
              label={t('admin.integrations.seerr.apiKey')}
              help={<span id={keyHelpId}>{t('admin.integrations.seerr.apiKeyHelp')}</span>}
            >
              <div className={styles.stack}>
                <TextField
                  name="apiKey"
                  type="password"
                  label={t('admin.integrations.seerr.apiKey')}
                  hideLabel
                  mono
                  autoComplete="off"
                  spellCheck="false"
                  aria-describedby={keyHelpId}
                  placeholder={keyPlaceholder}
                  description={keyEnv ? t('admin.integrations.envHint') : undefined}
                  isDisabled={keyEnv}
                  value={apiKey}
                  onChange={(text) => fields.set(SEERR_KEY_DRAFT, text)}
                />
              </div>
            </SettingRow>
            <div className={styles.footer}>
              <AriaButton icon="check" isPending={test.isPending} onPress={onTest}>
                {t('admin.integrations.seerr.test.action')}
              </AriaButton>
              <div className={styles.footerEnd}>
                {dirty && (
                  <AriaButton variant="ghost" isDisabled={save.isPending} onPress={onDiscard}>
                    {t('admin.integrations.discard')}
                  </AriaButton>
                )}
                {stored && !urlEnv && !keyEnv && (
                  <ClearButton
                    name={name}
                    body={t('admin.integrations.seerr.clearBody')}
                    onClear={() => void onClear()}
                  />
                )}
                <AriaButton
                  ref={saveRef}
                  type="submit"
                  variant="primary"
                  isPending={save.isPending}
                  aria-label={t('admin.integrations.saveLabel', { name })}
                >
                  {save.isPending ? t('admin.integrations.saving') : t('admin.integrations.save')}
                </AriaButton>
              </div>
            </div>
          </Form>
          {test.isPending && (
            <Text tone="secondary">{t('admin.integrations.seerr.test.running')}</Text>
          )}
          {result && (
            <div className={styles.stack}>
              <TestSummary result={result} />
              <CheckList result={result} />
            </div>
          )}
          <Outcome error={errors?.form ? failed : null} message={message} />
        </div>
      </DisclosureGroup>
      <DisclosureGroup
        title={t('admin.integrations.seerr.sync.title')}
        meta={
          status.data && (
            <span>
              {t('admin.integrations.seerr.sync.summary', { count: status.data.counts.open })}
            </span>
          )
        }
      >
        {status.data && <SyncBand status={status.data} />}
      </DisclosureGroup>
      {settings.urlSource !== 'none' && <SeerrArrBand drafts={arrDrafts} />}
      {settings.urlSource !== 'none' && <SeerrHookBand />}
      <DisclosureGroup
        title={t('admin.integrations.seerr.log.title')}
        meta={
          status.data && (
            <span>
              {t('admin.integrations.seerr.log.summary', {
                count: status.data.log.length,
                errors: errorCount ?? 0,
              })}
            </span>
          )
        }
      >
        {status.data && <CallLog status={status.data} />}
      </DisclosureGroup>
      <TextLink href={`${paths.adminWishes}?tab=seerr-benutzer`}>
        {t('admin.integrations.seerr.usersLink')}
      </TextLink>
    </div>
  );
}
