import type { Dashboard, StorageInfo, SystemInfo } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useRef, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, FactList, Notice, Skeleton, StatusDot, Text, TextLink } from '@/ui';
import { AdminPage, AriaButton, DashCard, LinkCard, StatTile, StreamRow, TaskRow } from '@/ui/aria';
import { firstOpenStep, setupPath, setupProgressQuery, stepState } from '../setup/setupState';
import { serviceHref } from './integrations/services';
import { formatSeconds, typeName } from './jobsFormat';
import { DASHBOARD_POLL_MS, dashboardQuery, systemQuery } from './queries';
import { adminAreaIcons, adminAreas, firstSectionOf, sectionsOf } from './sections';
import {
  ActivityCard,
  HintList,
  jobsFailed,
  percentOf,
  ROWS,
  streamProgress,
  updateState,
  Uptime,
  useHints,
} from './server/parts';
import { streamMode } from './streams/format';
import styles from './AdminHomePage.module.css';

// The sentence of an activity entry lives with the other shared parts; the tests read it here.
export { activitySentence } from './server/parts';

/** The share of a path that is used, in percent. */
const usedPercent = (info: StorageInfo) =>
  percentOf(Math.max(0, info.totalBytes - info.freeBytes), info.totalBytes);

/**
 * The four key figures (build plan 3), each a link to its page: running streams, running jobs
 * with the waiting and paused ones, titles to maintain, and storage with both paths (F-4).
 */
function Metrics({ data }: { data: Dashboard }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(intlLocale(i18n.language));
  const { jobs, storage } = data;
  const paused = jobs.paused ?? 0;
  const config = usedPercent(storage.config);
  const cache = usedPercent(storage.cache);
  return (
    <section aria-label={t('admin.home.metrics.label')} className={styles.metrics}>
      <StatTile
        href={paths.adminStreams}
        label={t('admin.home.metrics.streams')}
        value={number.format(data.streams.length)}
        note={t('admin.home.metrics.streamsNote')}
      />
      <StatTile
        href={paths.adminJobs}
        label={t('admin.home.metrics.jobs')}
        value={number.format(jobs.running)}
        tone={jobs.running > 0 ? 'success' : 'neutral'}
        note={
          paused > 0
            ? t('admin.home.metrics.jobsNotePaused', {
                queued: number.format(jobs.queued),
                paused: number.format(paused),
              })
            : t('admin.home.metrics.jobsNote', { queued: number.format(jobs.queued) })
        }
      />
      <StatTile
        href={paths.adminMaintenance}
        label={t('admin.home.metrics.maintenance')}
        value={number.format(data.counts.maintenanceOpen)}
        tone={data.counts.maintenanceOpen > 0 ? 'warning' : 'neutral'}
        note={t('admin.home.metrics.maintenanceNote')}
      />
      <StatTile
        href={paths.adminServer}
        label={t('admin.home.metrics.storage')}
        value={t('admin.home.metrics.storageValue', {
          config: number.format(config),
          cache: number.format(cache),
        })}
        tone={Math.max(config, cache) > 90 ? 'warning' : 'neutral'}
        note={t('admin.home.metrics.storageNote')}
      />
    </section>
  );
}

/**
 * «Gerade aktiv»: the running jobs with their progress, the running playbacks (at most three
 * each, G-4) and, while no job runs, the next planned one.
 */
function ActiveCard({ data }: { data: Dashboard }) {
  const { t, i18n } = useTranslation();
  const { jobs, streams } = data;
  return (
    <DashCard
      icon="activity"
      title={t('admin.home.active.title')}
      footer={
        <>
          <TextLink href={paths.adminJobs}>{t('admin.home.jobs.all')}</TextLink>
          {jobs.failed > 0 && (
            <TextLink href={jobsFailed}>{t('admin.home.jobs.toFailed')}</TextLink>
          )}
          <TextLink href={paths.adminStreams}>{t('admin.home.streams.all')}</TextLink>
        </>
      }
    >
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
                date: formatDateTime(jobs.nextRun.at, intlLocale(i18n.language)),
              })
            : t('admin.home.jobs.idle')}
        </Text>
      )}
      {streams.length === 0 ? (
        <Text tone="secondary" variant="caption">
          {t('admin.home.streams.empty')}
        </Text>
      ) : (
        <ul className={styles.rows} aria-label={t('admin.home.streams.title')}>
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

/** «Direkt zu deinen Aufgaben»: one link per area to its first page, its pages named below. */
function QuickLinks() {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="admin-quick" className={styles.section}>
      <h2 id="admin-quick" className={styles.sectionTitle}>
        {t('admin.home.quick.title')}
      </h2>
      <ul className={styles.quick}>
        {adminAreas.map((area) => (
          <li key={area}>
            <LinkCard
              href={firstSectionOf(area).href}
              title={t(`admin.nav.areas.${area}`)}
              description={sectionsOf(area)
                .map((section) => t(`admin.sections.${section.key}.title`))
                .join(t('format.listSeparator'))}
              icon={adminAreaIcons[area]}
            />
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * «Server & letzte Aktivitäten»: the server in short with the way to its page, the integrations
 * set up, the webhooks of the last 24 hours, the number of errors in the log, and the newest
 * activity. Everything in full stands on System › Server (build plan 4).
 */
function ServerSummary({
  data,
  now,
  system,
}: {
  data: Dashboard;
  now: number;
  system: SystemInfo | undefined;
}) {
  const { t } = useTranslation();
  const configured = data.integrations.filter((entry) => entry.state === 'configured').length;
  const { webhooks, errors } = data;
  return (
    <section aria-labelledby="admin-summary" className={styles.section}>
      <div className={styles.sectionHead}>
        <h2 id="admin-summary" className={styles.sectionTitle}>
          {t('admin.home.summary.title')}
        </h2>
        <TextLink href={paths.adminServer}>{t('admin.home.summary.details')}</TextLink>
      </div>
      <div className={styles.panels}>
        <DashCard
          icon="server"
          title={t('admin.server.state')}
          footer={
            <>
              <TextLink href={paths.adminServer}>{t('admin.home.summary.log')}</TextLink>
              <TextLink href={paths.adminIntegrations}>
                {t('admin.home.integrations.manage')}
              </TextLink>
              <TextLink href={serviceHref('webhooks')}>{t('admin.home.webhooks.manage')}</TextLink>
            </>
          }
        >
          <FactList
            facts={[
              ...(system
                ? ([
                    [t('admin.home.server.version'), system.version],
                    [
                      t('admin.home.server.uptime'),
                      <Uptime key="uptime" now={now} system={system} />,
                    ],
                  ] satisfies [string, ReactNode][])
                : []),
              [t('admin.home.server.update'), updateState(t, data.update)],
              [
                t('admin.home.integrations.title'),
                t('admin.home.integrations.count', {
                  count: configured,
                  total: data.integrations.length,
                }),
              ],
              [
                t('admin.home.webhooks.title'),
                webhooks.total === 0
                  ? t('admin.home.webhooks.none')
                  : t('admin.home.webhooks.count', {
                      count: webhooks.eventsLast24h,
                      total: webhooks.total,
                    }),
              ],
              [
                t('admin.home.errors.title'),
                errors.length === 0
                  ? t('admin.home.errors.none')
                  : t('admin.home.errors.count', { count: errors.length }),
              ],
            ]}
          />
        </DashCard>
        <ActivityCard
          activity={data.activity}
          now={now}
          shown={ROWS}
          footer={<TextLink href={paths.adminServer}>{t('admin.home.summary.details')}</TextLink>}
        />
      </div>
    </section>
  );
}

/** The overview's content for a dashboard, with the moment of the poll for the times. */
function Overview({
  data,
  now,
  system,
}: {
  data: Dashboard;
  now: number;
  system: SystemInfo | undefined;
}) {
  const { t, i18n } = useTranslation();
  const hints = useHints(data, system);
  const attention = useRef<HTMLDivElement>(null);
  const title = t('admin.home.attention.title');
  return (
    <>
      {hints.length > 0 && (
        <Notice
          tone="warning"
          live={false}
          title={t('admin.home.attention.band', { count: hints.length })}
          actions={
            <AriaButton
              variant="secondary"
              icon="chevronRight"
              onPress={() => {
                attention.current?.scrollIntoView({ block: 'start' });
                attention.current?.focus();
              }}
            >
              {t('admin.home.attention.show')}
            </AriaButton>
          }
        />
      )}
      <Metrics data={data} />
      <div className={styles.panels}>
        <div
          ref={attention}
          tabIndex={-1}
          role="group"
          aria-label={title}
          className={styles.target}
        >
          <DashCard
            icon="warning"
            title={title}
            count={new Intl.NumberFormat(intlLocale(i18n.language)).format(hints.length)}
            countLabel={t('admin.home.hints.count', { count: hints.length })}
            countLive
          >
            {hints.length === 0 ? (
              <Text tone="secondary" variant="caption">
                {t('admin.home.attention.none')}
              </Text>
            ) : (
              <HintList hints={hints} />
            )}
          </DashCard>
        </div>
        <ActiveCard data={data} />
      </div>
      <QuickLinks />
      <ServerSummary data={data} now={now} system={system} />
    </>
  );
}

/**
 * «Einrichtung fortsetzen» (concept 22, 3.4): while the owner's wizard is open, after «Später
 * fortsetzen»; without a library it says why the start page is empty.
 */
function SetupBand() {
  const { t } = useTranslation();
  const session = useActiveSession();
  // The wizard's state is the owner's primary profile's (x-visibility owner).
  const progress = useQuery({
    ...setupProgressQuery,
    enabled: session.account.isOwner && session.profile.isPrimary,
  }).data;
  if (!progress || progress.finished) return null;
  return (
    <Notice
      tone="warning"
      live={false}
      title={t('admin.home.setupBand.title')}
      actions={
        <ButtonLink href={setupPath(firstOpenStep(progress))} variant="secondary">
          {t('admin.home.setupBand.action')}
        </ButtonLink>
      }
    >
      {stepState(progress, 'libraries') === 'done'
        ? t('admin.home.setupBand.body')
        : t('admin.home.setupBand.noLibrary')}
    </Notice>
  );
}

/**
 * The administration's overview (concept 11, 11.1; A-3; redesign, build plan 3), asked again
 * every 5 s: a band with the number of points that need a look, four key figures that link their
 * pages, «Was benötigt Aufmerksamkeit?» with every hint, «Gerade aktiv», links to the areas and
 * the server in short with the activity. The server's functions in full stand on System › Server.
 */
export function AdminHomePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.home.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const dashboard = useQuery({ ...dashboardQuery, enabled: allowed });
  const system = useQuery({ ...systemQuery, enabled: allowed });
  const data = dashboard.data;
  const title = t('admin.home.heading');

  if (!allowed) {
    return (
      <AdminPage title={title}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }

  return (
    <AdminPage
      title={title}
      help={t('admin.home.intro')}
      liveSeconds={dashboard.isError ? undefined : DASHBOARD_POLL_MS / 1000}
      actions={
        <>
          {dashboard.isError && (
            <span className={styles.live}>
              <StatusDot state="danger" />
              {t('admin.home.liveFailed')}
            </span>
          )}
          <AriaButton variant="secondary" icon="refresh" onPress={() => void dashboard.refetch()}>
            {t('admin.home.refresh')}
          </AriaButton>
        </>
      }
    >
      {session.setupPending && <SetupBand />}
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
        {data && <Overview data={data} now={dashboard.dataUpdatedAt} system={system.data} />}
      </div>
    </AdminPage>
  );
}
