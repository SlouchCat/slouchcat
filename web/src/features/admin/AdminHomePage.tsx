import type {
  AdminStream,
  Dashboard,
  ErrorEntry,
  IntegrationStatus,
  StorageInfo,
  SystemInfo,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useId, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatBytes, formatDateTime, formatPlayerTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  FactList,
  Notice,
  Page,
  Skeleton,
  StatusDot,
  Text,
  TextLink,
  VisuallyHidden,
  type IconName,
  type StatusDotState,
} from '@/ui';
import {
  ActivityFeed,
  AriaButton,
  CodeBlock,
  ConfirmDialog,
  DashCard,
  StatTile,
  StorageMeter,
  StreamRow,
  TaskRow,
  type ActivityEntry,
  type ActivityTone,
} from '@/ui/aria';
import { composeCommand, SYSCTL_COMMAND } from '../shared/SystemState';
import { useHealth } from '../system/useHealth';
import { formatSeconds, typeName } from './jobsFormat';
import { DASHBOARD_POLL_MS, dashboardQuery, systemQuery } from './queries';
import { reloadPage } from './restart';
import { RestartPanel } from './RestartPanel';
import { streamMode } from './StreamsPage';
import styles from './AdminHomePage.module.css';

/** The dot of an integration's state; the text next to it carries the meaning. */
const integrationDot = {
  configured: 'success',
  missing: 'pending',
  error: 'danger',
  disabled: 'pending',
} as const satisfies Record<IntegrationStatus['state'], StatusDotState>;

/** Rows of a list on the overview (G-4); the pages behind the links have them all. */
const ROWS = 3;

/** Entries of «Letzte Aktivität»: the server sends the six newest. */
const ACTIVITY_SHOWN = 6;

type ActivityEvent = Dashboard['activity'][number];

/** A storage path this full is a notice (G-5). */
const STORAGE_FULL = 0.9;

/** GPU override files of deploy/ (SystemState), one command each when the maker is unknown. */
const OVERRIDES = ['nvidia', 'intel', 'amd'] as const;

const percentOf = (part: number, whole: number) =>
  whole > 0 ? Math.min(100, Math.max(0, Math.round((part * 100) / whole))) : 0;

/** "vor 5 Min." from the moment of the poll that brought the entry, so tests and polls agree. */
function relativeTime(iso: string, now: number, language: string): string {
  const seconds = Math.round((Date.parse(iso) - now) / 1000);
  const format = new Intl.RelativeTimeFormat(language, { numeric: 'auto', style: 'short' });
  const abs = Math.abs(seconds);
  if (abs < 60) return format.format(seconds, 'second');
  if (abs < 3600) return format.format(Math.round(seconds / 60), 'minute');
  if (abs < 86_400) return format.format(Math.round(seconds / 3600), 'hour');
  return format.format(Math.round(seconds / 86_400), 'day');
}

const activityLook: Record<string, { icon: IconName; tone: ActivityTone }> = {
  playback_started: { icon: 'play', tone: 'accent' },
  playback_completed: { icon: 'check', tone: 'success' },
  playback_stopped: { icon: 'close', tone: 'neutral' },
  scan_started: { icon: 'refresh', tone: 'neutral' },
  scan_done: { icon: 'folder', tone: 'success' },
  scan_failed: { icon: 'warning', tone: 'danger' },
  scan_aborted: { icon: 'warning', tone: 'warning' },
  job_failed: { icon: 'warning', tone: 'danger' },
  backup_created: { icon: 'archive', tone: 'neutral' },
  backup_restored: { icon: 'archive', tone: 'accent' },
  server_started: { icon: 'server', tone: 'neutral' },
  server_restart: { icon: 'power', tone: 'neutral' },
};

/**
 * The sentence of an activity entry (concept 11, 11.3): the server sends the kind and its fields,
 * the locale the words. A kind this client does not know yet gets a general sentence.
 */
export function activitySentence(t: TFunction, event: ActivityEvent, language: string): string {
  const profile = event.profileName ?? t('admin.activity.someone');
  const title = event.itemTitle ?? t('admin.activity.someTitle');
  const library = event.libraryName ?? t('admin.activity.someLibrary');
  const { detail } = event;
  switch (event.kind) {
    case 'playback_started':
      return t('admin.activity.playbackStarted', { profile, title });
    case 'playback_completed':
      return t('admin.activity.playbackCompleted', { profile, title });
    case 'playback_stopped':
      return detail.by === 'revoked'
        ? t('admin.activity.playbackRevoked', { profile, title })
        : t('admin.activity.playbackStopped', { profile, title });
    case 'scan_started':
      return t('admin.activity.scanStarted', { library });
    case 'scan_done':
      return t('admin.activity.scanDone', { library, count: detail.newTitles ?? 0 });
    case 'scan_failed':
      return t('admin.activity.scanFailed', { library });
    case 'scan_aborted':
      return detail.reason === 'cancelled'
        ? t('admin.activity.scanCancelled', { library })
        : t('admin.activity.scanAborted', { library });
    case 'job_failed':
      return t('admin.activity.jobFailed', {
        type: typeName(t, detail.jobType ?? ''),
        count: detail.count ?? 1,
      });
    case 'backup_created':
      return detail.sizeBytes === undefined
        ? t('admin.activity.backupCreated')
        : t('admin.activity.backupCreatedSize', { size: formatBytes(detail.sizeBytes, language) });
    case 'backup_restored':
      return t('admin.activity.backupRestored');
    case 'server_started':
      return detail.version
        ? t('admin.activity.serverStartedVersion', { version: detail.version })
        : t('admin.activity.serverStarted');
    case 'server_restart':
      return event.accountEmail
        ? t('admin.activity.serverRestartBy', { account: event.accountEmail })
        : t('admin.activity.serverRestart');
    default:
      return t('admin.activity.unknown');
  }
}

interface Hint {
  key: string;
  tone: StatusDotState;
  title: string;
  body?: string;
  /** A link or commands to copy, under the text. */
  action?: ReactNode;
}

function Command({ code, label }: { code: string; label: string }) {
  const { t } = useTranslation();
  return (
    <CodeBlock
      code={code}
      label={label}
      copyLabel={t('system.copy.action')}
      copiedMessage={t('system.copy.done')}
      copyFailedMessage={t('system.copy.failed')}
    />
  );
}

/**
 * What needs doing (11.1), most urgent first. Integration errors and silent webhooks stand in
 * their own cards (G-8), the errors of the log in the server log.
 */
function useHints(data: Dashboard, system: SystemInfo | undefined): Hint[] {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  const hints: Hint[] = [];
  const toJobs = <TextLink href={paths.adminJobs}>{t('admin.home.hints.toJobs')}</TextLink>;
  if (data.jobs.failed > 0) {
    hints.push({
      key: 'failed',
      tone: 'danger',
      title: t('admin.home.hints.failed', {
        count: data.jobs.failed,
        formatted: number.format(data.jobs.failed),
      }),
      action: toJobs,
    });
  }
  if ((data.jobs.paused ?? 0) > 0) {
    hints.push({
      key: 'paused',
      tone: 'warning',
      title: t('admin.home.hints.paused', { count: data.jobs.paused ?? 0 }),
      action: toJobs,
    });
  }
  for (const scan of data.attention.abortedScans) {
    hints.push({
      key: `scan-${scan.libraryId}`,
      tone: 'warning',
      title: t('admin.home.hints.scan', { name: scan.name }),
      body: t(
        scan.reason === 'root_missing' ||
          scan.reason === 'root_empty' ||
          scan.reason === 'loss_over_threshold'
          ? `admin.libraries.error.${scan.reason}`
          : 'admin.home.hints.scanOther',
      ),
      action: <TextLink href={paths.adminLibraries}>{t('admin.home.hints.toLibraries')}</TextLink>,
    });
  }
  if (data.browse && data.browse.lostSlots > 0) {
    hints.push({
      key: 'browse',
      tone: 'warning',
      title: t('admin.home.browse.title'),
      body:
        data.browse.lostSlots >= data.browse.slots
          ? t('admin.home.browse.off')
          : t('admin.home.browse.hanging', { count: data.browse.lostSlots }),
    });
  }
  if (system && !system.gpu.hardwareEncode) {
    const overrides = system.gpu.composeOverride ? [system.gpu.composeOverride] : OVERRIDES;
    hints.push({
      key: 'gpu',
      tone: 'warning',
      title: t('system.gpu.none.title'),
      body: system.gpu.composeOverride ? t('system.gpu.none.body') : t('system.gpu.none.unknown'),
      action: overrides.map((override) => (
        <Command
          key={override}
          code={composeCommand(override)}
          label={t('system.gpu.command', { vendor: t(`system.gpu.vendor.${override}`) })}
        />
      )),
    });
  }
  if (system?.inotify.warn) {
    hints.push({
      key: 'inotify',
      tone: 'warning',
      title: t('system.inotify.warn.title'),
      body: t('system.inotify.warn.body'),
      action: <Command code={SYSCTL_COMMAND} label={t('system.inotify.command')} />,
    });
  }
  for (const info of [data.storage.config, data.storage.cache]) {
    if (info.totalBytes > 0 && info.totalBytes - info.freeBytes > STORAGE_FULL * info.totalBytes) {
      hints.push({
        key: `storage-${info.path}`,
        tone: 'warning',
        title: t('admin.home.hints.storage', { path: info.path, percent: STORAGE_FULL * 100 }),
        body: t('admin.home.storage.free', {
          free: formatBytes(info.freeBytes, i18n.language),
          total: formatBytes(info.totalBytes, i18n.language),
        }),
      });
    }
  }
  const { update } = data;
  if (update.available && update.latestVersion) {
    hints.push({
      key: 'update',
      tone: 'active',
      title: t('admin.home.update.title', { version: update.latestVersion }),
      body: t('admin.home.update.body', { current: update.currentVersion }),
      action: update.url ? (
        <TextLink href={update.url} external>
          {t('admin.home.update.notes')}
        </TextLink>
      ) : undefined,
    });
  }
  // A build without a release version cannot be compared with the releases, so the server never
  // offers an update for it; the page says what runs instead.
  if (update.development) {
    hints.push({
      key: 'development',
      tone: 'active',
      title: t('admin.home.update.development.title'),
      body: update.latestVersion
        ? t('admin.home.update.development.body', {
            current: update.currentVersion,
            latest: update.latestVersion,
          })
        : t('admin.home.update.development.bodyUnknown', { current: update.currentVersion }),
      action:
        update.url && update.latestVersion ? (
          <TextLink href={update.url} external>
            {t('admin.home.update.notes')}
          </TextLink>
        ) : undefined,
    });
  }
  return hints;
}

function HintsCard({ hints }: { hints: Hint[] }) {
  const { t, i18n } = useTranslation();
  return (
    <DashCard
      icon="warning"
      title={t('admin.home.hints.title')}
      count={new Intl.NumberFormat(i18n.language).format(hints.length)}
      countLabel={t('admin.home.hints.count', { count: hints.length })}
      countLive
    >
      {hints.length === 0 ? (
        <Text tone="secondary" variant="caption">
          {t('admin.home.hints.none')}
        </Text>
      ) : (
        <ul className={styles.rows}>
          {hints.map((hint) => (
            <li key={hint.key} className={styles.hint}>
              <span className={styles.hintHead}>
                <StatusDot state={hint.tone} />
                <span className={styles.hintTitle}>{hint.title}</span>
              </span>
              {hint.body && <span className={styles.detail}>{hint.body}</span>}
              {hint.action && <span className={styles.hintAction}>{hint.action}</span>}
            </li>
          ))}
        </ul>
      )}
    </DashCard>
  );
}

/** The update state in one line: what runs against the newest release. */
function updateState(t: TFunction, update: Dashboard['update']): string {
  if (update.development) return t('admin.home.server.updateDevelopment');
  if (update.available && update.latestVersion) {
    return t('admin.home.server.updateAvailable', { version: update.latestVersion });
  }
  if (update.error) return t('admin.home.server.updateFailed');
  if (!update.enabled) return t('admin.home.server.updateOff');
  return t('admin.home.server.updateCurrent');
}

function ServerCard({
  data,
  now,
  system,
  systemError,
  onRestart,
}: {
  data: Dashboard;
  /** The moment of the poll: the uptime counts on to it. */
  now: number;
  system: SystemInfo | undefined;
  systemError: unknown;
  onRestart: () => void;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const check = useMutation({
    mutationFn: () => request(api.POST('/admin/update/check')),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
  });
  const restart = useMutation({
    mutationFn: () => request(api.POST('/admin/server/restart')),
    onSuccess: onRestart,
    // A restart or restore is already on its way: wait for it like for our own.
    onError: (error) => {
      if (isApiError(error, 409) && error.code === 'restart_pending') onRestart();
    },
  });
  const number = (value: number) => new Intl.NumberFormat(i18n.language).format(value);
  // The server's own uptime (getHealth), counted on with this browser's clock since it answered,
  // so a clock that differs from the server's does not shift it; the start time only without it.
  const health = useHealth();
  const uptime = health.data
    ? health.data.uptimeSeconds + Math.max(0, now - health.dataUpdatedAt) / 1000
    : system
      ? (now - Date.parse(system.startedAt)) / 1000
      : 0;
  const { update } = data;
  const streams = data.streams.length;
  const error = check.error ?? restart.error;

  return (
    <DashCard
      icon="server"
      title={t('admin.home.server.title')}
      footer={
        <>
          <AriaButton
            icon="refresh"
            isPending={check.isPending}
            onPress={() => {
              restart.reset();
              check.mutate();
            }}
          >
            {t('admin.home.server.check')}
          </AriaButton>
          <ConfirmDialog
            trigger={
              <AriaButton variant="ghost" icon="power" isPending={restart.isPending}>
                {t('admin.home.restart.action')}
              </AriaButton>
            }
            title={t('admin.home.restart.title')}
            body={
              streams > 0
                ? t('admin.home.restart.streams', { count: streams })
                : t('admin.home.restart.idle')
            }
            confirmLabel={t('admin.home.restart.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            onConfirm={() => {
              check.reset();
              restart.mutate();
            }}
          />
        </>
      }
    >
      {system ? (
        <FactList
          facts={[
            [t('admin.home.server.version'), system.version],
            [
              t('admin.home.server.uptime'),
              <time key="uptime" dateTime={system.startedAt}>
                {formatSeconds(t, uptime)}
              </time>,
            ],
            [t('system.gpu.encoder'), t(`system.gpu.backend.${system.gpu.backend}`)],
            system.inotify.maxUserWatches > 0
              ? [
                  t('system.inotify.usage'),
                  t('system.inotify.usageValue', {
                    used: number(system.inotify.watchedDirs),
                    limit: number(system.inotify.maxUserWatches),
                    percent: number(Math.round(system.inotify.usagePercent)),
                  }),
                ]
              : [t('system.inotify.limit'), t('system.inotify.unknown')],
            [t('admin.home.server.update'), updateState(t, update)],
          ]}
        />
      ) : systemError ? (
        <Notice tone="danger">{problemMessage(t, systemError)}</Notice>
      ) : (
        <Skeleton shape="block" />
      )}
      {update.checkedAt && (
        <Text tone="secondary" variant="caption">
          {t('admin.home.server.checked', {
            date: formatDateTime(update.checkedAt, i18n.language),
          })}
        </Text>
      )}
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
      {check.isSuccess && !error && (
        <Notice live={false}>{t('admin.home.server.checkedNow')}</Notice>
      )}
    </DashCard>
  );
}

function streamProgress(stream: AdminStream) {
  const position = stream.positionMs ?? 0;
  const duration = stream.durationMs ?? 0;
  return {
    percent: percentOf(position, duration),
    text:
      duration > 0
        ? `${formatPlayerTime(position, duration)} / ${formatPlayerTime(duration, duration)}`
        : undefined,
  };
}

function StreamsCard({ streams }: { streams: Dashboard['streams'] }) {
  const { t } = useTranslation();
  return (
    <DashCard
      icon="play"
      title={t('admin.home.streams.title')}
      footer={<TextLink href={paths.adminStreams}>{t('admin.home.streams.all')}</TextLink>}
    >
      {streams.length === 0 ? (
        <Text tone="secondary" variant="caption">
          {t('admin.home.streams.empty')}
        </Text>
      ) : (
        <ul className={styles.rows}>
          {streams.slice(0, ROWS).map((stream) => {
            const progress = streamProgress(stream);
            return (
              <li key={stream.id}>
                <StreamRow
                  title={stream.itemTitle}
                  posterUrl={stream.posterUrl}
                  profile={stream.profileName}
                  device={stream.device?.name ?? t('admin.streams.unknownDevice')}
                  mode={
                    stream.quality && stream.quality !== 'auto'
                      ? [streamMode(t, stream), stream.quality].join(t('format.separator'))
                      : streamMode(t, stream)
                  }
                  progress={progress.percent}
                  progressLabel={t('admin.home.streams.progress', { title: stream.itemTitle })}
                  progressText={progress.text}
                />
              </li>
            );
          })}
        </ul>
      )}
    </DashCard>
  );
}

function TasksCard({ jobs }: { jobs: Dashboard['jobs'] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  return (
    <DashCard
      icon="refresh"
      title={t('admin.home.jobs.title')}
      footer={<TextLink href={paths.adminJobs}>{t('admin.home.jobs.all')}</TextLink>}
    >
      <div className={styles.counters}>
        <StatTile label={t('admin.home.jobs.queued')} value={number.format(jobs.queued)} />
        <StatTile
          label={t('admin.home.jobs.running')}
          value={number.format(jobs.running)}
          tone={jobs.running > 0 ? 'success' : 'neutral'}
        />
        <StatTile
          label={t('admin.home.jobs.failed')}
          value={number.format(jobs.failed)}
          tone={jobs.failed > 0 ? 'danger' : 'neutral'}
        />
        {(jobs.paused ?? 0) > 0 && (
          <StatTile
            label={t('admin.home.jobs.paused')}
            value={number.format(jobs.paused ?? 0)}
            tone="warning"
          />
        )}
      </div>
      {jobs.active.length > 0 ? (
        <ul className={styles.rows} aria-label={t('admin.home.jobs.active')}>
          {jobs.active.slice(0, ROWS).map((run) => {
            const name = typeName(t, run.type);
            const total = run.progress?.total ?? 0;
            const done = run.progress?.done ?? 0;
            return (
              <li key={run.id}>
                <TaskRow
                  type={name}
                  subject={run.subject}
                  progress={percentOf(done, total)}
                  progressLabel={t('admin.home.jobs.progress', { name })}
                  progressText={
                    total > 0 ? t('admin.jobs.running.progress', { done, total }) : undefined
                  }
                  remaining={
                    run.estimatedSecondsLeft === undefined
                      ? undefined
                      : t('admin.jobs.estimate.left', {
                          duration: formatSeconds(t, run.estimatedSecondsLeft),
                        })
                  }
                />
              </li>
            );
          })}
        </ul>
      ) : (
        <Text tone="secondary" variant="caption">
          {jobs.nextRun
            ? t('admin.home.jobs.next', {
                type: typeName(t, jobs.nextRun.type),
                date: formatDateTime(jobs.nextRun.at, i18n.language),
              })
            : t('admin.home.jobs.idle')}
        </Text>
      )}
      {jobs.failedByType.length > 0 && (
        <ul className={styles.rows} aria-label={t('admin.home.jobs.failedByType')}>
          {jobs.failedByType.slice(0, ROWS).map((failure) => (
            <li key={failure.type} className={styles.failure}>
              <StatusDot state="danger" />
              <span className={styles.failureText}>
                <span className={styles.failureType}>
                  {t('admin.home.jobs.failedType', {
                    type: typeName(t, failure.type),
                    count: failure.count,
                  })}
                </span>
                {failure.lastError && (
                  <span className={styles.oneLine} title={failure.lastError}>
                    {failure.lastError}
                  </span>
                )}
              </span>
            </li>
          ))}
        </ul>
      )}
    </DashCard>
  );
}

function ActivityCard({ activity, now }: { activity: Dashboard['activity']; now: number }) {
  const { t, i18n } = useTranslation();
  const entries: ActivityEntry[] = activity.slice(0, ACTIVITY_SHOWN).map((event) => {
    const look = activityLook[event.kind] ?? { icon: 'info', tone: 'neutral' };
    return {
      id: String(event.id),
      icon: look.icon,
      tone: look.tone,
      text: activitySentence(t, event, i18n.language),
      time: relativeTime(event.at, now, i18n.language),
      dateTime: event.at,
    };
  });
  return (
    <DashCard icon="activity" title={t('admin.home.activity.title')}>
      {entries.length === 0 ? (
        <Text tone="secondary" variant="caption">
          {t('admin.home.activity.empty')}
        </Text>
      ) : (
        <ActivityFeed label={t('admin.home.activity.title')} entries={entries} />
      )}
    </DashCard>
  );
}

function IntegrationsCard({ integrations }: { integrations: Dashboard['integrations'] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  const configured = integrations.filter((entry) => entry.state === 'configured').length;
  return (
    <DashCard
      icon="plug"
      title={t('admin.home.integrations.title')}
      count={t('admin.nav.counts.ofTotal', {
        count: number.format(configured),
        total: number.format(integrations.length),
      })}
      countLabel={t('admin.home.integrations.count', {
        count: configured,
        total: integrations.length,
      })}
      footer={
        <TextLink href={paths.adminIntegrations}>{t('admin.home.integrations.manage')}</TextLink>
      }
    >
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
          </li>
        ))}
      </ul>
    </DashCard>
  );
}

/**
 * The incoming Radarr/Sonarr webhooks (M9, I-8, decision 10 (e)): how many arrived in the last
 * 24 hours and which integration has gone quiet for a week — a silent one usually means a changed
 * address.
 */
function WebhooksCard({ webhooks }: { webhooks: Dashboard['webhooks'] }) {
  const { t } = useTranslation();
  return (
    <DashCard
      icon="download"
      title={t('admin.home.webhooks.title')}
      footer={<TextLink href={paths.adminIntegrations}>{t('admin.home.webhooks.manage')}</TextLink>}
    >
      {webhooks.total === 0 ? (
        <Text tone="secondary" variant="caption">
          {t('admin.home.webhooks.none')}
        </Text>
      ) : (
        <>
          <Text variant="caption">
            {t('admin.home.webhooks.count', {
              count: webhooks.eventsLast24h,
              total: webhooks.total,
            })}
          </Text>
          {webhooks.silent.length > 0 && (
            <span className={styles.integrationName}>
              <StatusDot state="warning" />
              {t('admin.home.webhooks.silent', {
                names: webhooks.silent.join(t('format.listSeparator')),
              })}
            </span>
          )}
        </>
      )}
    </DashCard>
  );
}

function StorageCard({ storage }: { storage: Dashboard['storage'] }) {
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
  // F-4: always both paths, even on the same disk.
  return (
    <DashCard icon="disk" title={t('admin.home.storage.title')}>
      {meter(storage.config, t('admin.home.storage.config', { path: storage.config.path }))}
      {meter(storage.cache, t('admin.home.storage.cache', { path: storage.cache.path }))}
    </DashCard>
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

/**
 * The newest errors of the server log (A-3, F-3), collapsed at the bottom with their number. A
 * poll with new errors is announced as a short count, also while the list is closed.
 */
function ServerLog({ errors }: { errors: Dashboard['errors'] }) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const keys = errorKeys(errors);
  const [previous, setPrevious] = useState(errors);
  const [news, setNews] = useState({ count: 0, seq: 0 });
  if (errors !== previous) {
    const known = new Set(errorKeys(previous));
    const count = keys.filter((key) => !known.has(key)).length;
    setPrevious(errors);
    if (count > 0) setNews((current) => ({ count, seq: current.seq + 1 }));
  }
  return (
    <section aria-labelledby={id} className={styles.log}>
      <details className={styles.details}>
        <summary className={styles.summary}>
          <span id={id} className={styles.summaryTitle}>
            {t('admin.home.errors.title')}
          </span>
          <span className={styles.summaryCount}>
            {t('format.separator')}
            {errors.length === 0
              ? t('admin.home.errors.none')
              : t('admin.home.errors.count', { count: errors.length })}
          </span>
        </summary>
        <div className={styles.logBody}>
          {errors.length === 0 ? (
            <Text tone="secondary" variant="caption">
              {t('admin.home.errors.empty')}
            </Text>
          ) : (
            <Text tone="secondary" variant="caption">
              {t('admin.home.errors.intro')}
            </Text>
          )}
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
        </div>
      </details>
      {/* Outside the details, so it speaks while they are closed; a new node for every poll. */}
      <div role="status">
        {news.count > 0 && (
          <VisuallyHidden key={news.seq}>
            {t('admin.home.errors.new', { count: news.count })}
          </VisuallyHidden>
        )}
      </div>
    </section>
  );
}

/** The cards of the overview, with the moment of the poll for the relative times. */
function Cards({
  data,
  now,
  system,
  systemError,
  onRestart,
}: {
  data: Dashboard;
  now: number;
  system: SystemInfo | undefined;
  systemError: unknown;
  onRestart: () => void;
}) {
  const hints = useHints(data, system);
  return (
    <>
      <div className={styles.frame}>
        <div className={styles.grid}>
          <ServerCard
            data={data}
            now={now}
            system={system}
            systemError={systemError}
            onRestart={onRestart}
          />
          <StreamsCard streams={data.streams} />
          <TasksCard jobs={data.jobs} />
          <ActivityCard activity={data.activity} now={now} />
          <IntegrationsCard integrations={data.integrations} />
          <WebhooksCard webhooks={data.webhooks} />
          <StorageCard storage={data.storage} />
          <HintsCard hints={hints} />
        </div>
      </div>
      <ServerLog errors={data.errors} />
    </>
  );
}

/**
 * The administration's overview (concept 11, 11.1; A-3): eight equal cards in two columns, asked
 * again every 5 s — server, running streams, background jobs, the activity log, integrations,
 * webhooks, storage and what needs doing — and the server log collapsed below. «Neu starten»
 * replaces the cards with the restart's waiting panel. The sidebar (AdminShellNav) leads to every
 * admin page.
 */
export function AdminHomePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.home.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  // When the server accepted the restart (ms): the waiting panel replaces the cards.
  const [restartAt, setRestartAt] = useState<number | null>(null);
  const dashboard = useQuery({
    ...dashboardQuery,
    enabled: allowed && restartAt === null,
    refetchInterval: DASHBOARD_POLL_MS,
  });
  const system = useQuery({ ...systemQuery, enabled: allowed });
  const data = dashboard.data;

  return (
    <Page>
      <div className={styles.head}>
        <Text as="h1" variant="display">
          {t('admin.home.title')}
        </Text>
        {allowed && (
          <span className={styles.live}>
            <StatusDot state={dashboard.isError ? 'danger' : 'success'} />
            {dashboard.isError ? t('admin.home.liveFailed') : t('admin.home.live')}
          </span>
        )}
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : restartAt !== null ? (
        <RestartPanel
          acceptedAt={restartAt}
          title={t('admin.home.restart.waitTitle')}
          body={t('admin.home.restart.waitBody')}
          onBack={reloadPage}
        />
      ) : (
        <div aria-busy={dashboard.isPending} className={styles.body}>
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
            <Cards
              data={data}
              now={dashboard.dataUpdatedAt}
              system={system.data}
              systemError={system.error}
              onRestart={() => setRestartAt(Date.now())}
            />
          )}
        </div>
      )}
    </Page>
  );
}
