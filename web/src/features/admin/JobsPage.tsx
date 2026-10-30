import type { JobRun, JobTypeInfo } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Page, Panel, ProgressBar, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  FormDialog,
  StatTile,
  Tabs,
  type DataColumn,
} from '@/ui/aria';
import { JobScheduleDialog } from './JobScheduleDialog';
import {
  elapsedSeconds,
  formatInterval,
  formatMinute,
  formatSeconds,
  typeName,
  typeKey,
  typeState,
  type TypeState,
} from './jobsFormat';
import {
  cancelJob,
  cancelJobRuns,
  JOBS_POLL_MS,
  jobRunsQuery,
  jobTypesQuery,
  pauseJobType,
  resumeJobType,
  retryJob,
  runJobType,
  type JobRunState,
} from './queries';
import styles from './JobsPage.module.css';
import table from './AdminTablePage.module.css';

/** The groups of the cards, in their order (concept 09, 2.2). */
const GROUPS = ['library', 'metadata', 'delivery', 'system'] as const;

/** The tabs of the runs dialog, in their order. */
const RUN_STATES: readonly JobRunState[] = [
  'running',
  'queued',
  'planned',
  'failed',
  'done',
  'cancelled',
];

/** The tone of the state chip of a card. */
const stateTone = {
  idle: 'neutral',
  queued: 'info',
  running: 'success',
  paused: 'warning',
  stopping: 'warning',
  unknown: 'outline',
} as const satisfies Record<TypeState, 'neutral' | 'info' | 'success' | 'warning' | 'outline'>;

/** A clock that ticks every second while the page shows running durations. */
function useNow(active: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return undefined;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [active]);
  return now;
}

/** The remaining-time text of a running run (concept 09, 6.2). */
function remainingText(t: TFunction, run: JobRun): string {
  if (run.estimatedSecondsLeft !== undefined) {
    return t('admin.jobs.estimate.left', { duration: formatSeconds(t, run.estimatedSecondsLeft) });
  }
  return run.estimate === 'median'
    ? t('admin.jobs.estimate.longer')
    : t('admin.jobs.estimate.computing');
}

/** What the cancel confirmation of a type warns about, by type; "" for the rest. */
function cancelWarning(t: TFunction, type: string): string {
  switch (type) {
    case 'notify.deliver':
      return ` ${t('admin.jobs.confirm.notifyWarning')}`;
    case 'scan.library':
    case 'scan.path':
      return ` ${t('admin.jobs.confirm.scanWarning')}`;
    case 'download.prepare':
      return ` ${t('admin.jobs.confirm.downloadWarning')}`;
    default:
      return '';
  }
}

/**
 * The page "Hintergrundjobs" (concept 09, ADR 0017): the counters and pools, the running runs
 * with progress and remaining time, and one card per job type with its runs, last and next run,
 * and the buttons run, pause or resume, cancel the queued or all runs, schedule and runs. Polled
 * every 5 s like the dashboard, paused while a confirmation or dialog is open; running durations
 * count up between two polls on the client (display only, rule 12).
 */
export function JobsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.jobs.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const [dialogs, setDialogs] = useState(0);
  const overview = useQuery({
    ...jobTypesQuery,
    enabled: allowed,
    refetchInterval: dialogs > 0 ? false : JOBS_POLL_MS,
    refetchOnWindowFocus: dialogs === 0,
  });
  const [message, setMessage] = useState<string | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const data = overview.data;
  const now = useNow(Boolean(data?.types.some((info) => info.counts.running > 0)));
  const onDialog = (open: boolean) => setDialogs((n) => (open ? n + 1 : Math.max(0, n - 1)));

  const refresh = () => queryClient.invalidateQueries({ queryKey: jobTypesQuery.queryKey });
  const settled = async (text: string) => {
    setFailure(null);
    setMessage(text);
    await refresh();
  };
  const failed = (error: unknown) => {
    setMessage(null);
    setFailure(error);
  };
  const run = useMutation({
    mutationFn: runJobType,
    onSuccess: (runs, type) =>
      settled(t('admin.jobs.messages.ran', { name: typeName(t, type), count: runs.length })),
    onError: failed,
  });
  const pause = useMutation({
    mutationFn: pauseJobType,
    onSuccess: (_info, type) =>
      settled(t('admin.jobs.messages.paused', { name: typeName(t, type) })),
    onError: failed,
  });
  const resume = useMutation({
    mutationFn: resumeJobType,
    onSuccess: (_info, type) =>
      settled(t('admin.jobs.messages.resumed', { name: typeName(t, type) })),
    onError: failed,
  });
  const cancelRuns = useMutation({
    mutationFn: ({ type, scope }: { type: string; scope: 'queued' | 'all' }) =>
      cancelJobRuns(type, scope),
    onSuccess: (result, { type }) =>
      settled(
        t('admin.jobs.messages.cancelled', { name: typeName(t, type), count: result.cancelled }),
      ),
    onError: failed,
  });
  const cancelOne = useMutation({
    mutationFn: (job: JobRun) => cancelJob(job.id),
    onSuccess: (_data, job) => settled(t('admin.jobs.messages.runCancelled', { id: job.id })),
    onError: failed,
  });
  const pending = run.isPending || pause.isPending || resume.isPending || cancelRuns.isPending;

  if (!allowed) {
    return (
      <Page>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </Page>
    );
  }

  const running = data?.types.flatMap((info) => info.running) ?? [];
  const paused = data?.types.filter((info) => info.paused) ?? [];
  const totals = {
    running: data?.types.reduce((sum, info) => sum + info.counts.running, 0) ?? 0,
    queued: data?.types.reduce((sum, info) => sum + info.counts.queued, 0) ?? 0,
    planned: data?.types.reduce((sum, info) => sum + info.counts.planned, 0) ?? 0,
    failed: data?.types.reduce((sum, info) => sum + info.counts.failed, 0) ?? 0,
  };
  const stoppingTypes = new Set(
    data?.types.filter((info) => info.stopping).map((info) => info.type),
  );

  return (
    <Page>
      <div className={styles.intro}>
        <Text as="h1" variant="display">
          {t('admin.jobs.title')}
        </Text>
        <Text tone="secondary">{t('admin.jobs.intro')}</Text>
      </div>
      {overview.isPending && <Skeleton shape="block" />}
      {overview.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, overview.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void overview.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {data && (
        <>
          <div className={styles.counters}>
            <StatTile
              label={t('admin.jobs.counts.running')}
              value={String(totals.running)}
              tone={totals.running > 0 ? 'success' : 'neutral'}
            />
            <StatTile label={t('admin.jobs.counts.queued')} value={String(totals.queued)} />
            <StatTile label={t('admin.jobs.counts.planned')} value={String(totals.planned)} />
            <StatTile
              label={t('admin.jobs.counts.paused')}
              value={String(data.paused)}
              tone={data.paused > 0 ? 'warning' : 'neutral'}
            />
            <StatTile
              label={t('admin.jobs.counts.failed')}
              value={String(totals.failed)}
              tone={totals.failed > 0 ? 'danger' : 'neutral'}
            />
          </div>
          <ul className={styles.pools} aria-label={t('admin.jobs.pools.title')}>
            {data.pools.map((pool) => (
              <li key={pool.pool}>
                {t(`admin.jobs.pools.${pool.pool}`)}:{' '}
                {t('admin.jobs.pools.workers', { count: pool.workers })}
                {' · '}
                {pool.backlogSeconds === undefined
                  ? t('admin.jobs.pools.free')
                  : t('admin.jobs.pools.backlog', {
                      duration: formatSeconds(t, pool.backlogSeconds),
                    })}
              </li>
            ))}
          </ul>
          {paused.length > 0 && (
            <Notice
              tone="warning"
              title={t('admin.jobs.pausedBanner.title', { count: paused.length })}
              live={false}
              actions={paused.map((info) => (
                <AriaButton
                  key={info.type}
                  variant="ghost"
                  aria-label={t('admin.jobs.actions.resumeLabel', { name: typeName(t, info.type) })}
                  isDisabled={pending}
                  onPress={() => resume.mutate(info.type)}
                >
                  {t('admin.jobs.actions.resume')}: {typeName(t, info.type)}
                </AriaButton>
              ))}
            >
              {t('admin.jobs.pausedBanner.body', {
                types: paused.map((info) => typeName(t, info.type)).join(', '),
              })}
            </Notice>
          )}
          {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
          <div role="status" className={styles.live}>
            {message && failure === null && <Notice live={false}>{message}</Notice>}
          </div>

          <RunningSection
            runs={running}
            now={now}
            stopping={stoppingTypes}
            pending={pending}
            onPause={(type) => pause.mutate(type)}
            onCancel={(job) => cancelOne.mutate(job)}
            onDialog={onDialog}
          />

          {GROUPS.map((group) => {
            const types = data.types.filter((info) => info.group === group);
            if (types.length === 0) return null;
            return (
              <GroupSection key={group} group={group}>
                {types.map((info) => (
                  <JobCard
                    key={info.type}
                    info={info}
                    now={now}
                    pending={pending}
                    onRun={(type) => run.mutate(type)}
                    onPause={(type) => pause.mutate(type)}
                    onResume={(type) => resume.mutate(type)}
                    onCancelRuns={(type, scope) => cancelRuns.mutate({ type, scope })}
                    onDialog={onDialog}
                    onChanged={() => void refresh()}
                    onMessage={(text) => void settled(text)}
                    onFailure={failed}
                  />
                ))}
              </GroupSection>
            );
          })}
        </>
      )}
    </Page>
  );
}

function GroupSection({
  group,
  children,
}: {
  group: (typeof GROUPS)[number];
  children: React.ReactNode;
}) {
  const { t } = useTranslation();
  const id = useId();
  return (
    <section className={styles.section} aria-labelledby={id}>
      <h2 id={id} className={styles.sectionTitle}>
        {t(`admin.jobs.groups.${group}`)}
      </h2>
      <div className={styles.cards}>{children}</div>
    </section>
  );
}

/** "Läuft gerade": every running run across the types, with pause (of the job) and cancel. */
function RunningSection({
  runs,
  now,
  stopping,
  pending,
  onPause,
  onCancel,
  onDialog,
}: {
  runs: JobRun[];
  now: number;
  stopping: Set<string>;
  pending: boolean;
  onPause: (type: string) => void;
  onCancel: (job: JobRun) => void;
  onDialog: (open: boolean) => void;
}) {
  const { t } = useTranslation();
  const id = useId();
  return (
    <section className={styles.section} aria-labelledby={id}>
      <h2 id={id} className={styles.sectionTitle}>
        {t('admin.jobs.running.title')}
      </h2>
      <Panel>
        {runs.length === 0 ? (
          <Text tone="secondary">{t('admin.jobs.running.empty')}</Text>
        ) : (
          <ul className={styles.runs}>
            {runs.map((job) => {
              const name = typeName(t, job.type);
              const elapsed = elapsedSeconds(job, now);
              const total = job.progress?.total ?? 0;
              const percent =
                total > 0
                  ? Math.min(100, Math.round(((job.progress?.done ?? 0) * 100) / total))
                  : null;
              const isStopping = stopping.has(job.type);
              return (
                <li key={job.id} className={styles.run}>
                  <div className={styles.runBody}>
                    <div className={styles.runName}>
                      <Text as="span">{name}</Text>
                      {job.subject && (
                        <Text as="span" tone="secondary">
                          {job.subject}
                        </Text>
                      )}
                      {isStopping && <Chip tone="warning">{t('admin.jobs.state.stopping')}</Chip>}
                    </div>
                    {job.progress?.note && (
                      <Text as="span" tone="secondary" variant="caption">
                        {job.progress.note}
                      </Text>
                    )}
                    <div className={styles.runMeta}>
                      <span>
                        {t('admin.jobs.running.since', { duration: formatSeconds(t, elapsed) })}
                      </span>
                      {percent !== null && (
                        <span>
                          {t('admin.jobs.running.progress', {
                            done: job.progress?.done ?? 0,
                            total,
                          })}
                        </span>
                      )}
                      <span>{remainingText(t, job)}</span>
                    </div>
                    {percent !== null && (
                      <div className={styles.bar}>
                        <ProgressBar
                          value={percent}
                          label={t('admin.jobs.running.progressLabel', { name, percent })}
                          size="regular"
                        />
                      </div>
                    )}
                  </div>
                  <div className={styles.runActions}>
                    <AriaButton
                      variant="ghost"
                      icon="pause"
                      aria-label={t('admin.jobs.running.pauseLabel', { name })}
                      isDisabled={pending || isStopping}
                      onPress={() => onPause(job.type)}
                    >
                      {t('admin.jobs.running.pause')}
                    </AriaButton>
                    <ConfirmDialog
                      trigger={
                        <AriaButton
                          variant="ghost"
                          icon="trash"
                          aria-label={t('admin.jobs.running.cancelLabel', { id: job.id, name })}
                          isDisabled={pending}
                        >
                          {t('admin.jobs.running.cancel')}
                        </AriaButton>
                      }
                      title={t('admin.jobs.confirm.cancelRunTitle', { name })}
                      body={t('admin.jobs.confirm.cancelRunBody') + cancelWarning(t, job.type)}
                      confirmLabel={t('admin.jobs.confirm.confirm')}
                      cancelLabel={t('admin.jobs.confirm.keep')}
                      tone="danger"
                      onConfirm={() => onCancel(job)}
                      onOpenChange={onDialog}
                    />
                  </div>
                </li>
              );
            })}
          </ul>
        )}
      </Panel>
    </section>
  );
}

/** One job type: its state, four lines, and the buttons. */
function JobCard({
  info,
  now,
  pending,
  onRun,
  onPause,
  onResume,
  onCancelRuns,
  onDialog,
  onChanged,
  onMessage,
  onFailure,
}: {
  info: JobTypeInfo;
  now: number;
  pending: boolean;
  onRun: (type: string) => void;
  onPause: (type: string) => void;
  onResume: (type: string) => void;
  onCancelRuns: (type: string, scope: 'queued' | 'all') => void;
  onDialog: (open: boolean) => void;
  onChanged: () => void;
  onMessage: (text: string) => void;
  onFailure: (error: unknown) => void;
}) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const name = typeName(t, info.type);
  const state = typeState(info);
  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [runsOpen, setRunsOpen] = useState(false);
  const openDialog = (set: (open: boolean) => void) => (open: boolean) => {
    set(open);
    onDialog(open);
  };
  const last = info.lastRun;
  const lastText = () => {
    if (!last) return t('admin.jobs.card.never');
    const at = last.finishedAt ?? last.updatedAt;
    const parts = [
      t('admin.jobs.card.ago', { duration: formatSeconds(t, (now - Date.parse(at)) / 1000) }),
    ];
    if (last.durationSeconds !== undefined) parts.push(formatSeconds(t, last.durationSeconds));
    switch (last.state) {
      case 'failed':
        parts.push(t('admin.jobs.card.lastFailed', { error: last.lastError ?? '' }));
        break;
      case 'cancelled':
        parts.push(t('admin.jobs.card.lastCancelled'));
        break;
      default:
        parts.push(last.skipped ? t('admin.jobs.card.lastSkipped') : t('admin.jobs.card.lastOk'));
    }
    return parts.join(' · ');
  };
  const nextText = () => {
    if (info.paused) return t('admin.jobs.card.pausedNext');
    const schedule = info.schedule;
    if (!schedule)
      return info.trigger === 'manual'
        ? t('admin.jobs.card.triggerManual')
        : t('admin.jobs.card.onDemand');
    if (!schedule.enabled) return t('admin.jobs.card.off');
    const parts = [
      t('admin.jobs.card.in', {
        duration: formatSeconds(t, (Date.parse(schedule.nextRunAt) - now) / 1000),
      }),
      t('admin.jobs.card.every', { duration: formatInterval(t, schedule.everySeconds) }),
    ];
    if (schedule.window) {
      parts.push(
        t('admin.jobs.card.window', {
          start: formatMinute(schedule.window.startMinute),
          end: formatMinute(schedule.window.endMinute),
        }),
      );
    }
    return parts.join(' · ');
  };
  const triggerText = info.known
    ? t(`admin.jobs.types.${typeKey(info.type)}.trigger`, {
        defaultValue:
          info.trigger === 'manual'
            ? t('admin.jobs.card.triggerManual')
            : t('admin.jobs.card.triggerEvent'),
      })
    : t('admin.jobs.card.unknown');
  const queuedText = () => {
    const parts = [t('admin.jobs.card.queuedValue', { count: info.counts.queued })];
    if (info.queuedEstimateSeconds !== undefined && info.counts.queued > 0) {
      parts.push(
        t('admin.jobs.card.queuedEstimate', {
          duration: formatSeconds(t, info.queuedEstimateSeconds),
        }),
      );
    }
    if (info.counts.planned > 0)
      parts.push(t('admin.jobs.card.planned', { count: info.counts.planned }));
    if (info.medianDurationSeconds !== undefined) {
      parts.push(
        t('admin.jobs.card.median', { duration: formatSeconds(t, info.medianDurationSeconds) }),
      );
    }
    return parts.join(' · ');
  };

  return (
    <Panel as="article" aria-labelledby={id}>
      <div className={styles.card}>
        <div className={styles.cardHead}>
          <h3 id={id} className={styles.cardTitle}>
            <span>{name}</span>
            <code className={styles.code}>{info.type}</code>
          </h3>
          <Chip tone={stateTone[state]}>{t(`admin.jobs.state.${state}`)}</Chip>
        </div>
        <dl className={styles.facts}>
          <dt>{t('admin.jobs.card.queued')}</dt>
          <dd>{queuedText()}</dd>
          <dt>{t('admin.jobs.card.lastRun')}</dt>
          <dd>{lastText()}</dd>
          <dt>{t('admin.jobs.card.nextRun')}</dt>
          <dd>{nextText()}</dd>
          <dt>{t('admin.jobs.card.trigger')}</dt>
          <dd>{triggerText}</dd>
        </dl>
        {info.known && (
          <div className={styles.actions}>
            {info.runnable && !info.paused && (
              <AriaButton
                variant="primary"
                icon="play"
                aria-label={t('admin.jobs.actions.runLabel', { name })}
                isDisabled={pending}
                onPress={() => onRun(info.type)}
              >
                {t('admin.jobs.actions.run')}
              </AriaButton>
            )}
            {info.paused ? (
              <AriaButton
                variant="secondary"
                icon="play"
                aria-label={t('admin.jobs.actions.resumeLabel', { name })}
                isDisabled={pending}
                onPress={() => onResume(info.type)}
              >
                {t('admin.jobs.actions.resume')}
              </AriaButton>
            ) : (
              <AriaButton
                variant="secondary"
                icon="pause"
                aria-label={t('admin.jobs.actions.pauseLabel', { name })}
                isDisabled={pending}
                onPress={() => onPause(info.type)}
              >
                {t('admin.jobs.actions.pause')}
              </AriaButton>
            )}
            {info.counts.queued + info.counts.planned > 0 && (
              <ConfirmDialog
                trigger={
                  <AriaButton
                    variant="ghost"
                    aria-label={t('admin.jobs.actions.cancelQueuedLabel', { name })}
                    isDisabled={pending}
                  >
                    {t('admin.jobs.actions.cancelQueued', {
                      count: info.counts.queued + info.counts.planned,
                    })}
                  </AriaButton>
                }
                title={t('admin.jobs.confirm.cancelQueuedTitle', { name })}
                body={
                  t('admin.jobs.confirm.cancelQueuedBody', {
                    count: info.counts.queued + info.counts.planned,
                  }) + cancelWarning(t, info.type)
                }
                confirmLabel={t('admin.jobs.confirm.confirm')}
                cancelLabel={t('admin.jobs.confirm.keep')}
                tone="danger"
                onConfirm={() => onCancelRuns(info.type, 'queued')}
                onOpenChange={onDialog}
              />
            )}
            {info.counts.running > 0 && (
              <ConfirmDialog
                trigger={
                  <AriaButton
                    variant="ghost"
                    aria-label={t('admin.jobs.actions.cancelAllLabel', { name })}
                    isDisabled={pending}
                  >
                    {t('admin.jobs.actions.cancelAll')}
                  </AriaButton>
                }
                title={t('admin.jobs.confirm.cancelAllTitle', { name })}
                body={
                  t('admin.jobs.confirm.cancelAllBody', {
                    queued: info.counts.queued + info.counts.planned,
                    running: info.counts.running,
                  }) + cancelWarning(t, info.type)
                }
                confirmLabel={t('admin.jobs.confirm.confirm')}
                cancelLabel={t('admin.jobs.confirm.keep')}
                tone="danger"
                onConfirm={() => onCancelRuns(info.type, 'all')}
                onOpenChange={onDialog}
              />
            )}
            {info.schedule && (
              <AriaButton
                variant="ghost"
                aria-label={t('admin.jobs.actions.scheduleLabel', { name })}
                onPress={() => openDialog(setScheduleOpen)(true)}
              >
                {t('admin.jobs.actions.schedule')}
              </AriaButton>
            )}
            <AriaButton
              variant="ghost"
              aria-label={t('admin.jobs.actions.runsLabel', { name })}
              onPress={() => openDialog(setRunsOpen)(true)}
            >
              {t('admin.jobs.actions.runs')}
            </AriaButton>
          </div>
        )}
      </div>
      {info.schedule && scheduleOpen && (
        <JobScheduleDialog
          info={info}
          isOpen={scheduleOpen}
          onOpenChange={openDialog(setScheduleOpen)}
          onSaved={() => {
            onMessage(t('admin.jobs.messages.scheduleSaved', { name }));
            onChanged();
          }}
        />
      )}
      {runsOpen && (
        <RunsDialog
          info={info}
          isOpen={runsOpen}
          onOpenChange={openDialog(setRunsOpen)}
          now={now}
          language={i18n.language}
          onChanged={onChanged}
          onMessage={onMessage}
          onFailure={onFailure}
        />
      )}
    </Panel>
  );
}

/** The dialog "Läufe von X": one tab per state, the runs as a table with retry and cancel. */
function RunsDialog({
  info,
  isOpen,
  onOpenChange,
  now,
  language,
  onChanged,
  onMessage,
  onFailure,
}: {
  info: JobTypeInfo;
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  now: number;
  language: string;
  onChanged: () => void;
  onMessage: (text: string) => void;
  onFailure: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const name = typeName(t, info.type);
  const [tab, setTab] = useState<JobRunState>(info.counts.running > 0 ? 'running' : 'queued');
  const listRef = useRef<HTMLDivElement>(null);
  return (
    <FormDialog
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      title={t('admin.jobs.runs.title', { name })}
      size="wide"
      initialFocus="[role=tab][aria-selected=true]"
    >
      <div ref={listRef} className={styles.dialogBody}>
        <Tabs
          label={t('admin.jobs.runs.tabs')}
          selectedKey={tab}
          onSelectionChange={(key) => setTab(key as JobRunState)}
          tabs={RUN_STATES.map((state) => ({
            id: state,
            label: t(`admin.jobs.runs.tab.${state}`),
            content: (
              <RunsTab
                type={info.type}
                state={state}
                now={now}
                language={language}
                onChanged={onChanged}
                onMessage={onMessage}
                onFailure={onFailure}
              />
            ),
          }))}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
            {t('common.close')}
          </AriaButton>
        </div>
      </div>
    </FormDialog>
  );
}

function RunsTab({
  type,
  state,
  now,
  language,
  onChanged,
  onMessage,
  onFailure,
}: {
  type: string;
  state: JobRunState;
  now: number;
  language: string;
  onChanged: () => void;
  onMessage: (text: string) => void;
  onFailure: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const query = useQuery(jobRunsQuery(type, state));
  const done = async (text: string) => {
    onMessage(text);
    await queryClient.invalidateQueries({ queryKey: ['admin', 'job-runs', type] });
    onChanged();
  };
  const retry = useMutation({
    mutationFn: (job: JobRun) => retryJob(job.id),
    onSuccess: (_data, job) => done(t('admin.jobs.messages.runRetried', { id: job.id })),
    onError: onFailure,
  });
  const cancel = useMutation({
    mutationFn: (job: JobRun) => cancelJob(job.id),
    onSuccess: (_data, job) => done(t('admin.jobs.messages.runCancelled', { id: job.id })),
    onError: onFailure,
  });
  const columns: DataColumn<JobRun>[] = [
    {
      key: 'subject',
      header: t('admin.jobs.runs.columns.subject'),
      rowHeader: true,
      cell: (job) => (
        <>
          {job.subject ?? t('admin.jobs.runs.noSubject', { id: job.id })}
          {job.runAfter && state === 'planned' && (
            <>
              {' '}
              <span className={styles.muted}>
                {t('admin.jobs.runs.runAfter', { time: formatDateTime(job.runAfter, language) })}
              </span>
            </>
          )}
        </>
      ),
    },
    {
      key: 'priority',
      header: t('admin.jobs.runs.columns.priority'),
      cell: (job) => <span className={table.numbers}>{job.priority}</span>,
    },
    {
      key: 'started',
      header: t('admin.jobs.runs.columns.started'),
      cell: (job) =>
        job.startedAt ? (
          <time className={styles.nowrap} dateTime={job.startedAt}>
            {formatDateTime(job.startedAt, language)}
          </time>
        ) : (
          '–'
        ),
    },
    {
      key: 'duration',
      header: t('admin.jobs.runs.columns.duration'),
      cell: (job) =>
        job.startedAt ? (
          <span className={styles.nowrap}>{formatSeconds(t, elapsedSeconds(job, now))}</span>
        ) : (
          '–'
        ),
    },
    {
      key: 'attempts',
      header: t('admin.jobs.runs.columns.attempts'),
      cell: (job) => (
        <span className={table.numbers}>
          {t('admin.home.jobs.attempts', { attempts: job.attempts, max: job.maxAttempts })}
        </span>
      ),
    },
    {
      key: 'error',
      header: t('admin.jobs.runs.columns.error'),
      cell: (job) => job.lastError ?? '–',
    },
    {
      key: 'actions',
      header: t('admin.jobs.runs.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (job) => (
        <span className={table.rowActions}>
          {job.state === 'failed' && (
            <AriaButton
              variant="ghost"
              icon="refresh"
              aria-label={t('admin.jobs.runs.retryLabel', { id: job.id })}
              isPending={retry.isPending && retry.variables?.id === job.id}
              onPress={() => retry.mutate(job)}
            >
              {t('admin.jobs.runs.retry')}
            </AriaButton>
          )}
          {(job.state === 'queued' || job.state === 'running') && (
            <AriaButton
              variant="ghost"
              icon="trash"
              aria-label={t('admin.jobs.runs.cancelLabel', { id: job.id })}
              isPending={cancel.isPending && cancel.variables?.id === job.id}
              onPress={() => cancel.mutate(job)}
            >
              {t('admin.jobs.runs.cancel')}
            </AriaButton>
          )}
        </span>
      ),
    },
  ];
  if (query.isPending) return <Skeleton shape="block" />;
  if (query.isError) return <Notice tone="danger">{problemMessage(t, query.error)}</Notice>;
  const page = query.data;
  if (page.runs.length === 0) return <Text tone="secondary">{t('admin.jobs.runs.empty')}</Text>;
  return (
    <>
      <VisuallyHidden>
        {t('admin.jobs.runs.total', { count: page.total, shown: page.runs.length })}
      </VisuallyHidden>
      {page.total > page.runs.length && (
        <Text tone="secondary" variant="caption">
          {t('admin.jobs.runs.total', { count: page.total, shown: page.runs.length })}
        </Text>
      )}
      <DataTable
        caption={t('admin.jobs.runs.title', { name: typeName(t, type) })}
        columns={columns}
        rows={page.runs}
        rowKey={(job) => String(job.id)}
      />
    </>
  );
}
