import type {
  AdminStream,
  Dashboard,
  ErrorEntry,
  StorageInfo,
  SystemInfo,
  UpdateInstallStatus,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useEffect, useId, useRef, useState, type ReactNode, type Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatBytes, formatDate, formatDateTime, formatPlayerTime } from '@/lib/format';
import {
  Notice,
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
  StorageMeter,
  type ActivityEntry,
  type ActivityTone,
} from '@/ui/aria';
import { composeCommand, SYSCTL_COMMAND } from '../../shared/SystemState';
import { useHealth } from '../../system/useHealth';
import { PROVIDERS } from '../integrations/ProviderCard';
import { integrationDetail, integrationHref, serviceHref } from '../integrations/services';
import { formatSeconds, typeName } from '../jobsFormat';
import { dashboardQuery } from '../queries';
import { reloadPage, RESTART_POLL_MS, restartedSince } from '../restart';
import styles from './Server.module.css';

// The parts the overview (`/admin`) and the page «Server» (`/admin/server`) share (redesign,
// build plan 3 and 4): the hints, the update install, storage, the server log and the activity.

/** Rows of a list on the overview (G-4); the pages behind the links have them all. */
export const ROWS = 3;

/** Entries of «Letzte Aktivität»: the server sends the six newest. */
export const ACTIVITY_SHOWN = 6;

type ActivityEvent = Dashboard['activity'][number];

/** A storage path this full is a notice (G-5). */
const STORAGE_FULL = 0.9;

/** GPU override files of deploy/ (SystemState), one command each when the maker is unknown. */
const OVERRIDES = ['nvidia', 'intel', 'amd'] as const;

export const percentOf = (part: number, whole: number) =>
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
  library_deleted: { icon: 'trash', tone: 'warning' },
  item_deleted: { icon: 'trash', tone: 'warning' },
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
    case 'library_deleted':
      return t('admin.activity.libraryDeleted', {
        library,
        account: event.accountEmail ?? t('admin.activity.someone'),
        count: detail.count ?? 0,
      });
    case 'item_deleted': {
      const account = event.accountEmail ?? t('admin.activity.someone');
      return detail.mode === 'files'
        ? t('admin.activity.itemFilesDeleted', {
            title,
            account,
            count: detail.count ?? 0,
            size: formatBytes(detail.sizeBytes ?? 0, language),
          })
        : t('admin.activity.itemRemoved', { title, account });
    }
    default:
      return t('admin.activity.unknown');
  }
}

/**
 * Where a hint stands: `overview` only on the overview, the others on the overview and with the
 * state they belong to on the page «Server».
 */
export type HintPlace = 'overview' | 'system' | 'storage' | 'update';

export interface Hint {
  key: string;
  place: HintPlace;
  tone: StatusDotState;
  title: string;
  body?: string;
  /** A link or commands to copy, under the text. */
  action?: ReactNode;
}

export function Command({ code, label }: { code: string; label: string }) {
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
 * «Verstanden» under the one-time hint that ratings are minimum ages now (concept 22 section
 * 4.11): the server forgets the hint, the next poll shows the overview without it.
 */
function DismissMinimumAge() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const dismiss = useMutation({
    mutationFn: () => request(api.DELETE('/admin/dashboard/minimum-age-notice')),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
  });
  return (
    <AriaButton variant="secondary" onPress={() => dismiss.mutate()} isDisabled={dismiss.isPending}>
      {t('admin.home.hints.minimumAgeDismiss')}
    </AriaButton>
  );
}

/** The tab «Fehlschläge» of the jobs page (concept 12, JOB-8, 11.5 point 11). */
export const jobsFailed = `${paths.adminJobs}?tab=failed`;

/** The failures per job type under the hint of the failed jobs (at most three, G-4). */
function FailedByType({ failures }: { failures: Dashboard['jobs']['failedByType'] }) {
  const { t } = useTranslation();
  return (
    <ul className={styles.rows} aria-label={t('admin.home.jobs.failedByType')}>
      {failures.slice(0, ROWS).map((failure) => (
        <li key={failure.type} className={styles.failure}>
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
        </li>
      ))}
    </ul>
  );
}

/**
 * What needs doing (11.1; redesign, build plan 3), most urgent first: a running update install,
 * the hints of the dashboard and the system, the integrations with an error and the silent
 * webhooks. The page «Server» shows the ones of its states (`place`) next to them.
 */
export function useHints(data: Dashboard, system: SystemInfo | undefined): Hint[] {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(intlLocale(i18n.language));
  const hints: Hint[] = [];
  const toJobs = <TextLink href={paths.adminJobs}>{t('admin.home.hints.toJobs')}</TextLink>;
  const toServer = <TextLink href={paths.adminServer}>{t('admin.home.summary.details')}</TextLink>;
  // The install itself, its phases and its result stand on the page «Server» (build plan 7.2).
  const { install } = data.update;
  if (isInstalling(install)) {
    hints.push({
      key: 'install',
      place: 'overview',
      tone: 'active',
      title: install.to
        ? t('admin.home.install.running', { version: install.to })
        : t('admin.home.install.runningNoVersion'),
      action: toServer,
    });
  }
  if (data.jobs.failed > 0) {
    hints.push({
      key: 'failed',
      place: 'overview',
      tone: 'danger',
      title: t('admin.home.hints.failed', {
        count: data.jobs.failed,
        formatted: number.format(data.jobs.failed),
      }),
      action: (
        <>
          {data.jobs.failedByType.length > 0 && <FailedByType failures={data.jobs.failedByType} />}
          <TextLink href={jobsFailed}>{t('admin.home.hints.toJobs')}</TextLink>
        </>
      ),
    });
  }
  if ((data.jobs.paused ?? 0) > 0) {
    hints.push({
      key: 'paused',
      place: 'overview',
      tone: 'warning',
      title: t('admin.home.hints.paused', { count: data.jobs.paused ?? 0 }),
      action: toJobs,
    });
  }
  // The ratings are read anew (concept 22 F-16, F-17): until the job is done the old ones apply.
  if (data.attention.ratingsRereading) {
    hints.push({
      key: 'ratings-reread',
      place: 'overview',
      tone: 'active',
      title: t('admin.home.hints.ratingsReread'),
      body: t('admin.home.hints.ratingsRereadBody'),
      action: toJobs,
    });
  }
  // Once after the update to minimum ages: PG-13 is «ab 13» now, stricter than FSK 12 before.
  if (data.attention.minimumAgeNotice) {
    hints.push({
      key: 'minimum-age',
      place: 'overview',
      tone: 'warning',
      title: t('admin.home.hints.minimumAge'),
      body: t('admin.home.hints.minimumAgeBody'),
      action: <DismissMinimumAge />,
    });
  }
  // A provider that rejects its key (concept 21 P-2): one notice instead of a failed fill per title.
  for (const provider of data.attention.rejectedKeys) {
    const known = PROVIDERS.find((entry) => entry.id === provider);
    const name = known ? t(`admin.integrations.providers.${known.id}.name`) : provider;
    hints.push({
      key: `key-${provider}`,
      place: 'overview',
      tone: 'danger',
      title: t('admin.home.hints.keyRejected', { name }),
      body: t('admin.home.hints.keyRejectedBody'),
      action: <TextLink href={serviceHref('keys')}>{t('admin.home.hints.toKeys')}</TextLink>,
    });
  }
  // An integration with an error, with its reason (the dashboard's `detail`), and the webhooks
  // gone quiet for a week: the whole list stands on «Integrationen» (build plan 7.2).
  const wishes = data.attention.wishes;
  for (const integration of data.integrations) {
    if (integration.state !== 'error') continue;
    // Seerr's state has a hint of its own among the wishes' below.
    if (integration.key === 'seerr' && wishes?.seerrState) continue;
    hints.push({
      key: `integration-${integration.key}`,
      place: 'overview',
      tone: 'danger',
      title: t('admin.home.integrations.entry', {
        name: t(`admin.integrations.names.${integration.key}`),
        state: t('admin.home.integrations.state.error'),
      }),
      body: integrationDetail(t, integration),
      action: (
        <TextLink href={integrationHref(integration.key)}>
          {t('admin.home.integrations.manage')}
        </TextLink>
      ),
    });
  }
  // The wishes (concept 25, administration 2): Seerr's state, accounts without a Seerr user,
  // links made by an invitation or an e-mail change, used-up limits and users gone from Seerr.
  if (wishes) {
    const users = `${paths.adminWishes}?tab=seerr-benutzer`;
    const usersAt = (accountId: number) => `${users}&item=${accountId}`;
    if (wishes.seerrState) {
      hints.push({
        key: 'wishes-seerr',
        place: 'overview',
        tone: 'danger',
        title: t(`admin.home.wishes.seerrState.${wishes.seerrState}`),
        body: t(`admin.home.wishes.seerrStateBody.${wishes.seerrState}`),
        action: (
          <TextLink href={serviceHref('seerr')}>{t('admin.home.wishes.toConnection')}</TextLink>
        ),
      });
    }
    if (wishes.unmapped > 0) {
      const body = [
        t(
          wishes.unmappedFallback
            ? 'admin.home.wishes.unmappedFallback'
            : 'admin.home.wishes.unmappedDeny',
        ),
        ...(wishes.seerrState === 'unreachable' ? [t('admin.home.wishes.unmappedWaiting')] : []),
      ].join(' ');
      hints.push({
        key: 'wishes-unmapped',
        place: 'overview',
        tone: 'warning',
        title: t('admin.home.wishes.unmapped', { count: wishes.unmapped }),
        body,
        action: <TextLink href={users}>{t('admin.home.wishes.toUsers')}</TextLink>,
      });
    }
    for (const linked of wishes.linked) {
      hints.push({
        key: `wishes-linked-${linked.accountId}`,
        place: 'overview',
        tone: 'warning',
        title: t('admin.home.wishes.linked', {
          account: linked.accountEmail,
          name: linked.seerrDisplayName,
        }),
        body: t(`admin.home.wishes.linkedBody.${linked.linkedBy}`, {
          date: formatDate(linked.linkedAt, intlLocale(i18n.language)),
        }),
        action: (
          <TextLink href={usersAt(linked.accountId)}>{t('admin.home.wishes.toAccount')}</TextLink>
        ),
      });
    }
    for (const reached of wishes.quotaReached) {
      const halves = (['movie', 'tv'] as const).flatMap((kind) => {
        const part = reached[kind];
        if (!part || part.remaining > 0) return [];
        const used = t(`admin.wishes.quota.${kind}`, {
          used: part.used,
          limit: part.limit,
          days: part.days,
        });
        return [
          part.nextFreeAt
            ? t('admin.home.wishes.quotaNext', {
                used,
                date: formatDate(part.nextFreeAt, intlLocale(i18n.language)),
              })
            : used,
        ];
      });
      hints.push({
        key: `wishes-quota-${reached.accountId}`,
        place: 'overview',
        tone: 'warning',
        title: t('admin.home.wishes.quotaReached', { account: reached.accountEmail }),
        body: halves.join(' '),
        action: (
          <TextLink href={`${paths.adminAccounts}?item=${reached.accountId}`}>
            {t('admin.home.wishes.toLimit')}
          </TextLink>
        ),
      });
    }
    if (wishes.goneUsers > 0) {
      hints.push({
        key: 'wishes-gone',
        place: 'overview',
        tone: 'danger',
        title: t('admin.home.wishes.gone', { count: wishes.goneUsers }),
        body: t('admin.home.wishes.goneBody'),
        action: <TextLink href={users}>{t('admin.home.wishes.toUsers')}</TextLink>,
      });
    }
  }
  if (data.webhooks.silent.length > 0) {
    hints.push({
      key: 'webhooks-silent',
      place: 'overview',
      tone: 'warning',
      title: t('admin.home.webhooks.silent', {
        names: data.webhooks.silent.join(t('format.listSeparator')),
      }),
      action: <TextLink href={serviceHref('webhooks')}>{t('admin.home.webhooks.manage')}</TextLink>,
    });
  }
  for (const scan of data.attention.abortedScans) {
    hints.push({
      key: `scan-${scan.libraryId}`,
      place: 'overview',
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
      place: 'system',
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
      place: 'system',
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
  // A stored graphics chip failed its probe at start: auto runs instead (concept 22, F-10).
  if (system?.gpu.fallbackFrom) {
    const backend = system.gpu.fallbackFrom;
    hints.push({
      key: 'gpu-fallback',
      place: 'system',
      tone: 'warning',
      title: t('system.gpu.fallback.title'),
      body: t('system.gpu.fallback.body', {
        name:
          backend === 'software'
            ? t('system.gpu.choice.software')
            : t(`system.gpu.backend.${backend}`),
      }),
      // The graphics chip is chosen in the server's view of the integrations page.
      action: <TextLink href={serviceHref('server')}>{t('admin.home.hints.toGpu')}</TextLink>,
    });
  }
  if (system?.inotify.warn) {
    hints.push({
      key: 'inotify',
      place: 'system',
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
        place: 'storage',
        tone: 'warning',
        title: t('admin.home.hints.storage', { path: info.path, percent: STORAGE_FULL * 100 }),
        body: t('admin.home.storage.free', {
          free: formatBytes(info.freeBytes, intlLocale(i18n.language)),
          total: formatBytes(info.totalBytes, intlLocale(i18n.language)),
        }),
      });
    }
  }
  const { update } = data;
  if (update.available && update.latestVersion) {
    hints.push({
      key: 'update',
      place: 'update',
      tone: 'active',
      title: t('admin.home.update.title', { version: update.latestVersion }),
      // With a companion that answers the page offers the install button; otherwise the commands.
      body: update.updaterAvailable
        ? t('admin.home.update.bodyUpdater', { current: update.currentVersion })
        : t('admin.home.update.body', { current: update.currentVersion }),
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
      place: 'update',
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

/** Phases of a running install (UpdateInstallStatus); the others end it or never began one. */
const INSTALL_RUNNING: ReadonlySet<UpdateInstallStatus['phase']> = new Set([
  'checking',
  'backing_up',
  'pulling',
  'verifying',
  'restarting',
  'waiting',
]);

type RunningPhase = 'checking' | 'backing_up' | 'pulling' | 'verifying' | 'restarting' | 'waiting';

export const isInstalling = (
  install: UpdateInstallStatus | undefined,
): install is UpdateInstallStatus & { phase: RunningPhase } =>
  install !== undefined && INSTALL_RUNNING.has(install.phase);

/** The three steps the card shows (concept 15, 9.3), each with the companion's phases in it. */
const INSTALL_STEPS = [
  { key: 'download', phases: ['checking', 'backing_up', 'pulling', 'verifying'] },
  { key: 'restart', phases: ['restarting'] },
  { key: 'health', phases: ['waiting'] },
] as const satisfies readonly {
  key: string;
  phases: readonly UpdateInstallStatus['phase'][];
}[];

/** Refusals of the companion (422 of installUpdate) the card explains in its own words. */
const REFUSALS = [
  'not_newer',
  'no_digest',
  'no_compose_labels',
  'outside_deploy_dir',
  'manifest_invalid',
  'manifest_unreachable',
] as const;

/** Reasons of a failed install (contract 1.18.0) the card explains in its own words. */
const FAILURES = ['disk_full', 'deploy_files'] as const;

const failureKey = (code: string | undefined) =>
  code !== undefined && (FAILURES as readonly string[]).includes(code)
    ? (`admin.home.install.failure.${code as (typeof FAILURES)[number]}` as const)
    : undefined;

/** Refusals that mean the companion cannot restart this server: the card shows the manual steps. */
const MANUAL_REFUSALS: readonly string[] = ['no_compose_labels', 'outside_deploy_dir'];

/**
 * How long the card waits for the server during an install before it says that something is
 * wrong: the server reports a running install older than that as interrupted (installStaleAfter).
 */
const INSTALL_GIVE_UP_MS = 70 * 60_000;

const refusalKey = (code: string) =>
  (REFUSALS as readonly string[]).includes(code)
    ? (`admin.home.install.refused.${code as (typeof REFUSALS)[number]}` as const)
    : 'admin.home.install.refused.other';

/** The guide to the updater companion (docs/INSTALLATION.md, section 8). */
const UPDATER_GUIDE =
  'https://github.com/SlouchCat/slouchcat/blob/main/docs/INSTALLATION.md#update-per-knopf-der-updater-begleiter-freiwillig';

/** The two commands of the manual update (fallback C), run in deploy/ with the start's files. */
const PULL_COMMAND = 'docker compose pull';
const UP_COMMAND = 'docker compose up -d';

/** A step before, at and after the current one. */
const STEP_STATES = ['done', 'current', 'pending'] as const;

const installStatusKey = ['admin', 'update', 'status'] as const;

/** The steps of a running install: the done ones, the current one, the ones still to come. */
function InstallSteps({ install }: { install: UpdateInstallStatus }) {
  const { t } = useTranslation();
  const current = INSTALL_STEPS.findIndex((step) =>
    (step.phases as readonly string[]).includes(install.phase),
  );
  return (
    <ol className={styles.steps} aria-label={t('admin.home.install.stepsLabel')}>
      {INSTALL_STEPS.map((step, index) => {
        const state = STEP_STATES[Math.sign(index - current) + 1] ?? STEP_STATES[2];
        return (
          <li
            key={step.key}
            className={styles.step}
            aria-current={state === 'current' ? 'step' : undefined}
          >
            <StatusDot
              state={state === 'done' ? 'success' : state === 'current' ? 'active' : 'pending'}
            />
            {t(`admin.home.install.steps.${step.key}`)}
            <VisuallyHidden>
              {t('format.separator')}
              {t(`admin.home.install.stepState.${state}`)}
            </VisuallyHidden>
          </li>
        );
      })}
    </ol>
  );
}

/** How the last install ended, as the companion reported it; nothing for `idle`. */
function InstallResult({ install }: { install: UpdateInstallStatus }) {
  const { t, i18n } = useTranslation();
  const finished = install.finishedAt && (
    <Text as="span" tone="secondary" variant="caption">
      {t('admin.home.install.finished', {
        date: formatDateTime(install.finishedAt, intlLocale(i18n.language)),
      })}
    </Text>
  );
  if (install.phase === 'done') {
    if (!install.to) return null;
    return (
      <span className={styles.installDone}>
        <StatusDot state="success" />
        {t('admin.home.install.done', { version: install.to })}
        {finished}
      </span>
    );
  }
  if (install.phase === 'rolled_back') {
    return (
      <Notice
        tone="warning"
        live={false}
        title={
          install.from && install.to
            ? t('admin.home.install.rolledBack', { from: install.from, to: install.to })
            : t('admin.home.install.rolledBackNoVersion')
        }
      >
        {install.message && <span className={styles.installMessage}>{install.message}</span>}
        {finished}
      </Notice>
    );
  }
  if (install.phase !== 'failed') return null;
  const failure = failureKey(install.code);
  return (
    <Notice
      tone="danger"
      live={false}
      title={
        install.to
          ? t('admin.home.install.failed', { version: install.to })
          : t('admin.home.install.failedNoVersion')
      }
    >
      {failure && <span>{t(failure)}</span>}
      {install.migrated && (
        <span>
          {install.backup
            ? t('admin.home.install.migrated', { backup: install.backup })
            : t('admin.home.install.migratedNoBackup')}
        </span>
      )}
      {install.message && <span className={styles.installMessage}>{install.message}</span>}
      {finished}
    </Notice>
  );
}

/**
 * Why the card shows the manual steps: no companion is set up (`none`), it is set up but does not
 * answer (`unavailable`), or it refused because it cannot restart this server (`refused`, the
 * refusal stands above).
 */
type ManualReason = 'none' | 'unavailable' | 'refused';

/**
 * Fallback C (concept 15, 9.3): the line for .env and the two commands. It takes the focus when
 * the install button it replaces had it (`focusRef`).
 */
function ManualUpdate({
  version,
  reason,
  focusRef,
}: {
  version: string;
  reason: ManualReason;
  focusRef: Ref<HTMLDivElement>;
}) {
  const { t } = useTranslation();
  return (
    <div ref={focusRef} tabIndex={-1} className={styles.manual}>
      {reason === 'unavailable' && (
        <Notice tone="warning">{t('admin.home.install.manual.unavailable')}</Notice>
      )}
      <Text variant="caption">
        {reason === 'none'
          ? t('admin.home.install.manual.intro')
          : t('admin.home.install.manual.introByHand')}
      </Text>
      <Command code={`SLOUCHCAT_VERSION=${version}`} label={t('admin.home.install.manual.env')} />
      <Command code={PULL_COMMAND} label={t('admin.home.install.manual.pull')} />
      <Command code={UP_COMMAND} label={t('admin.home.install.manual.up')} />
      <Text tone="secondary" variant="caption">
        {t('admin.home.install.manual.files')}
      </Text>
      <TextLink href={UPDATER_GUIDE} external>
        {t('admin.home.install.manual.docs')}
      </TextLink>
    </div>
  );
}

/**
 * «Update installieren» on the server card (concept 15 P-8, ADR 0024). With the companion a
 * button and its confirmation start the install; the card then shows the companion's phases from
 * getUpdateInstallStatus, and while the server is away the health check like after «Neu starten»
 * until the new version answers, which reloads the page. Without the companion (or a 503) the
 * manual steps stand there instead. The last result stays until a newer install starts; the
 * server decides everything, the card only shows it.
 */
export function UpdateInstall({
  update,
  dashboardAt,
  installAt,
  onInstallAt,
}: {
  update: Dashboard['update'];
  /** When the dashboard that brought `update` answered (ms). */
  dashboardAt: number;
  /** When this page started (or joined) an install, ms; null while it watches none. */
  installAt: number | null;
  onInstallAt: (at: number | null) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // A 503 of installUpdate: the companion went away since the dashboard said otherwise.
  const [unavailable, setUnavailable] = useState(false);
  // A 422: the companion refused before it changed anything; state.json keeps the last install.
  const [refusal, setRefusal] = useState<{ code: string; detail?: string } | null>(null);
  const watching = installAt !== null || isInstalling(update.install);
  // A page that joins a running install (a reload, a second tab) follows it like its own: the
  // health check, and the reload once the new version answers. Once per page.
  const joined = useRef(false);
  useEffect(() => {
    if (joined.current || installAt !== null || !isInstalling(update.install)) return;
    joined.current = true;
    onInstallAt(Date.now());
  }, [installAt, update.install, onInstallAt]);
  // The health check of the restart panel (RestartPanel): who answers, and since when.
  const health = useQuery({
    queryKey: ['update', 'health', installAt],
    queryFn: ({ signal }) => request(api.GET('/healthz', { signal })),
    enabled: installAt !== null,
    refetchInterval: RESTART_POLL_MS,
    retry: false,
    gcTime: 0,
  });
  const away = installAt !== null && health.isError;
  // The server stays away far longer than an install takes: something went wrong on the host.
  const silent = away && health.errorUpdatedAt - installAt >= INSTALL_GIVE_UP_MS;
  const status = useQuery({
    queryKey: installStatusKey,
    queryFn: ({ signal }) => request(api.GET('/admin/update/status', { signal })),
    enabled: watching,
    // The 202's state counts as a fresh answer; while the server is away only the health check asks.
    staleTime: RESTART_POLL_MS,
    refetchInterval: away ? false : RESTART_POLL_MS,
    retry: false,
  });
  // The newer of the two answers: the status while the dashboard waits, the dashboard after it.
  const install =
    status.data !== undefined && status.dataUpdatedAt > dashboardAt ? status.data : update.install;
  const running = isInstalling(install);

  const start = useMutation({
    mutationFn: () => request(api.POST('/admin/update/install')),
    onMutate: () => {
      setRefusal(null);
      setUnavailable(false);
    },
    onSuccess: (state) => {
      const at = Date.now();
      queryClient.setQueryData(installStatusKey, state);
      onInstallAt(at);
    },
    onError: (error) => {
      if (isApiError(error, 409) && error.code === 'update_running') {
        onInstallAt(Date.now());
        void queryClient.invalidateQueries({ queryKey: installStatusKey });
      } else if (isApiError(error, 422)) {
        setRefusal({ code: error.code, detail: error.problem?.detail });
      } else if (isApiError(error, 503)) {
        setUnavailable(true);
      }
    },
  });
  const handled =
    (isApiError(start.error, 409) && start.error.code === 'update_running') ||
    isApiError(start.error, 422) ||
    isApiError(start.error, 503);

  // The page reloads once for the new server: when the new version answers the health check, or
  // when the companion reports the install done (whichever comes first).
  const reloaded = useRef(false);
  const target = install?.to;
  const back =
    installAt !== null &&
    health.data !== undefined &&
    target !== undefined &&
    health.data.version === target &&
    restartedSince(health.data, installAt, health.dataUpdatedAt);
  const ended =
    installAt !== null &&
    status.data !== undefined &&
    !isInstalling(status.data) &&
    status.dataUpdatedAt >= installAt;
  useEffect(() => {
    if (reloaded.current) return;
    if (back || (ended && status.data?.phase === 'done')) {
      reloaded.current = true;
      reloadPage();
      return;
    }
    // Rolled back or failed: the old server runs on; the dashboard asks again and shows it.
    if (ended) {
      onInstallAt(null);
      void queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
    }
  }, [back, ended, status.data, onInstallAt, queryClient]);

  // The button goes away once the install runs: the focus it had moves to the install's state.
  const state = useRef<HTMLDivElement>(null);
  const started = installAt !== null || start.isPending;
  useEffect(() => {
    if (running && started && document.activeElement === document.body) state.current?.focus();
  }, [running, started]);

  const latest = update.available ? update.latestVersion : undefined;
  const refusedForGood = refusal !== null && MANUAL_REFUSALS.includes(refusal.code);
  const manual: ManualReason | null = refusedForGood
    ? 'refused'
    : unavailable || (update.updaterConfigured && !update.updaterAvailable)
      ? 'unavailable'
      : !update.updaterAvailable
        ? 'none'
        : null;
  // A 503 or a refusal for good replaces the install button: its focus moves to the manual steps.
  const manualBlock = useRef<HTMLDivElement>(null);
  const replaced = unavailable || refusedForGood;
  useEffect(() => {
    if (replaced && document.activeElement === document.body) manualBlock.current?.focus();
  }, [replaced]);
  if (!latest && !install && !refusal) return null;

  return (
    <div className={styles.install}>
      {install && install.phase !== 'idle' && (
        <div ref={state} role="status" tabIndex={-1} className={styles.installState}>
          {running ? (
            <>
              <span className={styles.installTitle}>
                {install.to
                  ? t('admin.home.install.running', { version: install.to })
                  : t('admin.home.install.runningNoVersion')}
              </span>
              <InstallSteps install={install} />
              <Text as="span" tone="secondary" variant="caption">
                {away
                  ? t('admin.home.install.down')
                  : t(`admin.home.install.phase.${install.phase}`)}
              </Text>
              {silent && (
                <Notice tone="warning" live={false} title={t('admin.home.install.silent.title')}>
                  <span>{t('admin.home.install.silent.body')}</span>
                  <TextLink href={UPDATER_GUIDE} external>
                    {t('admin.home.install.manual.docs')}
                  </TextLink>
                </Notice>
              )}
            </>
          ) : (
            <InstallResult install={install} />
          )}
        </div>
      )}
      {refusal && (
        <Notice tone="danger" title={t('admin.home.install.refused.title')}>
          <span>{t(refusalKey(refusal.code))}</span>
          {refusal.detail && <span className={styles.installMessage}>{refusal.detail}</span>}
        </Notice>
      )}
      {start.error && !handled ? (
        <Notice tone="danger">{problemMessage(t, start.error)}</Notice>
      ) : null}
      {latest && !running && manual === null && (
        <ConfirmDialog
          trigger={
            <AriaButton variant="primary" icon="download" isPending={start.isPending}>
              {t('admin.home.install.action', { version: latest })}
            </AriaButton>
          }
          title={t('admin.home.install.title', { version: latest })}
          body={t('admin.home.install.body')}
          confirmLabel={t('admin.home.install.confirm')}
          cancelLabel={t('common.cancel')}
          onConfirm={() => start.mutate()}
        >
          {update.notes && (
            // Plain text from the manifest, rendered as text, never as markup. Text that scrolls
            // must take the focus (axe «scrollable-region-focusable»); the arrows then scroll it.
            <div
              role="region"
              aria-label={t('admin.home.install.notes')}
              // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
              tabIndex={0}
              className={styles.notes}
            >
              {update.notes}
            </div>
          )}
        </ConfirmDialog>
      )}
      {latest && !running && manual !== null && (
        <ManualUpdate version={latest} reason={manual} focusRef={manualBlock} />
      )}
    </div>
  );
}

/** A plain release "X.Y.Z" as numbers; undefined for anything else (a pre-release, a build). */
const plainRelease = (v: string | undefined) => {
  const m = v === undefined ? null : /^(\d+)\.(\d+)\.(\d+)$/.exec(v);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : undefined;
};

/** The first companion version that follows the server on its own. */
const SELF_UPDATE_SINCE = [1, 18, 0] as const;

/**
 * Whether the companion answers but comes from before 1.18.0: it does not report its version, or
 * reports a release older than 1.18.0. Such a companion does not update itself and is lifted by
 * hand once (ADR 0024, addendum of 8 Nov 2026). One from 1.18.0 that lags behind the server lifts
 * itself; a development build of the server or the companion is its builder's business.
 */
export function updaterOutdated(update: Dashboard['update']): boolean {
  if (!update.updaterAvailable || plainRelease(update.currentVersion) === undefined) return false;
  if (update.updaterVersion === undefined) return true;
  const companion = plainRelease(update.updaterVersion);
  if (companion === undefined) return false;
  const diff = companion.map((n, i) => n - (SELF_UPDATE_SINCE[i] ?? 0)).find((d) => d !== 0) ?? 0;
  return diff < 0;
}

/** The hint to lift a companion that does not update itself, with the one command for it. */
export function UpdaterOutdated({ version }: { version: string }) {
  const { t } = useTranslation();
  // The line may be missing from an .env of an older install: then it is appended.
  const line = `SLOUCHCAT_UPDATER_VERSION=${version}`;
  const command =
    `cd ${t('admin.home.install.outdated.deployDir')} && ` +
    `(grep -q '^SLOUCHCAT_UPDATER_VERSION=' .env && ` +
    `sed -i 's/^SLOUCHCAT_UPDATER_VERSION=.*/${line}/' .env || echo '${line}' >> .env) && ` +
    'docker compose up -d updater';
  return (
    <Notice tone="warning" live={false} title={t('admin.home.install.outdated.title')}>
      <Command code={command} label={t('admin.home.install.outdated.command')} />
      <Text tone="secondary" variant="caption">
        {t('admin.home.install.outdated.files')}
      </Text>
    </Notice>
  );
}

/** The companion in one line: its version, or that it does not answer; nothing without one. */
export function updaterFact(
  t: TFunction,
  update: Dashboard['update'],
): [string, string] | undefined {
  if (update.updaterAvailable) {
    return [
      t('admin.home.server.updater'),
      update.updaterVersion ?? t('admin.home.server.updaterUnknown'),
    ];
  }
  if (update.updaterConfigured) {
    return [t('admin.home.server.updater'), t('admin.home.server.updaterDown')];
  }
  return undefined;
}

/** The update state in one line: what runs against the newest release. */
export function updateState(t: TFunction, update: Dashboard['update']): string {
  if (update.development) return t('admin.home.server.updateDevelopment');
  if (update.available && update.latestVersion) {
    return t('admin.home.server.updateAvailable', { version: update.latestVersion });
  }
  if (update.error) return t('admin.home.server.updateFailed');
  if (!update.enabled) return t('admin.home.server.updateOff');
  return t('admin.home.server.updateCurrent');
}

export function streamProgress(stream: AdminStream) {
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

/**
 * Both storage paths with what they hold and what is free (F-4: always both, even on the same
 * disk), and the hints of a path more than 90 % full (`hints`, page «Server»).
 */
export function StorageCard({
  storage,
  hints = [],
}: {
  storage: Dashboard['storage'];
  hints?: readonly Hint[];
}) {
  const { t, i18n } = useTranslation();
  const bytes = (value: number) => formatBytes(value, intlLocale(i18n.language));
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
      <HintList hints={hints} />
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
export function ServerLog({ errors }: { errors: Dashboard['errors'] }) {
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
                  <time dateTime={entry.at}>
                    {formatDateTime(entry.at, intlLocale(i18n.language))}
                  </time>
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

/**
 * «Letzte Aktivität»: the newest entries of the activity log (the server sends six), each a
 * sentence with its icon and the time since. The overview shows fewer (`shown`) and links on.
 */
export function ActivityCard({
  activity,
  now,
  shown = ACTIVITY_SHOWN,
  footer,
}: {
  activity: Dashboard['activity'];
  now: number;
  shown?: number;
  footer?: ReactNode;
}) {
  const { t, i18n } = useTranslation();
  const entries: ActivityEntry[] = activity.slice(0, shown).map((event) => {
    const look = activityLook[event.kind] ?? { icon: 'info', tone: 'neutral' };
    return {
      id: String(event.id),
      icon: look.icon,
      tone: look.tone,
      text: activitySentence(t, event, intlLocale(i18n.language)),
      time: relativeTime(event.at, now, intlLocale(i18n.language)),
      dateTime: event.at,
    };
  });
  return (
    <DashCard icon="activity" title={t('admin.home.activity.title')} footer={footer}>
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

/** Hints as a list, each with its dot, title, text and link or commands; nothing without any. */
export function HintList({ hints, label }: { hints: readonly Hint[]; label?: string }) {
  if (hints.length === 0) return null;
  return (
    <ul className={styles.rows} aria-label={label}>
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
  );
}

/**
 * The server's own uptime in seconds at `now` (getHealth), counted on with this browser's clock
 * since it answered, so a clock that differs from the server's does not shift it; the start time
 * only without it.
 */
export function useUptime(now: number, system: SystemInfo | undefined): number {
  const health = useHealth();
  return health.data
    ? health.data.uptimeSeconds + Math.max(0, now - health.dataUpdatedAt) / 1000
    : system
      ? (now - Date.parse(system.startedAt)) / 1000
      : 0;
}

/** «Läuft seit» as a `<time>` with the start of the server. */
export function Uptime({ now, system }: { now: number; system: SystemInfo }) {
  const { t } = useTranslation();
  const uptime = useUptime(now, system);
  return <time dateTime={system.startedAt}>{formatSeconds(t, uptime)}</time>;
}
