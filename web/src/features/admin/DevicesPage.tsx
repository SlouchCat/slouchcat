import type { AdminDevice } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  DisclosureGroup,
  StatusRegion,
  afterDialogsClosed,
  focusTitleIfLost,
  useSearchSelection,
  useStatusMessage,
  type TabItem,
} from '@/ui/aria';
import { capabilityColumns, deviceColumns, deviceName } from './devices/columns';
import { devicesQuery } from './queries';
import styles from './devices/Devices.module.css';

/** The tabs in the band: the list (the default, no `?tab=`) and the capability profiles. */
const DEVICES = 'devices';
const CAPABILITIES = 'capabilities';
const TAB_IDS = [DEVICES, CAPABILITIES] as const;
type View = (typeof TAB_IDS)[number];

/** A section of a tab: a type of the contract, or «Unbekannt». */
type TypeKey = NonNullable<AdminDevice['type']> | 'unknown';

/** The sections of a tab, one per device type, in the order of the mockup. */
const TYPE_ORDER = [
  'tv',
  'desktop',
  'phone',
  'tablet',
  'other',
  'unknown',
] as const satisfies readonly TypeKey[];

/**
 * The section of a device: a device without a type, or with one this build does not know (a
 * newer server), is «Unbekannt», so no device is left out.
 */
const typeOf = (device: AdminDevice): TypeKey =>
  (TYPE_ORDER as readonly string[]).includes(device.type ?? '')
    ? (device.type as TypeKey)
    : 'unknown';

/**
 * Devices (W-7; concept 12, variant C): every browser that signed in. The tab «Geräte» shows the
 * owner of its last playback start, its last contact, its streams and «Vergessen», which ends its
 * streams and sessions; the tab «Fähigkeiten» shows the capability profile it reported. In both,
 * each device type is a section that folds away (GER-1).
 */
export function DevicesPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.devices.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.devices.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <DevicesContent />;
}

function DevicesContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);

  const [forgetting, setForgetting] = useState<AdminDevice | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // No refetch while the confirmation is open: its row stays as it was asked about.
  const devices = useQuery({ ...devicesQuery, refetchOnWindowFocus: !confirmOpen });

  // An unknown `?tab=` shows «Geräte», and the address says so too (replaced, not pushed).
  const [tab, selectTab] = useSearchSelection('tab', TAB_IDS, DEVICES, { normalise: true });

  // Folded sections by type, held by the page: the same type is folded in both tabs.
  const [folded, setFolded] = useState<ReadonlySet<TypeKey>>(() => new Set());
  const onExpandedChange = useCallback(
    (type: TypeKey, open: boolean) =>
      setFolded((current) => {
        const next = new Set(current);
        if (open) next.delete(type);
        else next.add(type);
        return next;
      }),
    [],
  );

  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);

  const forget = useMutation({
    mutationFn: (device: AdminDevice) =>
      request(api.DELETE('/admin/devices/{id}', { params: { path: { id: device.id } } })),
    onSuccess: async (_data, device) => {
      setFailure(null);
      setMessage(t('admin.devices.forgotten', { name: deviceName(t, device) }));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: devicesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ['admin', 'streams'] }),
      ]);
      // The row and its button are gone: once the dialog has closed, the h1 takes the focus.
      await afterDialogsClosed();
      titleRef.current?.focus();
    },
    onError: (error) => {
      setMessage(null);
      setFailure(error);
    },
  });

  // The confirmation is the page's own: a row that goes while it is open (a refetch) cannot take
  // the dialog with it.
  const askForget = useCallback((device: AdminDevice) => {
    setForgetting(device);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const columns = useMemo(
    () => ({
      [DEVICES]: deviceColumns(t, i18n.language, number, askForget),
      [CAPABILITIES]: capabilityColumns(t),
    }),
    [t, i18n.language, number, askForget],
  );

  const list = useMemo(() => devices.data ?? [], [devices.data]);
  const groups = useMemo(
    () =>
      TYPE_ORDER.map((type) => ({
        type,
        devices: list.filter((device) => typeOf(device) === type),
      })).filter((group) => group.devices.length > 0),
    [list],
  );
  const profiled = list.filter((device) => device.capabilities !== undefined).length;

  const tabOf = (id: View, count: number, countLabel: string): TabItem => {
    const label = t(`admin.devices.tabs.${id}`);
    return {
      id,
      label,
      count: number.format(count),
      countLabel,
      content:
        list.length === 0 ? (
          <Text tone="secondary">{t('admin.devices.empty')}</Text>
        ) : (
          <div className={styles.tab}>
            <Text tone="secondary">{t(`admin.devices.hint.${id}`)}</Text>
            {groups.map((group) => {
              const name = t(`admin.devices.types.${group.type}`);
              const streams = group.devices.reduce((sum, device) => sum + device.activeStreams, 0);
              return (
                <DisclosureGroup
                  key={group.type}
                  title={name}
                  count={number.format(group.devices.length)}
                  countLabel={t('admin.devices.count', {
                    count: group.devices.length,
                    number: number.format(group.devices.length),
                  })}
                  meta={
                    streams > 0
                      ? t('admin.devices.streamCount', {
                          count: streams,
                          number: number.format(streams),
                        })
                      : undefined
                  }
                  isExpanded={!folded.has(group.type)}
                  onExpandedChange={(open) => onExpandedChange(group.type, open)}
                >
                  <DataTable
                    caption={t('admin.devices.caption', { view: label, type: name })}
                    columns={columns[id]}
                    rows={group.devices}
                    rowKey={(device) => device.id}
                    layout="fixed"
                  />
                </DisclosureGroup>
              );
            })}
          </div>
        ),
    };
  };

  return (
    <AdminPage title={t('admin.devices.title')} help={t('admin.devices.intro')} titleRef={titleRef}>
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {devices.isPending && <Skeleton shape="block" />}
      {devices.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, devices.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void devices.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {devices.data && (
        <BandPanel
          title={t('admin.devices.list')}
          hideTitle
          icon="monitor"
          tabs={{
            label: t('admin.devices.tabs.label'),
            selectedKey: tab,
            onSelectionChange: selectTab,
            tabs: [
              tabOf(
                DEVICES,
                list.length,
                t('admin.devices.count', {
                  count: list.length,
                  number: number.format(list.length),
                }),
              ),
              tabOf(
                CAPABILITIES,
                profiled,
                t('admin.devices.profiled', {
                  count: profiled,
                  number: number.format(profiled),
                }),
              ),
            ],
          }}
        />
      )}
      {forgetting && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={t('admin.devices.forget.title', { name: deviceName(t, forgetting) })}
          body={t('admin.devices.forget.body')}
          confirmLabel={t('admin.devices.forget.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => forget.mutate(forgetting)}
        />
      )}
    </AdminPage>
  );
}
