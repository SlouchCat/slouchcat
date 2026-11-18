import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { Chip, FactList, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  afterRender,
  AriaButton,
  DisclosureGroup,
  Form,
  SettingRow,
  TextField,
  useStatusMessage,
  type Drafts,
} from '@/ui/aria';
import {
  arrStatusQuery,
  dashboardQuery,
  seerrKeys,
  testArrConnection,
  updateArrSettings,
  wishSettingsQuery,
  type ArrService,
  type ArrServiceStatus,
  type ArrSettingsUpdate,
  type ArrStatus,
  type ArrTestResult,
} from '../queries';
import { arrErrorText, serviceName } from '../wishes/progress';
import styles from './Integrations.module.css';
import { BandMeta, Outcome } from './shared';

const SERVICES = ['radarr', 'sonarr'] as const satisfies readonly ArrService[];

/** How a service stands for the jobs: no server in Seerr, its host not confirmed, the last call. */
type ArrState = 'none' | 'unconfirmed' | 'unreachable' | 'reachable' | 'untested';

function arrState(service: ArrServiceStatus): ArrState {
  if (service.source === 'none') return 'none';
  if (!service.confirmed) return 'unconfirmed';
  if (service.reachable === undefined) return 'untested';
  return service.reachable ? 'reachable' : 'unreachable';
}

const stateTone = {
  none: 'outline',
  unconfirmed: 'warning',
  unreachable: 'danger',
  reachable: 'success',
  untested: 'neutral',
} as const satisfies Record<ArrState, string>;

/** Something to fix: the band opens with it (design rule 1.3). */
const needsCare = (status: ArrStatus) =>
  SERVICES.some((service) => {
    const state = arrState(status[service]);
    return state === 'unconfirmed' || state === 'unreachable';
  });

/** The own address a draft starts from: the stored one, else empty (the address from Seerr). */
export const ownAddress = (service: ArrServiceStatus) =>
  service.source === 'own' ? (service.address ?? '') : '';

/** The facts of one service; never a key. */
function serviceFacts(
  t: TFunction,
  service: ArrService,
  status: ArrServiceStatus,
  language: string,
): (readonly [string, ReactNode])[] {
  const name = serviceName(t, service);
  const state = arrState(status);
  const address =
    status.source === 'none' || !status.address
      ? t('admin.integrations.seerr.arr.addressNone', { service: name })
      : t(`admin.integrations.seerr.arr.address.${status.source}`, { address: status.address });
  const when = status.lastAt ? formatDateTime(status.lastAt, language) : undefined;
  const reason = status.lastError ? arrErrorText(t, status.lastError, service) : undefined;
  const progress = status.progress;
  return [
    [t('admin.integrations.seerr.arr.facts.address'), address],
    [
      t('admin.integrations.seerr.arr.facts.state'),
      <span key="state" className={styles.factLine}>
        <Chip tone={stateTone[state]}>
          {t(`admin.integrations.seerr.arr.state.${state}`, { service: name })}
        </Chip>
        {reason && <span>{reason}</span>}
        {when && (
          <span className={styles.factDetail}>
            {t('admin.integrations.seerr.arr.lastAt', { time: when })}
          </span>
        )}
      </span>,
    ],
    ...(status.tagRequests === undefined
      ? []
      : ([
          [
            t('admin.integrations.seerr.arr.facts.tagRequests'),
            t(
              status.tagRequests
                ? 'admin.integrations.seerr.arr.tagRequestsOn'
                : 'admin.integrations.seerr.arr.tagRequestsOff',
            ),
          ],
        ] as const)),
    [
      t('admin.integrations.seerr.arr.facts.tags'),
      t('admin.integrations.seerr.arr.tags', status.counts),
    ],
    [
      t('admin.integrations.seerr.arr.facts.progress'),
      !progress.enabled
        ? t('admin.integrations.seerr.arr.progressOff')
        : progress.error
          ? t('admin.integrations.seerr.arr.progressError', {
              reason: arrErrorText(t, progress.error, service),
            })
          : progress.lastReadAt
            ? t('admin.integrations.seerr.arr.progressAt', {
                count: progress.tracked,
                time: formatDateTime(progress.lastReadAt, language),
              })
            : t('admin.integrations.seerr.arr.progressOn', { count: progress.tracked }),
    ],
  ];
}

/** The outcome of «Testen» in words: confirmed, the key refused, out of reach, or not in Seerr. */
function TestOutcome({ service, result }: { service: ArrService; result: ArrTestResult }) {
  const { t } = useTranslation();
  const name = serviceName(t, service);
  const host = result.host ?? t('admin.integrations.seerr.arr.test.noHost');
  if (result.error === 'not_in_seerr') {
    return (
      <Notice
        tone="warning"
        title={t('admin.integrations.seerr.arr.test.notInSeerr', { service: name })}
      >
        {t('admin.integrations.seerr.arr.test.notInSeerrBody')}
      </Notice>
    );
  }
  if (result.reachable && result.keyValid) {
    return (
      <Notice
        tone="info"
        title={t('admin.integrations.seerr.arr.test.passed', { service: name, host })}
      >
        {result.tagRequests === undefined
          ? undefined
          : t(
              result.tagRequests
                ? 'admin.integrations.seerr.arr.tagRequestsOn'
                : 'admin.integrations.seerr.arr.tagRequestsOff',
            )}
      </Notice>
    );
  }
  return (
    <Notice
      tone="danger"
      title={t(
        result.reachable
          ? 'admin.integrations.seerr.arr.test.keyRefused'
          : 'admin.integrations.seerr.arr.test.unreachable',
        { service: name, host },
      )}
    >
      {[
        ...(result.error ? [arrErrorText(t, result.error, service)] : []),
        t('admin.integrations.seerr.arr.test.ownHint'),
      ].join(' ')}
    </Notice>
  );
}

/** One service: its facts, «Testen» and the outcome. */
function ServiceBlock({
  service,
  status,
  result,
  testing,
  error,
  onTest,
}: {
  service: ArrService;
  status: ArrServiceStatus;
  result: ArrTestResult | undefined;
  testing: boolean;
  /** Why the last «Testen» of this service failed. */
  error: unknown;
  onTest: () => void;
}) {
  const { t, i18n } = useTranslation();
  const name = serviceName(t, service);
  return (
    <section className={styles.arrService} aria-labelledby={`arr-${service}`}>
      <h4 id={`arr-${service}`} className={styles.itemTitle}>
        {name}
      </h4>
      <FactList facts={serviceFacts(t, service, status, intlLocale(i18n.language))} />
      <div className={styles.actions}>
        <AriaButton
          icon="check"
          isPending={testing}
          aria-label={t('admin.integrations.seerr.arr.test.label', { service: name })}
          onPress={onTest}
        >
          {t('admin.integrations.seerr.arr.test.action')}
        </AriaButton>
      </div>
      {result && <TestOutcome service={service} result={result} />}
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
    </section>
  );
}

/**
 * The band «Radarr und Sonarr (Profil-Tags und Fortschritt)» of Integrationen › Seerr (concept 25
 * stage 4, M-22, administration 4.8): per service where its address comes from (Seerr, an own
 * one, none), whether the jobs may call it, «Testen» with its outcome (which confirms the host),
 * «Tag Requests» in Seerr, the tag counts and the state of the progress reads; below, the own
 * addresses, saved with the band's own «Speichern» (empty: the address from Seerr). The key
 * always comes from Seerr and never shows. Closed at first, open while a service needs «Testen»
 * or did not answer.
 */
export function SeerrArrBand({ drafts }: { drafts: Drafts<string> }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const arr = useQuery(arrStatusQuery);
  const settings = useQuery(wishSettingsQuery);
  const [open, setOpen] = useState<boolean>();
  const [results, setResults] = useState<Partial<Record<ArrService, ArrTestResult>>>({});
  const [message, setMessage] = useStatusMessage();
  const [formKey, setFormKey] = useState(0);
  const saveRef = useRef<HTMLButtonElement>(null);
  const status = arr.data;
  // Opens whenever a service newly needs care (waits for «Testen» or did not answer); the reader
  // may close it again.
  const care = status !== undefined && needsCare(status);
  const [cared, setCared] = useState(false);
  if (care !== cared) {
    setCared(care);
    if (care) setOpen(true);
  }
  if (open === undefined && status !== undefined) setOpen(care);

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: seerrKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
  // One «Testen» per service: the second test neither hides the first one's spinner nor its error.
  const testOf = (service: ArrService) => ({
    mutationFn: () => testArrConnection(service),
    onMutate: () => {
      setMessage(null);
      setResults((current) => ({ ...current, [service]: undefined }));
    },
    onSuccess: async (result: ArrTestResult) => {
      setResults((current) => ({ ...current, [service]: result }));
      await refresh();
    },
  });
  const tests = { radarr: useMutation(testOf('radarr')), sonarr: useMutation(testOf('sonarr')) };
  const save = useMutation({
    mutationFn: updateArrSettings,
    onMutate: () => setMessage(null),
    onSuccess: async (next, body) => {
      queryClient.setQueryData(arrStatusQuery.queryKey, next);
      // What was typed while the save ran stays a draft (as in the band «Verbindung»).
      for (const service of SERVICES) {
        const sent = body[`${service}Url`];
        if (sent !== undefined && (drafts.valueOf(service) ?? '').trim() === sent) {
          drafts.discard(service);
        }
      }
      setMessage(t('admin.integrations.seerr.arr.saved'));
      await refresh();
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const dirty = SERVICES.some((service) => drafts.isDirty(service));

  const valueOf = (service: ArrService) =>
    drafts.valueOf(service) ?? (status ? ownAddress(status[service]) : '');

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!status || !dirty) return;
    const body: ArrSettingsUpdate = {};
    for (const service of SERVICES) {
      if (drafts.isDirty(service)) body[`${service}Url`] = valueOf(service).trim();
    }
    save.mutate(body);
  };
  const onDiscard = () => {
    setMessage(null);
    save.reset();
    for (const service of SERVICES) drafts.discard(service);
    setFormKey((key) => key + 1);
    void afterRender().then(() => saveRef.current?.focus());
  };

  const off = settings.data && !settings.data.profileTags && !settings.data.showProgress;
  const open$ = status
    ? SERVICES.reduce(
        (sum, service) => sum + status[service].counts.pending + status[service].counts.failed,
        0,
      )
    : 0;
  const tracked = status
    ? SERVICES.reduce((sum, service) => sum + status[service].progress.tracked, 0)
    : 0;

  return (
    <DisclosureGroup
      title={t('admin.integrations.seerr.arr.title')}
      isExpanded={open ?? false}
      onExpandedChange={setOpen}
      meta={
        <>
          <BandMeta unsaved={dirty} />
          {off ? (
            <span>{t('admin.integrations.seerr.arr.summaryOff')}</span>
          ) : (
            status && (
              <span>
                {[
                  ...SERVICES.map((service) =>
                    t(`admin.integrations.seerr.arr.state.${arrState(status[service])}`, {
                      service: serviceName(t, service),
                    }),
                  ),
                  t('admin.integrations.seerr.arr.summaryTags', { count: open$ }),
                  t('admin.integrations.seerr.arr.summaryProgress', { count: tracked }),
                ].join(t('format.separator'))}
              </span>
            )
          )}
        </>
      }
    >
      <div className={styles.stack}>
        <Text tone="secondary">{t('admin.integrations.seerr.arr.intro')}</Text>
        {off && <Notice tone="info">{t('admin.integrations.seerr.arr.offNotice')}</Notice>}
        {arr.isPending && <Skeleton shape="block" />}
        {arr.isError && (
          <Notice
            tone="danger"
            title={problemMessage(t, arr.error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void arr.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {status && (
          <>
            <div className={styles.arrServices}>
              {SERVICES.map((service) => (
                <ServiceBlock
                  key={service}
                  service={service}
                  status={status[service]}
                  result={results[service]}
                  testing={tests[service].isPending}
                  error={tests[service].error}
                  onTest={() => tests[service].mutate()}
                />
              ))}
            </div>
            <Form key={formKey} onSubmit={onSubmit} validationErrors={errors?.fields}>
              {SERVICES.map((service) => {
                const name = serviceName(t, service);
                return (
                  <SettingRow
                    key={service}
                    label={t('admin.integrations.seerr.arr.own', { service: name })}
                    help={t('admin.integrations.seerr.arr.ownHelp')}
                  >
                    <TextField
                      name={`${service}Url`}
                      type="url"
                      label={t('admin.integrations.seerr.arr.own', { service: name })}
                      hideLabel
                      inputMode="url"
                      autoComplete="off"
                      spellCheck="false"
                      placeholder={t('admin.integrations.seerr.arr.ownPlaceholder')}
                      value={valueOf(service)}
                      onChange={(text) => drafts.set(service, text)}
                    />
                  </SettingRow>
                );
              })}
              <div className={styles.footer}>
                <div className={styles.footerEnd}>
                  {dirty && (
                    <AriaButton variant="ghost" isDisabled={save.isPending} onPress={onDiscard}>
                      {t('admin.integrations.discard')}
                    </AriaButton>
                  )}
                  <AriaButton
                    ref={saveRef}
                    type="submit"
                    variant="primary"
                    isPending={save.isPending}
                    aria-label={t('admin.integrations.seerr.arr.saveLabel')}
                  >
                    {save.isPending ? t('admin.integrations.saving') : t('admin.integrations.save')}
                  </AriaButton>
                </div>
              </div>
            </Form>
            <Outcome error={errors?.form ? save.error : null} message={message} />
          </>
        )}
        <ul className={styles.links}>
          <li>
            <TextLink href={`${paths.adminWishes}?tab=einstellungen`}>
              {t('admin.integrations.seerr.arr.toSettings')}
            </TextLink>
          </li>
          <li>
            <TextLink href={`${paths.adminWishes}?tab=seerr-benutzer`}>
              {t('admin.integrations.seerr.arr.toUsers')}
            </TextLink>
          </li>
          <li>
            <TextLink href={`${paths.adminIntegrations}?item=webhooks`}>
              {t('admin.integrations.seerr.arr.toWebhooks')}
            </TextLink>
          </li>
        </ul>
      </div>
    </DisclosureGroup>
  );
}
