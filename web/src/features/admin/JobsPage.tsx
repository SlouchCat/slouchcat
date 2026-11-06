import type {
  Dashboard,
  JobRun,
  JobOverview,
  JobScheduleUpdate,
  JobSummary,
  JobTypeInfo,
  JobTypeWindowUpdate,
  JobWindow,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import {
  memo,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type FormEvent,
} from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Chip,
  Notice,
  Page,
  ProgressBar,
  SelectField,
  Skeleton,
  StatusDot,
  Text,
  VisuallyHidden,
} from '@/ui';
import {
  ActionMenu,
  AdminPage,
  AriaButton,
  ConfirmDialog,
  DataTable,
  DisclosureGroup,
  Form,
  StatusRegion,
  StatusStrip,
  Switch,
  Tabs,
  TextField,
  afterDialogsClosed,
  focusTitleIfLost,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type ActionMenuItem,
  type DataColumn,
} from '@/ui/aria';
import {
  elapsedSeconds,
  formatClock,
  formatInterval,
  formatMinute,
  formatOnce,
  formatSeconds,
  typeDescription,
  typeName,
  typeKey,
  typeState,
  type TypeState,
} from './jobsFormat';
import {
  cancelJob,
  cancelJobRuns,
  dashboardQuery,
  JOBS_POLL_MS,
  jobRunsQuery,
  jobTypesQuery,
  pauseJobType,
  resumeJobType,
  retryJob,
  runJobType,
  updateJobSchedule,
  updateJobTypeWindow,
  type JobRunState,
} from './queries';
import styles from './jobs/Jobs.module.css';

/** The groups of the job types, in their order (concept 09, 2.2); one tab each. */
const GROUPS = ['library', 'metadata', 'delivery', 'system'] as const;

/** The tabs of the page: the four groups and the failures of every type (JOB-8). */
const TABS: readonly string[] = [...GROUPS, 'failed'];

/** The tabs of a type's runs, in their order. */
const RUN_STATES: readonly JobRunState[] = [
  'running',
  'queued',
  'planned',
  'failed',
  'done',
  'cancelled',
];

/** The intervals the schedule offers, in seconds; the server's limits of the type filter them. */
const EVERY_OPTIONS = [
  ['m5', 5 * 60],
  ['m15', 15 * 60],
  ['m30', 30 * 60],
  ['h1', 3600],
  ['h2', 2 * 3600],
  ['h6', 6 * 3600],
  ['h12', 12 * 3600],
  ['h24', 24 * 3600],
  ['h48', 48 * 3600],
  ['d7', 7 * 24 * 3600],
] as const;

/** The tone of the state chip of a type. */
const stateTone = {
  idle: 'neutral',
  queued: 'info',
  running: 'success',
  paused: 'warning',
  stopping: 'warning',
  unknown: 'outline',
} as const satisfies Record<TypeState, 'neutral' | 'info' | 'success' | 'warning' | 'outline'>;

/** The schedule of a type as its form holds it; the draft of a type is one of these (S-17). */
interface ScheduleDraft {
  enabled: boolean;
  /** Seconds, as the select's value. */
  every: string;
  windowed: boolean;
  /** "03:00", the value of a time field. */
  start: string;
  end: string;
  /**
   * The one-off date as the server's wall time ("2026-11-08T03:30", the value of a
   * `datetime-local` field), or "" for none. It goes to the server as typed (concept 14, 5.2).
   */
  once: string;
}

function scheduleDraftOf(schedule: NonNullable<JobTypeInfo['schedule']>): ScheduleDraft {
  return {
    enabled: schedule.enabled,
    every: String(schedule.everySeconds),
    windowed: schedule.window !== undefined,
    start: schedule.window ? formatMinute(schedule.window.startMinute) : '03:00',
    end: schedule.window ? formatMinute(schedule.window.endMinute) : '05:00',
    once: schedule.once?.local ?? '',
  };
}

/**
 * The time window of a windowable event type as its form holds it (concept 15, 4.3); the draft
 * of such a type is one of these. Without a window the times start at 01:00–06:00.
 */
interface WindowDraft {
  windowed: boolean;
  /** "01:00", the value of a time field. */
  start: string;
  end: string;
}

function windowDraftOf(window: JobWindow | undefined): WindowDraft {
  return {
    windowed: window !== undefined,
    start: window ? formatMinute(window.startMinute) : '01:00',
    end: window ? formatMinute(window.endMinute) : '06:00',
  };
}

/**
 * Without a window its times do not count: they take the server's values, so a schedule (or a
 * type's window) that ends up as the server holds it is no draft (an hour typed and the window
 * switched off again).
 */
function normaliseSchedule<T extends WindowDraft>(next: T, original: T | undefined): T {
  if (next.windowed || !original) return next;
  return { ...next, start: original.start, end: original.end };
}

/** Minutes of the day of an `<input type="time">` value ("03:00"); null when it is not a time. */
function minuteOf(value: string): number | null {
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  return Number(match[1]) * 60 + Number(match[2]);
}

// One clock for every running duration and "vor X" / "in X" on the page (JOB-4a): it ticks once
// a second while any of them is shown, and only those texts render again.
let clockNow = Date.now();
let clockTimer: number | undefined;
const clockListeners = new Set<() => void>();

function subscribeClock(listener: () => void): () => void {
  clockListeners.add(listener);
  if (clockTimer === undefined) {
    clockNow = Date.now();
    clockTimer = window.setInterval(() => {
      clockNow = Date.now();
      for (const notify of clockListeners) notify();
    }, 1000);
  }
  return () => {
    clockListeners.delete(listener);
    if (clockListeners.size === 0 && clockTimer !== undefined) {
      window.clearInterval(clockTimer);
      clockTimer = undefined;
    }
  };
}

const readClock = () => clockNow;

function useNow(): number {
  return useSyncExternalStore(subscribeClock, readClock, readClock);
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

/** Waiting runs with the estimate of the backlog, planned runs and the usual duration (JOB-5). */
function queuedText(t: TFunction, info: JobTypeInfo): string {
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
  return parts.join(t('admin.jobs.separator'));
}

/** The last run: when, how long, and how it ended (with the whole error). */
function lastText(t: TFunction, info: JobTypeInfo, now: number): string {
  const last = info.lastRun;
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
  return parts.join(t('admin.jobs.separator'));
}

/**
 * The next run: "in X · alle Y · window", or why there is none; then the one-off date
 * ("Termin: So., 8. Nov., 03:30"), which fires with the schedule switched off as well. A paused
 * type lets its date lapse (concept 14, 6.7). An event type with a window says so; while the
 * server names the window closed and runs wait, until when (concept 15, 4.3). The pause wins.
 */
function nextText(t: TFunction, info: JobTypeInfo, now: number, language: string): string {
  if (info.paused) return t('admin.jobs.card.pausedNext');
  const schedule = info.schedule;
  if (!schedule) {
    if (info.trigger === 'manual') return t('admin.jobs.card.triggerManual');
    if (!info.window) return t('admin.jobs.card.automatic');
    if (info.windowOpensAt && info.counts.queued + info.counts.planned > 0) {
      return t('admin.jobs.card.waitsUntil', {
        time: formatClock(info.windowOpensAt, info.windowTimeZone, language),
      });
    }
    return [
      t('admin.jobs.card.automatic'),
      t('admin.jobs.card.window', {
        start: formatMinute(info.window.startMinute),
        end: formatMinute(info.window.endMinute),
      }),
    ].join(t('admin.jobs.separator'));
  }
  const parts: string[] = [];
  if (!schedule.enabled) {
    parts.push(t('admin.jobs.card.off'));
  } else {
    parts.push(
      t('admin.jobs.card.in', {
        duration: formatSeconds(t, (Date.parse(schedule.nextRunAt) - now) / 1000),
      }),
      t('admin.jobs.card.every', { duration: formatInterval(t, schedule.everySeconds) }),
    );
    if (schedule.window) {
      parts.push(
        t('admin.jobs.card.window', {
          start: formatMinute(schedule.window.startMinute),
          end: formatMinute(schedule.window.endMinute),
        }),
      );
    }
  }
  if (schedule.once) {
    parts.push(t('admin.jobs.card.once', { date: formatOnce(schedule.once.local, language) }));
  }
  return parts.join(t('admin.jobs.separator'));
}

/** The last run of a type; in a row cut to one line with the whole text in the tooltip (S-5). */
function LastRun({ info, oneLine = false }: { info: JobTypeInfo; oneLine?: boolean }) {
  const { t } = useTranslation();
  const text = lastText(t, info, useNow());
  if (!oneLine) return <span className={styles.wrap}>{text}</span>;
  return (
    <span className={styles.oneLine} title={text}>
      {text}
    </span>
  );
}

function NextRun({ info }: { info: JobTypeInfo }) {
  const { t, i18n } = useTranslation();
  return <>{nextText(t, info, useNow(), i18n.language)}</>;
}

/** The duration of a run, counting up while it runs. */
function Elapsed({ run, since = false }: { run: JobRun; since?: boolean }) {
  const { t } = useTranslation();
  const duration = formatSeconds(t, elapsedSeconds(run, useNow()));
  return (
    <span className={styles.nowrap}>
      {since ? t('admin.jobs.running.since', { duration }) : duration}
    </span>
  );
}

/** A long error in a table: the whole text, wrapped (S-5: reachable without a pointer). */
function ErrorText({ error }: { error: string | undefined }) {
  const { t } = useTranslation();
  if (!error) return <>{t('admin.jobs.none')}</>;
  return <span className={styles.wrap}>{error}</span>;
}

/** A confirmation the page opens itself (from a menu or a row that may go away meanwhile). */
interface Confirmation {
  title: string;
  body: string;
  onConfirm: () => void;
}

/** What a page-level action of a type is doing right now, for the button's pending state. */
type Busy = 'run' | 'pause' | 'resume' | null;

/** The handlers of a fold-out, stable across renders so the fold-outs need not render again. */
interface JobHandlers {
  onRun: (type: string) => void;
  onPause: (type: string) => void;
  onResume: (type: string) => void;
  onAsk: (confirmation: Confirmation) => void;
  onCancelRuns: (type: string, scope: 'queued' | 'all') => void;
  onScheduleChange: (type: string, next: ScheduleDraft) => void;
  onScheduleDiscard: (type: string) => void;
  /** The save of `sent` went through. */
  onScheduleSaved: (type: string, sent: ScheduleDraft) => Promise<void>;
  onWindowChange: (type: string, next: WindowDraft) => void;
  onWindowDiscard: (type: string) => void;
  /** The save of `sent` went through. */
  onWindowSaved: (type: string, sent: WindowDraft) => Promise<void>;
  onRunSettled: (text: string) => void;
  onFailure: (error: unknown) => void;
}

/**
 * The page "Hintergrundjobs" (concept 09, ADR 0017; pattern of concept 12, variant C): the paused
 * types, the counters, «Läuft gerade» with the worker pools, then one tab per group with a row
 * per job type that folds out into its actions, its schedule and its runs, and the tab
 * «Fehlschläge» with the ten newest failures of every type (JOB-8). Polled every 5 s, paused
 * while a confirmation is open or a schedule has unsaved changes; running durations count up
 * between two polls on the client (display only, rule 12).
 */
export function JobsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.jobs.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <Page>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </Page>
    );
  }
  return <JobsContent />;
}

function JobsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);

  // The drafts need what the server holds before the polling query knows whether to pause for
  // them: a passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...jobTypesQuery, enabled: false }).data;
  const originals = useMemo(
    () =>
      new Map(
        (cached?.types ?? []).flatMap((info) =>
          info.schedule ? [[info.type, scheduleDraftOf(info.schedule)] as const] : [],
        ),
      ),
    [cached],
  );
  const originalOf = useCallback((type: string) => originals.get(type), [originals]);
  const drafts = useDrafts<ScheduleDraft>(originalOf);
  const { set: setDraft, discard: discardDraft } = drafts;
  // The windowable event types have a window instead of a schedule (concept 15, 4.3).
  const windowOriginals = useMemo(
    () =>
      new Map(
        (cached?.types ?? []).flatMap((info) =>
          info.windowable ? [[info.type, windowDraftOf(info.window)] as const] : [],
        ),
      ),
    [cached],
  );
  const windowOriginalOf = useCallback(
    (type: string) => windowOriginals.get(type),
    [windowOriginals],
  );
  const windowDrafts = useDrafts<WindowDraft>(windowOriginalOf);
  const { set: setWindowDraft, discard: discardWindowDraft } = windowDrafts;
  const unsaved = useUnsavedChanges(drafts.dirty || windowDrafts.dirty);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const hold = confirmOpen || unsaved.pausePolling;

  // An unknown `?tab=` shows the first tab; the address says so too.
  const [tab, selectTab] = useSearchSelection('tab', TABS, 'library', { normalise: true });

  const overview = useQuery({
    ...jobTypesQuery,
    refetchInterval: hold ? false : JOBS_POLL_MS,
    refetchOnWindowFocus: !hold,
  });
  // The failures come with the dashboard; the page asks it only while their tab is open, and
  // the sidebar's slower poll of the same entry does the rest.
  const failuresOpen = tab === 'failed';
  const dashboard = useQuery({
    ...dashboardQuery,
    enabled: failuresOpen,
    refetchInterval: failuresOpen && !hold ? JOBS_POLL_MS : false,
    refetchOnWindowFocus: failuresOpen && !hold,
  });

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const onExpandedChange = useCallback(
    (key: string, open: boolean) =>
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      }),
    [],
  );
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const data = overview.data;

  // `?item=` opens that type's row once, when the list knows it (the link of the libraries page
  // to «Keyframe-Index nachholen»), and shows its group; folding it away is the reader's choice.
  const search: Record<string, unknown> = useSearch({ strict: false });
  const askedItem = typeof search.item === 'string' ? search.item : undefined;
  const askedInfo =
    askedItem === undefined ? undefined : data?.types.find((info) => info.type === askedItem);
  const [openedItem, setOpenedItem] = useState<string | undefined>(undefined);
  // Adjusted while rendering (no effect): the row is open in the same render that shows it.
  if (askedInfo && openedItem !== askedInfo.type) {
    setOpenedItem(askedInfo.type);
    setExpanded((current) => new Set(current).add(askedInfo.type));
  } else if (askedItem === undefined && openedItem !== undefined) {
    setOpenedItem(undefined);
  }
  const askedGroup = askedInfo?.group;
  useEffect(() => {
    if (openedItem !== undefined && askedGroup !== undefined) selectTab(askedGroup);
  }, [openedItem, askedGroup, selectTab]);

  /**
   * Shows the outcome and fetches again. Once React has rendered the answer, the focus moves to
   * the page's h1 when the row that had it is gone (a retried or cancelled run, ALL-2) or when
   * the button that had it went with the change.
   */
  const settled = useCallback(
    async (text: string, removed = false) => {
      setFailure(null);
      setMessage(text);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: jobTypesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      ]);
      await afterDialogsClosed();
      if (removed) titleRef.current?.focus();
      else focusTitleIfLost(titleRef);
    },
    [queryClient, setMessage],
  );
  const failed = useCallback(
    (error: unknown) => {
      setMessage(null);
      setFailure(error);
    },
    [setMessage],
  );
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
    mutationFn: ({ type }: { type: string; fromNotice: boolean }) => resumeJobType(type),
    // The notice's button goes with the last paused type: the h1 takes the focus.
    onSuccess: (_info, { type, fromNotice }) =>
      settled(t('admin.jobs.messages.resumed', { name: typeName(t, type) }), fromNotice),
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
    mutationFn: (job: JobRun | JobSummary) => cancelJob(job.id),
    onSuccess: (_data, job) => settled(t('admin.jobs.messages.runCancelled', { id: job.id }), true),
    onError: failed,
  });
  const retryOne = useMutation({
    mutationFn: (job: JobSummary) => retryJob(job.id),
    onSuccess: (_data, job) => settled(t('admin.jobs.messages.runRetried', { id: job.id }), true),
    onError: failed,
  });
  const pending =
    run.isPending ||
    pause.isPending ||
    resume.isPending ||
    cancelRuns.isPending ||
    cancelOne.isPending ||
    retryOne.isPending;
  const busyOf = (type: string): Busy => {
    if (run.isPending && run.variables === type) return 'run';
    if (pause.isPending && pause.variables === type) return 'pause';
    if (resume.isPending && resume.variables?.type === type) return 'resume';
    return null;
  };

  // Every confirmation is the page's own: a row that goes while it is open (a run ends, a
  // failure drops out of the newest ten) cannot take the dialog, and the polling pause with it.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);
  const { mutate: runMutate } = run;
  const { mutate: pauseMutate } = pause;
  const { mutate: resumeMutate } = resume;
  const { mutate: cancelRunsMutate } = cancelRuns;
  const { mutate: cancelOneMutate } = cancelOne;
  const { mutate: retryOneMutate } = retryOne;
  const askCancelRun = useCallback(
    (job: JobRun) => {
      const name = typeName(t, job.type);
      ask({
        title: t('admin.jobs.confirm.cancelRunTitle', { name }),
        body: t('admin.jobs.confirm.cancelRunBody') + cancelWarning(t, job.type),
        onConfirm: () => cancelOneMutate(job),
      });
    },
    [ask, t, cancelOneMutate],
  );
  const askCancelFailed = useCallback(
    (job: JobSummary) =>
      ask({
        title: t('admin.jobs.confirm.cancelRunTitle', { name: typeName(t, job.type) }),
        body: t('admin.jobs.confirm.cancelFailedBody'),
        onConfirm: () => cancelOneMutate(job),
      }),
    [ask, t, cancelOneMutate],
  );
  const pauseType = useCallback((type: string) => pauseMutate(type), [pauseMutate]);

  const handlers = useMemo<JobHandlers>(
    () => ({
      onRun: (type) => runMutate(type),
      onPause: (type) => pauseMutate(type),
      onResume: (type) => resumeMutate({ type, fromNotice: false }),
      onAsk: ask,
      onCancelRuns: (type, scope) => cancelRunsMutate({ type, scope }),
      onScheduleChange: (type, next) => setDraft(type, normaliseSchedule(next, originalOf(type))),
      onScheduleDiscard: discardDraft,
      onScheduleSaved: async (type, sent) => {
        await settled(t('admin.jobs.messages.scheduleSaved', { name: typeName(t, type) }));
        // The draft is spent when it still says what was sent; what was typed while the save
        // ran stays a draft.
        setDraft(type, (current) =>
          JSON.stringify(current) === JSON.stringify(sent)
            ? (originalOf(type) ?? current)
            : current,
        );
      },
      onWindowChange: (type, next) =>
        setWindowDraft(type, normaliseSchedule(next, windowOriginalOf(type))),
      onWindowDiscard: discardWindowDraft,
      onWindowSaved: async (type, sent) => {
        await settled(t('admin.jobs.messages.windowSaved', { name: typeName(t, type) }));
        setWindowDraft(type, (current) =>
          JSON.stringify(current) === JSON.stringify(sent)
            ? (windowOriginalOf(type) ?? current)
            : current,
        );
      },
      onRunSettled: (text) => void settled(text, true),
      onFailure: failed,
    }),
    [
      runMutate,
      pauseMutate,
      resumeMutate,
      ask,
      cancelRunsMutate,
      setDraft,
      originalOf,
      discardDraft,
      setWindowDraft,
      windowOriginalOf,
      discardWindowDraft,
      settled,
      t,
      failed,
    ],
  );

  const columns = useMemo(() => typeColumns(t), [t]);
  const stopping = useMemo(
    () => new Set(data?.types.filter((info) => info.stopping).map((info) => info.type)),
    [data],
  );
  const running = data?.types.flatMap((info) => info.running) ?? [];
  const paused = data?.types.filter((info) => info.paused) ?? [];
  // One source for the failures: the job types' counts, polled every 5 s. They count the same
  // rows as `JobCounts.failed` of the dashboard (state failed).
  const totals = {
    running: data?.types.reduce((sum, info) => sum + info.counts.running, 0) ?? 0,
    queued: data?.types.reduce((sum, info) => sum + info.counts.queued, 0) ?? 0,
    planned: data?.types.reduce((sum, info) => sum + info.counts.planned, 0) ?? 0,
    failed: data?.types.reduce((sum, info) => sum + info.counts.failed, 0) ?? 0,
  };

  const status = data && (
    <>
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
              isPending={busyOf(info.type) === 'resume'}
              onPress={() => resumeMutate({ type: info.type, fromNotice: true })}
            >
              {t('admin.jobs.actions.resumeNamed', { name: typeName(t, info.type) })}
            </AriaButton>
          ))}
        >
          {t('admin.jobs.pausedBanner.body', {
            types: paused.map((info) => typeName(t, info.type)).join(', '),
          })}
          {paused
            // Runs a playback requests start and keep running while the type is paused (concept 14).
            .filter((info) => info.overridesPauseForPlayback === true)
            .map(
              (info) =>
                ` ${t('admin.jobs.pausedBanner.playbackOverride', { name: typeName(t, info.type) })}`,
            )}
        </Notice>
      )}
      <StatusStrip
        label={t('admin.jobs.counts.label')}
        tiles={[
          {
            key: 'running',
            label: t('admin.jobs.counts.running'),
            value: number.format(totals.running),
            tone: totals.running > 0 ? 'success' : 'neutral',
          },
          {
            key: 'queued',
            label: t('admin.jobs.counts.queued'),
            value: number.format(totals.queued),
          },
          {
            key: 'planned',
            label: t('admin.jobs.counts.planned'),
            value: number.format(totals.planned),
          },
          {
            key: 'paused',
            label: t('admin.jobs.counts.paused'),
            value: number.format(data.paused),
            tone: data.paused > 0 ? 'warning' : 'neutral',
          },
          {
            key: 'failed',
            label: t('admin.jobs.counts.failed'),
            value: number.format(totals.failed),
            tone: totals.failed > 0 ? 'danger' : 'neutral',
          },
        ]}
      />
    </>
  );

  const detail = (info: JobTypeInfo) => (
    <JobDetail
      info={info}
      pending={pending}
      busy={busyOf(info.type)}
      hold={hold}
      schedule={drafts.valueOf(info.type)}
      scheduleDirty={drafts.isDirty(info.type)}
      windowDraft={windowDrafts.valueOf(info.type)}
      windowDirty={windowDrafts.isDirty(info.type)}
      handlers={handlers}
    />
  );

  return (
    <AdminPage
      title={t('admin.jobs.title')}
      help={t('admin.jobs.intro')}
      liveSeconds={hold ? undefined : JOBS_POLL_MS / 1000}
      actions={
        hold ? (
          <span className={styles.livePaused}>
            <StatusDot state="warning" />
            {t('admin.jobs.livePaused')}
          </span>
        ) : undefined
      }
      titleRef={titleRef}
      status={status}
    >
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
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {data && (
        <>
          <DisclosureGroup
            level={2}
            title={t('admin.jobs.running.title')}
            count={number.format(running.length)}
            countLabel={t('admin.jobs.running.count', { count: running.length })}
          >
            {/* The first work row of the page (docs/perf/verwaltungsseiten.md). */}
            <div data-perf="first-work-row">
              <RunningTable
                runs={running}
                stopping={stopping}
                pausing={(type) => busyOf(type) === 'pause'}
                onPause={pauseType}
                onCancel={askCancelRun}
              />
            </div>
            <Pools data={data} />
          </DisclosureGroup>
          <Tabs
            label={t('admin.jobs.tabs.label')}
            selectedKey={tab}
            onSelectionChange={selectTab}
            tabs={[
              ...GROUPS.map((group) => {
                const types = data.types.filter((info) => info.group === group);
                const groupName = t(`admin.jobs.groups.${group}`);
                return {
                  id: group,
                  label: groupName,
                  count: number.format(types.length),
                  countLabel: t('admin.jobs.tabs.types', { count: types.length }),
                  content: (
                    <div className={styles.tabBody}>
                      <VisuallyHidden as="h2">{groupName}</VisuallyHidden>
                      {types.length === 0 ? (
                        <Text tone="secondary">{t('admin.jobs.tabs.empty')}</Text>
                      ) : (
                        <DataTable
                          caption={t('admin.jobs.tabs.caption', { group: groupName })}
                          columns={columns}
                          rows={types}
                          rowKey={(info) => info.type}
                          expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
                          layout="fixed"
                        />
                      )}
                    </div>
                  ),
                };
              }),
              {
                id: 'failed',
                label: t('admin.jobs.tabs.failed'),
                count: number.format(totals.failed),
                countLabel: t('admin.jobs.failed.count', { count: totals.failed }),
                tone: totals.failed > 0 ? 'danger' : 'neutral',
                content: (
                  <FailedTab
                    query={dashboard}
                    total={totals.failed}
                    retrying={(id) => retryOne.isPending && retryOne.variables?.id === id}
                    onRetry={retryOneMutate}
                    onCancel={askCancelFailed}
                  />
                ),
              },
            ]}
          />
        </>
      )}
      {confirmation && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={confirmation.title}
          body={confirmation.body}
          confirmLabel={t('admin.jobs.confirm.confirm')}
          cancelLabel={t('admin.jobs.confirm.keep')}
          tone="danger"
          onConfirm={confirmation.onConfirm}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}

/**
 * What a type does, in its own column of the row (concept 15, P-2): at most two muted lines cut
 * with «…» and the whole sentence in the tooltip; empty for an unknown type. The fold-out reads
 * the sentence in full at its top.
 */
function TypeDescription({ info }: { info: JobTypeInfo }) {
  const { t } = useTranslation();
  const description = typeDescription(t, info);
  if (!description) return null;
  return (
    <span className={styles.description} title={description}>
      {description}
    </span>
  );
}

/**
 * The columns of a group's table: the row folds out on the name (JOB-5). The widths keep the
 * columns in place across the group tabs and when a state changes (K12-9). The description takes
 * what is left, the largest share (concept 15, P-2); the name keeps 12.5rem so its longest word
 * fits at 768 px, and the state its chip.
 */
function typeColumns(t: TFunction): DataColumn<JobTypeInfo>[] {
  return [
    {
      key: 'name',
      width: '12.5rem',
      header: t('admin.jobs.columns.job'),
      rowHeader: true,
      cell: (info) => typeName(t, info.type),
    },
    {
      key: 'description',
      header: t('admin.jobs.columns.description'),
      cell: (info) => <TypeDescription info={info} />,
    },
    {
      key: 'state',
      width: '7rem',
      header: t('admin.jobs.columns.state'),
      cell: (info) => {
        const state = typeState(info);
        return <Chip tone={stateTone[state]}>{t(`admin.jobs.state.${state}`)}</Chip>;
      },
    },
    {
      key: 'queued',
      width: '11%',
      header: t('admin.jobs.card.queued'),
      cell: (info) => queuedText(t, info),
    },
    {
      key: 'last',
      width: '12%',
      header: t('admin.jobs.card.lastRun'),
      // Cut to one line here; the fold-out shows the whole text.
      cell: (info) => <LastRun info={info} oneLine />,
    },
    {
      key: 'next',
      width: '16%',
      header: t('admin.jobs.card.nextRun'),
      cell: (info) => <NextRun info={info} />,
    },
  ];
}

/** The worker pools below «Läuft gerade»: workers and backlog (JOB-2). */
function Pools({ data }: { data: JobOverview }) {
  const { t } = useTranslation();
  return (
    <ul className={styles.pools} aria-label={t('admin.jobs.pools.title')}>
      {data.pools.map((pool) => (
        <li key={pool.pool}>
          {t('admin.jobs.pools.line', {
            pool: t(`admin.jobs.pools.${pool.pool}`),
            workers: t('admin.jobs.pools.workers', { count: pool.workers }),
            state:
              pool.backlogSeconds === undefined
                ? t('admin.jobs.pools.free')
                : t('admin.jobs.pools.backlog', {
                    duration: formatSeconds(t, pool.backlogSeconds),
                  }),
          })}
        </li>
      ))}
    </ul>
  );
}

/** «Läuft gerade»: every running run across the types, with pause (of the job) and cancel. */
function RunningTable({
  runs,
  stopping,
  pausing,
  onPause,
  onCancel,
}: {
  runs: JobRun[];
  stopping: Set<string>;
  pausing: (type: string) => boolean;
  onPause: (type: string) => void;
  onCancel: (job: JobRun) => void;
}) {
  const { t } = useTranslation();
  if (runs.length === 0) return <Text tone="secondary">{t('admin.jobs.running.empty')}</Text>;
  const columns: DataColumn<JobRun>[] = [
    {
      key: 'job',
      header: t('admin.jobs.running.columns.job'),
      rowHeader: true,
      cell: (job) => (
        <span className={styles.runName}>
          {typeName(t, job.type)}
          {stopping.has(job.type) && <Chip tone="warning">{t('admin.jobs.state.stopping')}</Chip>}
          {(job.overridesPause || job.overridesWindow) && (
            <Chip tone="info">
              {job.overridesPause
                ? t('admin.jobs.running.overridesPause')
                : t('admin.jobs.running.overridesWindow')}
            </Chip>
          )}
        </span>
      ),
    },
    {
      key: 'subject',
      header: t('admin.jobs.running.columns.subject'),
      cell: (job) => job.subject ?? t('admin.jobs.none'),
    },
    {
      key: 'progress',
      header: t('admin.jobs.running.columns.progress'),
      cell: (job) => {
        const total = job.progress?.total ?? 0;
        const done = job.progress?.done ?? 0;
        const percent = total > 0 ? Math.min(100, Math.round((done * 100) / total)) : null;
        return (
          <span className={styles.progress}>
            {job.progress?.note && <span className={styles.note}>{job.progress.note}</span>}
            {percent !== null && (
              <>
                <span>
                  {t(`admin.jobs.types.${typeKey(job.type)}.progress`, {
                    done,
                    total,
                    defaultValue: t('admin.jobs.running.progress', { done, total }),
                  })}
                </span>
                <ProgressBar
                  value={percent}
                  label={t('admin.jobs.running.progressLabel', {
                    name: typeName(t, job.type),
                    percent,
                  })}
                  size="regular"
                />
              </>
            )}
            {!job.progress?.note && percent === null && t('admin.jobs.none')}
          </span>
        );
      },
    },
    {
      key: 'since',
      header: t('admin.jobs.running.columns.since'),
      cell: (job) => <Elapsed run={job} since />,
    },
    {
      key: 'left',
      header: t('admin.jobs.running.columns.left'),
      cell: (job) => remainingText(t, job),
    },
    {
      key: 'actions',
      header: t('admin.jobs.runs.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (job) => {
        const name = typeName(t, job.type);
        return (
          <span className={styles.rowActions}>
            <AriaButton
              variant="ghost"
              icon="pause"
              aria-label={t('admin.jobs.running.pauseLabel', { name })}
              isDisabled={stopping.has(job.type)}
              isPending={pausing(job.type)}
              onPress={() => onPause(job.type)}
            >
              {t('admin.jobs.running.pause')}
            </AriaButton>
            <AriaButton
              variant="ghost"
              icon="trash"
              aria-label={t('admin.jobs.running.cancelLabel', { id: job.id, name })}
              onPress={() => onCancel(job)}
            >
              {t('admin.jobs.running.cancel')}
            </AriaButton>
          </span>
        );
      },
    },
  ];
  return (
    <DataTable
      caption={t('admin.jobs.running.title')}
      columns={columns}
      rows={runs}
      rowKey={(job) => String(job.id)}
    />
  );
}

/**
 * The fold-out of a job type: «Jetzt ausführen» and «Pausieren»/«Fortsetzen» as buttons, the
 * cancellations in «Weitere Aktionen» (S-6), what the type does (concept 15, P-2), trigger and
 * last run in full; then the schedule (types with one) or, for an event type, why it has none
 * (P-3) with its time window where it may have one (4.3), and the runs (JOB-6, JOB-7). A type
 * this server no longer knows offers
 * nothing. Its handlers are stable, so it renders again only when its own data changes.
 */
const JobDetail = memo(function JobDetail({
  info,
  pending,
  busy,
  hold,
  schedule,
  scheduleDirty,
  windowDraft,
  windowDirty,
  handlers,
}: {
  info: JobTypeInfo;
  /** Some action of the page is under way: the menu's items wait. */
  pending: boolean;
  busy: Busy;
  hold: boolean;
  schedule: ScheduleDraft | undefined;
  scheduleDirty: boolean;
  windowDraft: WindowDraft | undefined;
  windowDirty: boolean;
  handlers: JobHandlers;
}) {
  const { t } = useTranslation();
  const name = typeName(t, info.type);
  const description = typeDescription(t, info);
  const waiting = info.counts.queued + info.counts.planned;
  const triggerText = info.known
    ? t(`admin.jobs.types.${typeKey(info.type)}.trigger`, {
        defaultValue:
          info.trigger === 'manual'
            ? t('admin.jobs.card.triggerManual')
            : t('admin.jobs.card.triggerEvent'),
      })
    : t('admin.jobs.card.unknown');
  // The menu stays while the queue empties between two polls, so the focus on its button does
  // not get lost; what has nothing to cancel, or waits for another action, is disabled.
  const menu: ActionMenuItem[] = [
    {
      id: 'queued',
      label: t('admin.jobs.actions.cancelQueued', { count: waiting }),
      danger: true,
      disabled: pending || waiting === 0,
    },
    {
      id: 'all',
      label: t('admin.jobs.actions.cancelAll'),
      danger: true,
      disabled: pending || info.counts.running === 0,
    },
  ];
  const onMenu = (id: string) => {
    if (id === 'queued') {
      handlers.onAsk({
        title: t('admin.jobs.confirm.cancelQueuedTitle', { name }),
        body:
          t('admin.jobs.confirm.cancelQueuedBody', { count: waiting }) +
          cancelWarning(t, info.type),
        onConfirm: () => handlers.onCancelRuns(info.type, 'queued'),
      });
    } else if (id === 'all') {
      handlers.onAsk({
        title: t('admin.jobs.confirm.cancelAllTitle', { name }),
        body:
          t('admin.jobs.confirm.cancelAllBody', { queued: waiting, running: info.counts.running }) +
          cancelWarning(t, info.type),
        onConfirm: () => handlers.onCancelRuns(info.type, 'all'),
      });
    }
  };

  return (
    <div className={styles.detail}>
      {description && <p className={styles.detailDescription}>{description}</p>}
      <div className={styles.detailHead}>
        {info.known && (
          <div className={styles.actions}>
            {info.runnable && !info.paused && (
              <AriaButton
                variant="primary"
                icon="play"
                aria-label={t('admin.jobs.actions.runLabel', { name })}
                isPending={busy === 'run'}
                onPress={() => handlers.onRun(info.type)}
              >
                {t('admin.jobs.actions.run')}
              </AriaButton>
            )}
            {info.paused ? (
              <AriaButton
                variant="secondary"
                icon="play"
                aria-label={t('admin.jobs.actions.resumeLabel', { name })}
                isPending={busy === 'resume'}
                onPress={() => handlers.onResume(info.type)}
              >
                {t('admin.jobs.actions.resume')}
              </AriaButton>
            ) : (
              <AriaButton
                variant="secondary"
                icon="pause"
                aria-label={t('admin.jobs.actions.pauseLabel', { name })}
                isPending={busy === 'pause'}
                onPress={() => handlers.onPause(info.type)}
              >
                {t('admin.jobs.actions.pause')}
              </AriaButton>
            )}
            <ActionMenu rowName={name} items={menu} onAction={onMenu} />
          </div>
        )}
        <dl className={styles.facts}>
          <dt>{t('admin.jobs.card.trigger')}</dt>
          <dd>
            {triggerText} <code className={styles.code}>{info.type}</code>
          </dd>
          <dt>{t('admin.jobs.card.lastRun')}</dt>
          <dd>
            <LastRun info={info} />
          </dd>
        </dl>
      </div>
      {info.known && (
        <div className={styles.blocks}>
          {info.schedule && schedule && (
            <ScheduleBlock
              info={info}
              value={schedule}
              dirty={scheduleDirty}
              onChange={handlers.onScheduleChange}
              onDiscard={handlers.onScheduleDiscard}
              onSaved={handlers.onScheduleSaved}
            />
          )}
          {info.trigger === 'event' &&
            (info.windowable && windowDraft ? (
              <WindowBlock
                info={info}
                value={windowDraft}
                dirty={windowDirty}
                onChange={handlers.onWindowChange}
                onDiscard={handlers.onWindowDiscard}
                onSaved={handlers.onWindowSaved}
              />
            ) : (
              <section className={styles.block}>
                <h3 className={styles.blockTitle}>{t('admin.jobs.schedule.heading')}</h3>
                <Text tone="secondary">{t('admin.jobs.schedule.eventNote')}</Text>
              </section>
            ))}
          <RunsBlock
            info={info}
            hold={hold}
            onSettled={handlers.onRunSettled}
            onFailure={handlers.onFailure}
          />
        </div>
      )}
    </div>
  );
});

/**
 * The schedule of a type inside its row (concept 09, F-4; instead of the dialog of concept 09):
 * the switch, the interval as a select of the values within the type's limits, and a time
 * window. What is typed is the page's draft of the type (S-17), so folding the row or switching
 * tabs keeps it. The server checks the limits (rule 12); a 422 names the field until it changes.
 */
function ScheduleBlock({
  info,
  value,
  dirty,
  onChange,
  onDiscard,
  onSaved,
}: {
  info: JobTypeInfo;
  value: ScheduleDraft;
  dirty: boolean;
  onChange: (type: string, next: ScheduleDraft) => void;
  onDiscard: (type: string) => void;
  onSaved: (type: string, sent: ScheduleDraft) => Promise<void>;
}) {
  const { t } = useTranslation();
  const name = typeName(t, info.type);
  const saveButton = useRef<HTMLButtonElement>(null);
  const onceInput = useRef<HTMLInputElement>(null);
  const save = useMutation({
    mutationFn: ({ body }: { body: JobScheduleUpdate; sent: ScheduleDraft }) =>
      updateJobSchedule(info.type, body),
    onSuccess: (_data, { sent }) => onSaved(info.type, sent),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  // The one-off date has texts of its own: outside now to a year, or no wall time at all.
  const onceCode = isApiError(save.error)
    ? save.error.problem?.errors?.find((fieldError) => fieldError.field === 'onceLocal')?.code
    : undefined;
  const fieldErrors =
    errors && onceCode
      ? {
          ...errors.fields,
          onceLocal:
            onceCode === 'out_of_range'
              ? t('admin.jobs.schedule.onceRange')
              : t('admin.jobs.schedule.onceInvalid'),
        }
      : errors?.fields;
  const schedule = info.schedule;
  if (!schedule) return null;
  const set = (patch: Partial<ScheduleDraft>) => {
    if (save.error) save.reset();
    onChange(info.type, { ...value, ...patch });
  };

  const options = EVERY_OPTIONS.filter(
    ([, seconds]) =>
      seconds >= schedule.limits.minEverySeconds && seconds <= schedule.limits.maxEverySeconds,
  ).map(([key, seconds]) => ({
    value: String(seconds),
    label: t(`admin.jobs.schedule.everyOptions.${key}`),
  }));
  // The stored interval may lie outside the offered steps (set by hand elsewhere): keep it.
  if (!options.some((option) => option.value === value.every)) {
    options.unshift({ value: value.every, label: value.every });
  }

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Nothing changed: nothing to send, the focus stays on «Speichern».
    if (!dirty) return;
    const body: JobScheduleUpdate = { enabled: value.enabled, everySeconds: Number(value.every) };
    if (value.windowed) {
      const startMinute = minuteOf(value.start);
      const endMinute = minuteOf(value.end);
      if (startMinute === null || endMinute === null) return;
      body.window = { startMinute, endMinute };
    } else if (schedule.window) {
      body.clearWindow = true;
    }
    // The date only when it changed: the wall time as typed, the server converts it.
    if (value.once !== (schedule.once?.local ?? '')) {
      if (value.once === '') body.clearOnce = true;
      else body.onceLocal = value.once;
    }
    save.mutate({ body, sent: value });
  };

  return (
    <section className={styles.block} aria-label={t('admin.jobs.schedule.title', { name })}>
      <h3 className={styles.blockTitle}>{t('admin.jobs.schedule.heading')}</h3>
      <Form className={styles.fields} onSubmit={onSubmit} validationErrors={fieldErrors}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <Switch isSelected={value.enabled} onChange={(enabled) => set({ enabled })}>
          {t('admin.jobs.schedule.enabled')}
        </Switch>
        <SelectField
          label={t('admin.jobs.schedule.every')}
          value={value.every}
          options={options}
          onChange={(every) => set({ every })}
        />
        {errors?.fields.everySeconds && (
          <Text tone="secondary" variant="caption">
            {errors.fields.everySeconds}
          </Text>
        )}
        <Switch isSelected={value.windowed} onChange={(windowed) => set({ windowed })}>
          {t('admin.jobs.schedule.window')}
        </Switch>
        {value.windowed && (
          <div className={styles.windowFields}>
            <TextField
              name="windowStart"
              type="time"
              label={t('admin.jobs.schedule.windowStart')}
              value={value.start}
              onChange={(start) => set({ start })}
              isRequired
            />
            <TextField
              name="windowEnd"
              type="time"
              label={t('admin.jobs.schedule.windowEnd')}
              description={t('admin.jobs.schedule.windowHint', { zone: schedule.timeZone })}
              value={value.end}
              onChange={(end) => set({ end })}
              isRequired
            />
          </div>
        )}
        {errors?.fields.window && (
          <Text tone="secondary" variant="caption">
            {errors.fields.window}
          </Text>
        )}
        {/* Native date and time: the value is the server's wall time and is sent unchanged. */}
        <div className={styles.onceField}>
          <TextField
            name="onceLocal"
            type="datetime-local"
            label={t('admin.jobs.schedule.once')}
            description={t('admin.jobs.schedule.onceHint', { zone: schedule.timeZone })}
            value={value.once}
            onChange={(once) => set({ once })}
            inputRef={onceInput}
          />
          {value.once !== '' && (
            <AriaButton
              variant="ghost"
              icon="close"
              onPress={() => {
                set({ once: '' });
                // The button goes with the date; the focus stays in the form.
                onceInput.current?.focus();
              }}
            >
              {t('admin.jobs.schedule.clearOnce')}
            </AriaButton>
          )}
        </div>
        <Text tone="secondary" variant="caption">
          {t('admin.jobs.schedule.description')}
        </Text>
        {info.type === 'trakt.refresh' && (
          <Text tone="secondary" variant="caption">
            {t('admin.jobs.schedule.traktHint')}
          </Text>
        )}
        {info.type === 'update.check' && (
          <Text tone="secondary" variant="caption">
            {t('admin.jobs.schedule.updateHint')}
          </Text>
        )}
        <div className={styles.formActions}>
          {dirty && (
            <AriaButton
              variant="ghost"
              isDisabled={save.isPending}
              onPress={() => {
                save.reset();
                onDiscard(info.type);
                // The button goes with the draft; the focus stays in the form.
                saveButton.current?.focus();
              }}
            >
              {t('admin.jobs.schedule.discard')}
            </AriaButton>
          )}
          <AriaButton ref={saveButton} type="submit" variant="primary" isPending={save.isPending}>
            {t('admin.jobs.schedule.save')}
          </AriaButton>
        </div>
      </Form>
    </section>
  );
}

/**
 * The time window of a windowable event type inside its row (concept 15, 4.3), built like the
 * window of a schedule: the sentence why the type has no schedule, then «Nur in einem
 * Zeitfenster» with Von and Bis in the server's zone (across midnight allowed). Saving sends the
 * window, or `clearWindow` once it is switched off. What is typed is the page's draft of the type
 * (S-17). The server checks the window (rule 12); a 422 names it until a field changes.
 */
function WindowBlock({
  info,
  value,
  dirty,
  onChange,
  onDiscard,
  onSaved,
}: {
  info: JobTypeInfo;
  value: WindowDraft;
  dirty: boolean;
  onChange: (type: string, next: WindowDraft) => void;
  onDiscard: (type: string) => void;
  onSaved: (type: string, sent: WindowDraft) => Promise<void>;
}) {
  const { t } = useTranslation();
  const name = typeName(t, info.type);
  const saveButton = useRef<HTMLButtonElement>(null);
  const save = useMutation({
    mutationFn: ({ body }: { body: JobTypeWindowUpdate; sent: WindowDraft }) =>
      updateJobTypeWindow(info.type, body),
    onSuccess: (_data, { sent }) => onSaved(info.type, sent),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const set = (patch: Partial<WindowDraft>) => {
    if (save.error) save.reset();
    onChange(info.type, { ...value, ...patch });
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Nothing changed: nothing to send, the focus stays on «Speichern».
    if (!dirty) return;
    let body: JobTypeWindowUpdate = { clearWindow: true };
    if (value.windowed) {
      const startMinute = minuteOf(value.start);
      const endMinute = minuteOf(value.end);
      if (startMinute === null || endMinute === null) return;
      body = { window: { startMinute, endMinute } };
    }
    save.mutate({ body, sent: value });
  };

  return (
    <section className={styles.block} aria-label={t('admin.jobs.schedule.windowTitle', { name })}>
      <h3 className={styles.blockTitle}>{t('admin.jobs.schedule.heading')}</h3>
      <Text tone="secondary">{t('admin.jobs.schedule.eventNote')}</Text>
      <Form className={styles.fields} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <Switch isSelected={value.windowed} onChange={(windowed) => set({ windowed })}>
          {t('admin.jobs.schedule.window')}
        </Switch>
        {value.windowed && (
          <>
            <div className={styles.windowFields}>
              <TextField
                name="windowStart"
                type="time"
                label={t('admin.jobs.schedule.windowStart')}
                value={value.start}
                onChange={(start) => set({ start })}
                isRequired
              />
              <TextField
                name="windowEnd"
                type="time"
                label={t('admin.jobs.schedule.windowEnd')}
                description={t('admin.jobs.schedule.windowHint', {
                  zone: info.windowTimeZone ?? '',
                })}
                value={value.end}
                onChange={(end) => set({ end })}
                isRequired
              />
            </div>
            <Text tone="secondary" variant="caption">
              {t('admin.jobs.schedule.windowWaits')}
            </Text>
          </>
        )}
        {errors?.fields.window && (
          <Text tone="secondary" variant="caption">
            {errors.fields.window}
          </Text>
        )}
        <div className={styles.formActions}>
          {dirty && (
            <AriaButton
              variant="ghost"
              isDisabled={save.isPending}
              onPress={() => {
                save.reset();
                onDiscard(info.type);
                // The button goes with the draft; the focus stays in the form.
                saveButton.current?.focus();
              }}
            >
              {t('admin.jobs.schedule.discard')}
            </AriaButton>
          )}
          <AriaButton ref={saveButton} type="submit" variant="primary" isPending={save.isPending}>
            {t('admin.jobs.schedule.save')}
          </AriaButton>
        </div>
      </Form>
    </section>
  );
}

/** The runs of a type inside its row: one tab per state (JOB-7; instead of the runs dialog). */
function RunsBlock({
  info,
  hold,
  onSettled,
  onFailure,
}: {
  info: JobTypeInfo;
  hold: boolean;
  onSettled: (text: string) => void;
  onFailure: (error: unknown) => void;
}) {
  const { t, i18n } = useTranslation();
  const name = typeName(t, info.type);
  const [tab, setTab] = useState<JobRunState>(info.counts.running > 0 ? 'running' : 'queued');
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const countOf = (state: JobRunState): number | undefined => {
    switch (state) {
      case 'running':
      case 'queued':
      case 'planned':
      case 'failed':
        return info.counts[state];
      default:
        return undefined;
    }
  };
  return (
    <section className={styles.block} aria-label={t('admin.jobs.runs.title', { name })}>
      <h3 className={styles.blockTitle}>{t('admin.jobs.runs.heading')}</h3>
      <Tabs
        label={t('admin.jobs.runs.tabs')}
        selectedKey={tab}
        onSelectionChange={(key) => setTab(key as JobRunState)}
        tabs={RUN_STATES.map((state) => {
          const count = countOf(state);
          return {
            id: state,
            label: t(`admin.jobs.runs.tab.${state}`),
            ...(count === undefined
              ? {}
              : {
                  count: number.format(count),
                  tone: state === 'failed' && count > 0 ? ('danger' as const) : undefined,
                }),
            content: (
              <RunsTab
                type={info.type}
                state={state}
                hold={hold}
                language={i18n.language}
                onSettled={onSettled}
                onFailure={onFailure}
              />
            ),
          };
        })}
      />
    </section>
  );
}

const RunsTab = memo(function RunsTab({
  type,
  state,
  hold,
  language,
  onSettled,
  onFailure,
}: {
  type: string;
  state: JobRunState;
  /** The page holds its polling (a draft, a confirmation): so do the runs. */
  hold: boolean;
  language: string;
  onSettled: (text: string) => void;
  onFailure: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const query = useQuery({
    ...jobRunsQuery(type, state),
    refetchInterval: hold ? false : JOBS_POLL_MS,
    refetchOnWindowFocus: !hold,
  });
  const done = async (text: string) => {
    await queryClient.invalidateQueries({ queryKey: ['admin', 'job-runs', type] });
    onSettled(text);
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
          {(job.overridesPause || job.overridesWindow) && (
            <>
              {' '}
              <Chip tone="info">
                {job.overridesPause
                  ? t('admin.jobs.running.overridesPause')
                  : t('admin.jobs.running.overridesWindow')}
              </Chip>
            </>
          )}
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
      cell: (job) => <span className={styles.numbers}>{job.priority}</span>,
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
          t('admin.jobs.none')
        ),
    },
    {
      key: 'duration',
      header: t('admin.jobs.runs.columns.duration'),
      cell: (job) => (job.startedAt ? <Elapsed run={job} /> : t('admin.jobs.none')),
    },
    {
      key: 'attempts',
      header: t('admin.jobs.runs.columns.attempts'),
      cell: (job) => (
        <span className={styles.numbers}>
          {t('admin.home.jobs.attempts', { attempts: job.attempts, max: job.maxAttempts })}
        </span>
      ),
    },
    {
      key: 'error',
      header: t('admin.jobs.runs.columns.error'),
      cell: (job) => <ErrorText error={job.lastError} />,
    },
    {
      key: 'actions',
      header: t('admin.jobs.runs.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (job) => (
        <span className={styles.rowActions}>
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
});

/**
 * The tab «Fehlschläge» (JOB-8): the ten newest failures of every type from the dashboard
 * (`jobs.recentFailed`) with «Erneut» and «Abbrechen» (asks first, through the page); the tab
 * itself carries the total. All failures of one type are under «Läufe» of its row.
 */
function FailedTab({
  query,
  total,
  retrying,
  onRetry,
  onCancel,
}: {
  query: UseQueryResult<Dashboard>;
  total: number;
  retrying: (id: number) => boolean;
  onRetry: (job: JobSummary) => void;
  onCancel: (job: JobSummary) => void;
}) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const heading = <h2 className={styles.tabTitle}>{t('admin.jobs.failed.title')}</h2>;
  if (query.isPending) return <Skeleton shape="block" />;
  if (query.isError) {
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, query.error)}
        actions={
          <AriaButton icon="refresh" onPress={() => void query.refetch()}>
            {t('common.retry')}
          </AriaButton>
        }
      />
    );
  }
  const jobs = query.data.jobs.recentFailed;
  if (jobs.length === 0) {
    return (
      <div className={styles.tabBody}>
        {heading}
        <Text tone="secondary">{t('admin.jobs.failed.empty')}</Text>
      </div>
    );
  }
  const columns: DataColumn<JobSummary>[] = [
    {
      key: 'type',
      header: t('admin.jobs.failed.columns.type'),
      rowHeader: true,
      cell: (job) => typeName(t, job.type),
    },
    {
      key: 'attempts',
      header: t('admin.jobs.runs.columns.attempts'),
      cell: (job) => (
        <span className={styles.numbers}>
          {t('admin.home.jobs.attempts', { attempts: job.attempts, max: job.maxAttempts })}
        </span>
      ),
    },
    {
      key: 'error',
      header: t('admin.jobs.runs.columns.error'),
      cell: (job) => <ErrorText error={job.lastError} />,
    },
    {
      key: 'at',
      header: t('admin.jobs.failed.columns.at'),
      cell: (job) => (
        <time className={styles.nowrap} dateTime={job.updatedAt}>
          {formatDateTime(job.updatedAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'actions',
      header: t('admin.jobs.runs.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (job) => {
        const name = typeName(t, job.type);
        return (
          <span className={styles.rowActions}>
            <AriaButton
              variant="ghost"
              icon="refresh"
              aria-label={t('admin.jobs.failed.retryLabel', { id: job.id, name })}
              isPending={retrying(job.id)}
              onPress={() => onRetry(job)}
            >
              {t('admin.jobs.runs.retry')}
            </AriaButton>
            <AriaButton
              variant="ghost"
              icon="trash"
              aria-label={t('admin.jobs.running.cancelLabel', { id: job.id, name })}
              onPress={() => onCancel(job)}
            >
              {t('admin.jobs.runs.cancel')}
            </AriaButton>
          </span>
        );
      },
    },
  ];
  return (
    <div className={styles.tabBody}>
      {heading}
      <DataTable
        caption={t('admin.jobs.failed.title')}
        columns={columns}
        rows={jobs}
        rowKey={(job) => String(job.id)}
      />
      <Text tone="secondary" variant="caption">
        {t('admin.jobs.failed.shown', {
          shown: number.format(jobs.length),
          total: number.format(total),
        })}
      </Text>
    </div>
  );
}
