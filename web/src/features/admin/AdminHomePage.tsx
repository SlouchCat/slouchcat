import type {
  Dashboard,
  ErrorEntry,
  IntegrationStatus,
  JobSummary,
  StorageInfo,
  SystemInfo,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useRef, useState, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatBytes, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Notice,
  Page,
  Panel,
  Skeleton,
  StatusDot,
  Text,
  TextLink,
  VisuallyHidden,
  type StatusDotState,
} from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  LinkCard,
  StatTile,
  StorageMeter,
  type DataColumn,
} from '@/ui/aria';
import { SystemState } from '../shared/SystemState';
import { cancelJob, DASHBOARD_POLL_MS, dashboardQuery, retryJob, systemQuery } from './queries';
import { adminGroups, adminSections } from './sections';
import { streamMode } from './StreamsPage';
import styles from './AdminHomePage.module.css';
import table from './AdminTablePage.module.css';

/** The dot of an integration's state; the text next to it carries the meaning. */
const integrationDot = {
  configured: 'success',
  missing: 'pending',
  error: 'danger',
  disabled: 'pending',
} as const satisfies Record<IntegrationStatus['state'], StatusDotState>;

/** Streams shown on the dashboard; the streams page lists them all. */
const STREAMS_SHOWN = 5;

/** A tile of the dashboard: a panel with a heading. */
function Tile({
  title,
  children,
  wide = false,
  headingRef,
}: {
  title: string;
  children: ReactNode;
  wide?: boolean;
  headingRef?: Ref<HTMLHeadingElement>;
}) {
  const id = useId();
  return (
    <div className={wide ? styles.wide : undefined}>
      <Panel as="section" aria-labelledby={id}>
        <h3
          id={id}
          ref={headingRef}
          tabIndex={headingRef ? -1 : undefined}
          className={styles.tileTitle}
        >
          {title}
        </h3>
        {children}
      </Panel>
    </div>
  );
}

function StreamsTile({ streams }: { streams: Dashboard['streams'] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  return (
    <Tile title={t('admin.home.streams.title')}>
      <StatTile
        label={t('admin.home.streams.count')}
        value={number.format(streams.length)}
        tone={streams.length > 0 ? 'success' : 'neutral'}
      />
      {streams.length === 0 ? (
        <Text tone="secondary">{t('admin.home.streams.empty')}</Text>
      ) : (
        <ul className={styles.list}>
          {streams.slice(0, STREAMS_SHOWN).map((stream) => (
            <li key={stream.id}>
              {t('admin.home.streams.entry', {
                profile: stream.profileName,
                title: stream.itemTitle,
              })}
              {t('format.separator')}
              {streamMode(t, stream)}
            </li>
          ))}
        </ul>
      )}
      <TextLink href={paths.adminStreams}>{t('admin.home.streams.all')}</TextLink>
    </Tile>
  );
}

function StorageTile({ storage }: { storage: Dashboard['storage'] }) {
  const { t, i18n } = useTranslation();
  const bytes = (value: number) => formatBytes(value, i18n.language);
  const meter = (info: StorageInfo, label: string) => {
    const used = Math.max(0, info.totalBytes - info.freeBytes);
    return (
      <div className={styles.meter}>
        <Text as="span" variant="caption" tone="secondary">
          {label}
        </Text>
        <StorageMeter
          label={label}
          valueText={t('admin.home.storage.used', {
            used: bytes(used),
            total: bytes(info.totalBytes),
          })}
          total={info.totalBytes}
          parts={[
            {
              key: 'used',
              value: used,
              label: t('admin.home.storage.part', { size: bytes(used) }),
              tone: 'accent',
            },
          ]}
          caption={t('admin.home.storage.free', {
            free: bytes(info.freeBytes),
            total: bytes(info.totalBytes),
          })}
        />
      </div>
    );
  };
  return (
    <Tile title={t('admin.home.storage.title')}>
      {meter(storage.config, t('admin.home.storage.config', { path: storage.config.path }))}
      {meter(storage.cache, t('admin.home.storage.cache', { path: storage.cache.path }))}
    </Tile>
  );
}

function IntegrationsTile({ integrations }: { integrations: Dashboard['integrations'] }) {
  const { t, i18n } = useTranslation();
  return (
    <Tile title={t('admin.home.integrations.title')}>
      <ul className={styles.list}>
        {integrations.map((integration) => (
          <li key={integration.key} className={styles.integration}>
            <span className={styles.integrationName}>
              <StatusDot state={integrationDot[integration.state]} />
              {t('admin.home.integrations.entry', {
                name: t(`admin.integrations.names.${integration.key}`),
                state: t(`admin.home.integrations.state.${integration.state}`),
              })}
            </span>
            {integration.detail && <span className={styles.detail}>{integration.detail}</span>}
            {integration.state === 'error' && integration.checkedAt && (
              <span className={styles.detail}>
                {t('admin.home.integrations.checked', {
                  date: formatDateTime(integration.checkedAt, i18n.language),
                })}
              </span>
            )}
          </li>
        ))}
      </ul>
      <TextLink href={paths.adminIntegrations}>{t('admin.home.integrations.manage')}</TextLink>
    </Tile>
  );
}

/**
 * The incoming Radarr/Sonarr webhooks (M9, I-8): how many arrived in the last 24 hours and which
 * integration has gone quiet for a week — a silent one usually means a changed address.
 */
function WebhooksTile({ webhooks }: { webhooks: Dashboard['webhooks'] }) {
  const { t } = useTranslation();
  return (
    <Tile title={t('admin.home.webhooks.title')}>
      {webhooks.total === 0 ? (
        <Text tone="secondary">{t('admin.home.webhooks.none')}</Text>
      ) : (
        <>
          <Text>
            {t('admin.home.webhooks.count', {
              count: webhooks.eventsLast24h,
              total: webhooks.total,
            })}
          </Text>
          {webhooks.silent.length > 0 && (
            <span className={styles.integrationName}>
              <StatusDot state="warning" />
              {t('admin.home.webhooks.silent', { names: webhooks.silent.join(', ') })}
            </span>
          )}
        </>
      )}
      <TextLink href={paths.adminIntegrations}>{t('admin.home.webhooks.manage')}</TextLink>
    </Tile>
  );
}

function ServerTile({ system }: { system: SystemInfo }) {
  const { t, i18n } = useTranslation();
  return (
    <Tile title={t('admin.home.server.title')} wide>
      <SystemState
        system={system}
        showUsage
        facts={[
          [t('admin.home.server.version'), system.version],
          [t('admin.home.server.started'), formatDateTime(system.startedAt, i18n.language)],
        ]}
      />
    </Tile>
  );
}

/**
 * Stable keys of the error list (newest first): time, source and message, with a counter for the
 * same entry twice. A new error on top keeps the keys, and so the nodes, of the older ones.
 */
function errorKeys(errors: readonly ErrorEntry[]): string[] {
  const seen = new Map<string, number>();
  return errors.map((entry) => {
    const key = `${entry.at}|${entry.source}|${entry.message}`;
    const count = seen.get(key) ?? 0;
    seen.set(key, count + 1);
    return count === 0 ? key : `${key}|${count}`;
  });
}

function ErrorsTile({ errors }: { errors: Dashboard['errors'] }) {
  const { t, i18n } = useTranslation();
  const keys = errorKeys(errors);
  // A poll with new errors is announced as a short count, not as the whole list.
  const [previous, setPrevious] = useState(errors);
  const [news, setNews] = useState({ count: 0, seq: 0 });
  if (errors !== previous) {
    const known = new Set(errorKeys(previous));
    const count = keys.filter((key) => !known.has(key)).length;
    setPrevious(errors);
    if (count > 0) setNews((current) => ({ count, seq: current.seq + 1 }));
  }
  return (
    <Tile title={t('admin.home.errors.title')} wide>
      <Text tone="secondary" variant="caption">
        {t('admin.home.errors.intro')}
      </Text>
      {errors.length === 0 && <Text tone="secondary">{t('admin.home.errors.empty')}</Text>}
      {/* Always there, so the first count is read out too; a new node for every poll with news. */}
      <div role="status">
        {news.count > 0 && (
          <VisuallyHidden key={news.seq}>
            {t('admin.home.errors.new', { count: news.count })}
          </VisuallyHidden>
        )}
      </div>
      <ol className={styles.errors} aria-label={t('admin.home.errors.list')}>
        {errors.map((entry, index) => (
          <li key={keys[index]} className={styles.error}>
            <span className={styles.errorMeta}>
              <time dateTime={entry.at}>{formatDateTime(entry.at, i18n.language)}</time>
              <span>{entry.source}</span>
              {entry.jobId !== undefined && (
                <span>
                  {t('admin.home.errors.job', { id: entry.jobId, type: entry.jobType ?? '' })}
                </span>
              )}
            </span>
            <span className={styles.errorMessage}>{entry.message}</span>
          </li>
        ))}
      </ol>
    </Tile>
  );
}

/** The cards of every admin page (A-2 "an einem Ort"), in the groups of the admin navigation. */
function ManagementCards() {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="admin-manage" className={styles.section}>
      <h2 id="admin-manage" className={styles.sectionTitle}>
        {t('admin.home.manage')}
      </h2>
      {adminGroups.map((group) => (
        <section key={group} aria-labelledby={`admin-group-${group}`} className={styles.group}>
          <h3 id={`admin-group-${group}`} className={styles.groupTitle}>
            {t(`admin.nav.groups.${group}`)}
          </h3>
          <ul className={styles.cards}>
            {adminSections
              .filter((section) => section.group === group)
              .map((section) => (
                <li key={section.key}>
                  <LinkCard
                    href={section.href}
                    icon={section.icon}
                    title={t(`admin.sections.${section.key}.title`)}
                    description={t(`admin.sections.${section.key}.description`)}
                  />
                </li>
              ))}
          </ul>
        </section>
      ))}
    </section>
  );
}

/**
 * The administration's overview: the dashboard (A-3) with streams, jobs, storage, integrations,
 * the server's hardware and the newest errors, asked again every 5 s, then a card for every
 * admin page (A-2). Failed jobs can be queued again or cancelled.
 */
export function AdminHomePage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.home.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  // The poll waits while a confirmation is open, so its row does not go away underneath it.
  const [confirming, setConfirming] = useState(false);
  const dashboard = useQuery({
    ...dashboardQuery,
    enabled: allowed,
    refetchInterval: confirming ? false : DASHBOARD_POLL_MS,
    refetchOnWindowFocus: !confirming,
  });
  const system = useQuery({ ...systemQuery, enabled: allowed });
  const [message, setMessage] = useState<string | null>(null);
  // The row of a retried or cancelled job goes away with its buttons: focus moves to the jobs.
  const jobsHeading = useRef<HTMLHeadingElement>(null);
  const settled = async (text: string) => {
    setMessage(text);
    await queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
    jobsHeading.current?.focus();
  };
  const retry = useMutation({
    mutationFn: (job: JobSummary) => retryJob(job.id),
    onSuccess: (_data, job) => settled(t('admin.home.jobs.retried', { type: job.type })),
  });
  const cancel = useMutation({
    mutationFn: (job: JobSummary) => cancelJob(job.id),
    onSuccess: (_data, job) => settled(t('admin.home.jobs.cancelled', { type: job.type })),
  });
  const jobError = retry.error ?? cancel.error;
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setMessage(null);
    retry.reset();
    cancel.reset();
  };
  const number = new Intl.NumberFormat(i18n.language);

  const jobColumns: DataColumn<JobSummary>[] = [
    {
      key: 'type',
      header: t('admin.home.jobs.columns.type'),
      rowHeader: true,
      cell: (job) => <span className={table.nowrap}>{job.type}</span>,
    },
    {
      key: 'attempts',
      header: t('admin.home.jobs.columns.attempts'),
      cell: (job) => (
        <span className={table.numbers}>
          {t('admin.home.jobs.attempts', { attempts: job.attempts, max: job.maxAttempts })}
        </span>
      ),
    },
    {
      key: 'error',
      header: t('admin.home.jobs.columns.error'),
      cell: (job) => job.lastError ?? t('admin.home.jobs.noError'),
    },
    {
      key: 'at',
      header: t('admin.home.jobs.columns.at'),
      cell: (job) => (
        <time className={table.nowrap} dateTime={job.updatedAt}>
          {formatDateTime(job.updatedAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'actions',
      header: t('admin.home.jobs.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (job) => (
        <span className={table.rowActions}>
          <AriaButton
            variant="ghost"
            icon="refresh"
            aria-label={t('admin.home.jobs.retry.label', { type: job.type, id: job.id })}
            isPending={retry.isPending && retry.variables?.id === job.id}
            onPress={() => {
              start();
              retry.mutate(job);
            }}
          >
            {t('admin.home.jobs.retry.action')}
          </AriaButton>
          <ConfirmDialog
            trigger={
              <AriaButton
                variant="ghost"
                aria-label={t('admin.home.jobs.cancel.label', { type: job.type, id: job.id })}
              >
                {t('admin.home.jobs.cancel.action')}
              </AriaButton>
            }
            title={t('admin.home.jobs.cancel.title', { type: job.type })}
            body={t('admin.home.jobs.cancel.body')}
            confirmLabel={t('admin.home.jobs.cancel.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            onConfirm={() => {
              start();
              cancel.mutate(job);
            }}
            onOpenChange={setConfirming}
          />
        </span>
      ),
    },
  ];

  const data = dashboard.data;
  const failed = data?.jobs.recentFailed ?? [];

  return (
    <Page>
      <div className={table.header}>
        <Text as="h1" variant="display">
          {t('admin.home.title')}
        </Text>
        <Text tone="secondary">{t('admin.home.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <>
          <section
            aria-labelledby="admin-state"
            className={styles.section}
            aria-busy={dashboard.isPending}
          >
            <h2 id="admin-state" className={styles.sectionTitle}>
              {t('admin.home.state')}
            </h2>
            {data?.update.available && data.update.latestVersion && (
              <Notice
                title={t('admin.home.update.title', { version: data.update.latestVersion })}
                actions={
                  data.update.url ? (
                    <TextLink href={data.update.url} external>
                      {t('admin.home.update.notes')}
                    </TextLink>
                  ) : undefined
                }
              >
                {t('admin.home.update.body', { current: data.update.currentVersion })}
              </Notice>
            )}
            {data?.browse && data.browse.lostSlots > 0 && (
              <Notice tone="warning" title={t('admin.home.browse.title')} live={false}>
                {data.browse.lostSlots >= data.browse.slots
                  ? t('admin.home.browse.off')
                  : t('admin.home.browse.hanging', { count: data.browse.lostSlots })}
              </Notice>
            )}
            {dashboard.isPending && <Skeleton shape="block" />}
            {dashboard.isError && (
              <Notice
                tone="danger"
                title={problemMessage(t, dashboard.error)}
                actions={
                  <AriaButton icon="refresh" onPress={() => void dashboard.refetch()}>
                    {t('common.retry')}
                  </AriaButton>
                }
              />
            )}
            {data && (
              <div className={styles.tiles}>
                <StreamsTile streams={data.streams} />
                <Tile title={t('admin.home.jobs.title')} headingRef={jobsHeading}>
                  <div className={styles.counters}>
                    <StatTile
                      label={t('admin.home.jobs.queued')}
                      value={number.format(data.jobs.queued)}
                    />
                    <StatTile
                      label={t('admin.home.jobs.running')}
                      value={number.format(data.jobs.running)}
                      tone={data.jobs.running > 0 ? 'success' : 'neutral'}
                    />
                    <StatTile
                      label={t('admin.home.jobs.failed')}
                      value={number.format(data.jobs.failed)}
                      tone={data.jobs.failed > 0 ? 'danger' : 'neutral'}
                    />
                  </div>
                </Tile>
                <StorageTile storage={data.storage} />
                <IntegrationsTile integrations={data.integrations} />
                <WebhooksTile webhooks={data.webhooks} />
                {(failed.length > 0 || message || jobError) && (
                  <Tile title={t('admin.home.jobs.recent')} wide>
                    {jobError && <Notice tone="danger">{problemMessage(t, jobError)}</Notice>}
                    <div role="status" className={styles.live}>
                      {message && !jobError && <Notice live={false}>{message}</Notice>}
                    </div>
                    {failed.length > 0 && (
                      <DataTable
                        caption={t('admin.home.jobs.recent')}
                        columns={jobColumns}
                        rows={failed}
                        rowKey={(job) => String(job.id)}
                      />
                    )}
                  </Tile>
                )}
                {system.data && <ServerTile system={system.data} />}
                {system.isError && (
                  <div className={styles.wide}>
                    <Notice tone="danger">{problemMessage(t, system.error)}</Notice>
                  </div>
                )}
                <ErrorsTile errors={data.errors} />
              </div>
            )}
          </section>
          <ManagementCards />
        </>
      )}
    </Page>
  );
}
