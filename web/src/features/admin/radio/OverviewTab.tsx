import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useMemo, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatBitrate, formatBytes, formatDateTime } from '@/lib/format';
import { FactList, Notice, Skeleton, StatusDot, Text, TextLink } from '@/ui';
import {
  AriaButton,
  BandPanel,
  ConfirmDialog,
  CoverageBars,
  StatusRegion,
  StatusStrip,
  afterDialogsClosed,
  focusTitleIfLost,
  useStatusMessage,
} from '@/ui/aria';
import { formatSeconds } from '../jobsFormat';
import {
  radioAdminKeys,
  stopRadioListener,
  syncRadioCatalog,
  type RadioCatalogState,
  type RadioRelays,
  type RadioTagRules,
} from '../queries';
import { coverageRows } from './TagRulesTab';
import styles from './Radio.module.css';

type Relay = RadioRelays['relays'][number];
type Listener = Relay['listeners'][number];

interface OverviewTabProps {
  catalog: UseQueryResult<RadioCatalogState>;
  relays: UseQueryResult<RadioRelays>;
  tagRules: UseQueryResult<RadioTagRules>;
  /** Opens the tab «Einordnung». */
  onOpenRules: () => void;
  titleRef: RefObject<HTMLHeadingElement | null>;
  /** Told when the stop confirmation opens and closes (the page holds its poll meanwhile). */
  onConfirmOpenChange: (open: boolean) => void;
}

const isoOf = (ms: number) => new Date(ms).toISOString();

const relayTone = {
  connecting: 'pending',
  live: 'success',
  reconnecting: 'warning',
  ending: 'pending',
} as const;

/**
 * «Übersicht» (concept 16, 11): the catalog and its sync, the coverage per facet with the tags
 * that wait for a rule, the running relays with their listeners (each one can be stopped) and the
 * server load.
 */
export function OverviewTab({
  catalog,
  relays,
  tagRules,
  onOpenRules,
  titleRef,
  onConfirmOpenChange,
}: OverviewTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();

  const sync = useMutation({
    mutationFn: syncRadioCatalog,
    onSuccess: async () => {
      setMessage(t('admin.radio.catalog.syncQueued'));
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    },
  });

  const [stopping, setStopping] = useState<{ listener: Listener; relay: Relay } | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const stop = useMutation({
    mutationFn: ({ listener }: { listener: Listener; relay: Relay }) =>
      stopRadioListener(listener.listenId),
    onSuccess: async (_data, { listener }) => {
      setMessage(t('admin.radio.relays.stopped', { profile: listener.profileName }));
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
  });
  const setOpen = (open: boolean) => {
    setConfirmOpen(open);
    onConfirmOpenChange(open);
    if (!open) void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  };

  const state = catalog.data;
  const load = relays.data?.load;

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      {(sync.error ?? stop.error) && (
        <Notice tone="danger">{problemMessage(t, sync.error ?? stop.error)}</Notice>
      )}

      <BandPanel
        title={t('admin.radio.catalog.title')}
        icon="radio"
        meta={
          <AriaButton icon="refresh" isPending={sync.isPending} onPress={() => sync.mutate()}>
            {t('admin.radio.catalog.sync')}
          </AriaButton>
        }
      >
        {catalog.isPending ? (
          <Skeleton shape="block" />
        ) : catalog.isError ? (
          <Notice tone="danger" title={problemMessage(t, catalog.error)} />
        ) : state ? (
          <>
            <FactList
              facts={[
                [t('admin.radio.catalog.source'), t('admin.radio.catalog.sourceValue')],
                [
                  t('admin.radio.catalog.lastSync'),
                  state.lastSyncAt === undefined
                    ? t('admin.radio.catalog.never')
                    : t('admin.radio.catalog.lastSyncValue', {
                        date: formatDateTime(isoOf(state.lastSyncAt), language),
                        entries: number.format(state.entriesTotal),
                        reachable: number.format(state.entriesReachable),
                        added: number.format(state.lastAdded),
                        removed: number.format(state.lastRemoved),
                      }),
                ],
                [t('admin.radio.catalog.stations'), number.format(state.stationsTotal)],
                [
                  t('admin.radio.catalog.nextRun'),
                  state.nextRunAt === undefined
                    ? t('admin.radio.catalog.notPlanned')
                    : formatDateTime(isoOf(state.nextRunAt), language),
                ],
              ]}
            />
            {state.lastSyncError && (
              <Notice tone="warning" title={t('admin.radio.catalog.error')}>
                <p>{state.lastSyncError}</p>
                {state.syncFailures > 0 && (
                  <p>{t('admin.radio.catalog.failures', { count: state.syncFailures })}</p>
                )}
              </Notice>
            )}
            {state.runningJobId !== undefined && (
              <Text tone="secondary">{t('admin.radio.catalog.running')}</Text>
            )}
            <TextLink href={paths.adminJobs}>{t('admin.radio.catalog.jobs')}</TextLink>
          </>
        ) : null}
      </BandPanel>

      <BandPanel
        title={t('admin.radio.rules.title')}
        icon="list"
        meta={<AriaButton onPress={onOpenRules}>{t('admin.radio.rules.open')}</AriaButton>}
      >
        {tagRules.isPending ? (
          <Skeleton shape="block" />
        ) : tagRules.isError ? (
          <Notice tone="danger" title={problemMessage(t, tagRules.error)} />
        ) : tagRules.data ? (
          <>
            <CoverageBars
              label={t('admin.radio.rules.coverage')}
              rows={coverageRows(t, language, tagRules.data.coverage)}
            />
            <p className={styles.muted} data-testid="radio-open-tags">
              {tagRules.data.worklist.length > 0
                ? t('admin.radio.rules.unassigned', { count: tagRules.data.worklist.length })
                : t('admin.radio.rules.worklistEmpty')}
            </p>
          </>
        ) : null}
      </BandPanel>

      <BandPanel title={t('admin.radio.relays.title')} icon="activity">
        {relays.isPending ? (
          <Skeleton shape="block" />
        ) : relays.isError ? (
          <Notice tone="danger" title={problemMessage(t, relays.error)} />
        ) : (relays.data?.relays.length ?? 0) === 0 ? (
          <Text tone="secondary">{t('admin.radio.relays.empty')}</Text>
        ) : (
          <ul className={styles.relays} data-testid="radio-relays">
            {relays.data?.relays.map((relay) => (
              <li key={relay.stationId} className={styles.relay}>
                <div className={styles.relayHead}>
                  <StatusDot state={relayTone[relay.state]} />
                  <span className={styles.relayName}>{relay.name}</span>
                  <span className={styles.muted}>
                    {t(`admin.radio.relays.state.${relay.state}`)}
                  </span>
                </div>
                <p className={styles.muted}>
                  {[
                    relay.codec,
                    relay.bitrateKbps !== undefined &&
                      formatBitrate(relay.bitrateKbps * 1000, language),
                    relay.transcoded
                      ? t('admin.radio.relays.transcoded')
                      : t('admin.radio.relays.copied'),
                    t('admin.radio.relays.buffer', {
                      time: formatSeconds(t, relay.bufferSeconds),
                    }),
                    formatBytes(relay.bufferBytes, language),
                    t('admin.radio.relays.since', {
                      time: formatDateTime(isoOf(relay.startedAt), language),
                    }),
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                </p>
                {relay.listeners.length === 0 ? (
                  <Text tone="secondary">{t('admin.radio.relays.noListeners')}</Text>
                ) : (
                  <ul className={styles.listeners} aria-label={t('admin.radio.relays.listeners')}>
                    {relay.listeners.map((listener) => (
                      <li key={listener.listenId} className={styles.listener}>
                        <span>
                          <span className={styles.relayName}>{listener.profileName}</span>{' '}
                          <span className={styles.muted}>
                            {listener.accountEmail} ·{' '}
                            {listener.playing
                              ? t('admin.radio.relays.playing')
                              : t('admin.radio.relays.paused')}
                          </span>
                        </span>
                        <AriaButton
                          variant="dangerQuiet"
                          size="sm"
                          aria-label={t('admin.radio.relays.stop.label', {
                            profile: listener.profileName,
                          })}
                          onPress={() => {
                            setStopping({ listener, relay });
                            setOpen(true);
                          }}
                        >
                          {t('admin.radio.relays.stop.action')}
                        </AriaButton>
                      </li>
                    ))}
                  </ul>
                )}
              </li>
            ))}
          </ul>
        )}
      </BandPanel>

      {load && (
        <BandPanel title={t('admin.radio.load.title')} icon="server">
          <StatusStrip
            label={t('admin.radio.load.title')}
            tiles={[
              {
                key: 'relays',
                label: t('admin.radio.load.relays'),
                value: t('admin.radio.load.relaysValue', {
                  count: load.relays,
                  max: number.format(load.maxRelays),
                }),
              },
              {
                key: 'upload',
                label: t('admin.radio.load.upload'),
                value: formatBitrate(load.uploadKbps * 1000, language),
              },
              {
                key: 'buffer',
                label: t('admin.radio.load.buffer'),
                value: t('admin.radio.load.bufferValue', {
                  used: formatBytes(load.bufferBytes, language),
                  budget: formatBytes(load.budgetBytes, language),
                }),
                tone:
                  load.budgetBytes > 0 && load.bufferBytes > load.budgetBytes * 0.9
                    ? 'warning'
                    : 'neutral',
              },
            ]}
          />
        </BandPanel>
      )}

      {stopping && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={setOpen}
          title={t('admin.radio.relays.stop.title', { profile: stopping.listener.profileName })}
          body={t('admin.radio.relays.stop.body', { station: stopping.relay.name })}
          confirmLabel={t('admin.radio.relays.stop.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => stop.mutate(stopping)}
        />
      )}
    </div>
  );
}
