import type { Dashboard, SystemInfo } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { intlLocale } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { FactList, Notice, Skeleton, StatusDot, Text, TextLink } from '@/ui';
import { AdminPage, AriaButton, ConfirmDialog, DashCard } from '@/ui/aria';
import { DASHBOARD_POLL_MS, dashboardQuery, systemQuery } from './queries';
import { reloadPage } from './restart';
import { RestartPanel } from './RestartPanel';
import {
  ActivityCard,
  HintList,
  isInstalling,
  ServerLog,
  StorageCard,
  UpdateInstall,
  updaterFact,
  updaterOutdated,
  UpdaterOutdated,
  updateState,
  Uptime,
  useHints,
} from './server/parts';
import styles from './server/Server.module.css';

/** The facts of the server: version, uptime, video encoding and the file watcher. */
function systemFacts(
  t: TFunction,
  language: string,
  system: SystemInfo,
  now: number,
): [string, ReactNode][] {
  const number = (value: number) => new Intl.NumberFormat(intlLocale(language)).format(value);
  return [
    [t('admin.home.server.version'), system.version],
    [t('admin.home.server.uptime'), <Uptime key="uptime" now={now} system={system} />],
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
  ];
}

/**
 * The server's cards (redesign, build plan 4; mockup D067 to D069): its state, the updates with
 * the install and «Neu starten», both storage paths, the activity and the server log. `check` is
 * the header's «Nach Updates suchen»; its outcome stands on the card «Updates & Neustart».
 */
function ServerCards({
  data,
  now,
  system,
  systemError,
  check,
  restart,
  installAt,
  onInstallAt,
}: {
  data: Dashboard;
  /** The moment of the poll: the uptime counts on to it. */
  now: number;
  system: SystemInfo | undefined;
  systemError: unknown;
  check: { error: unknown; isSuccess: boolean; reset: () => void };
  restart: { error: unknown; isPending: boolean; mutate: () => void };
  installAt: number | null;
  onInstallAt: (at: number | null) => void;
}) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const hints = useHints(data, system);
  const { update } = data;
  const streams = data.streams.length;
  // While an install runs the companion restarts the server; the server refuses a restart
  // (update_running) as well.
  const installing = installAt !== null || isInstalling(update.install);
  const updater = updaterFact(t, update);
  const at = (place: string) => hints.filter((hint) => hint.place === place);

  return (
    <>
      <div className={styles.frame}>
        <div className={styles.grid}>
          <DashCard icon="server" title={t('admin.server.state')}>
            {system ? (
              <FactList facts={systemFacts(t, language, system, now)} />
            ) : systemError ? (
              <Notice tone="danger">{problemMessage(t, systemError)}</Notice>
            ) : (
              <Skeleton shape="block" />
            )}
            <HintList hints={at('system')} />
            <TextLink href={paths.adminIntegrations}>{t('admin.server.toIntegrations')}</TextLink>
          </DashCard>
          <DashCard
            icon="download"
            title={t('admin.server.updates')}
            footer={
              !installing && (
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
              )
            }
          >
            <FactList
              facts={[
                [t('admin.home.server.update'), updateState(t, update)],
                ...(updater ? [updater] : []),
              ]}
            />
            {update.checkedAt && (
              <Text tone="secondary" variant="caption">
                {t('admin.home.server.checked', {
                  date: formatDateTime(update.checkedAt, intlLocale(language)),
                })}
              </Text>
            )}
            {/* A failed check keeps the known release: when it was read stays in sight. */}
            {update.error && update.lastSuccessAt && (
              <Text tone="secondary" variant="caption">
                {t('admin.home.server.lastSuccess', {
                  date: formatDateTime(update.lastSuccessAt, intlLocale(language)),
                })}
              </Text>
            )}
            {updaterOutdated(update) && <UpdaterOutdated version={update.currentVersion} />}
            {check.error ? <Notice tone="danger">{problemMessage(t, check.error)}</Notice> : null}
            {restart.error ? (
              <Notice tone="danger">{problemMessage(t, restart.error)}</Notice>
            ) : null}
            {check.isSuccess && !check.error && (
              <Notice live={false}>{t('admin.home.server.checkedNow')}</Notice>
            )}
            <HintList hints={at('update')} />
            <UpdateInstall
              update={update}
              dashboardAt={now}
              installAt={installAt}
              onInstallAt={onInstallAt}
            />
          </DashCard>
          <StorageCard storage={data.storage} hints={at('storage')} />
          <ActivityCard activity={data.activity} now={now} />
        </div>
      </div>
      <ServerLog errors={data.errors} />
    </>
  );
}

/**
 * System › Server (redesign, build plan 4): every function of the server the overview had before,
 * in full — version, uptime, video encoding and file watcher, the updates with the install by the
 * companion or by hand, «Neu starten» with its waiting panel, both storage paths, the server log
 * (polled every 5 s, A-3) and the activity. The graphics chip, the public address and the update
 * check are set on «Integrationen», which the page links.
 */
export function ServerPage() {
  const { t } = useTranslation();
  const title = t('admin.sections.server.title');
  useDocumentTitle(title);
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  // When the server accepted the restart (ms): the waiting panel replaces the cards.
  const [restartAt, setRestartAt] = useState<number | null>(null);
  // When this page started (or joined) an update install (ms): the server goes away during it, so
  // the dashboard waits while the card follows the install itself.
  const [installAt, setInstallAt] = useState<number | null>(null);
  const dashboard = useQuery({
    ...dashboardQuery,
    enabled: allowed && restartAt === null,
    refetchInterval: installAt === null ? DASHBOARD_POLL_MS : false,
  });
  const system = useQuery({ ...systemQuery, enabled: allowed });
  const check = useMutation({
    mutationFn: () => request(api.POST('/admin/update/check')),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
  });
  const restart = useMutation({
    mutationFn: () => request(api.POST('/admin/server/restart')),
    onSuccess: () => setRestartAt(Date.now()),
    // A restart or restore is already on its way: wait for it like for our own.
    onError: (error) => {
      if (isApiError(error, 409) && error.code === 'restart_pending') setRestartAt(Date.now());
    },
  });
  const data = dashboard.data;
  const polling = allowed && restartAt === null && installAt === null;

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
      help={t('admin.server.intro')}
      liveSeconds={polling && !dashboard.isError ? DASHBOARD_POLL_MS / 1000 : undefined}
      actions={
        <>
          {polling && dashboard.isError && (
            <span className={styles.live}>
              <StatusDot state="danger" />
              {t('admin.home.liveFailed')}
            </span>
          )}
          {restartAt === null && (
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
          )}
        </>
      }
    >
      {restartAt !== null ? (
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
            <ServerCards
              data={data}
              now={dashboard.dataUpdatedAt}
              system={system.data}
              systemError={system.error}
              check={check}
              restart={restart}
              installAt={installAt}
              onInstallAt={setInstallAt}
            />
          )}
        </div>
      )}
    </AdminPage>
  );
}
