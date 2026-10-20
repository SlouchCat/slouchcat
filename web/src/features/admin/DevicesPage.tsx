import type { AdminDevice } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, DataTable, type DataColumn } from '@/ui/aria';
import { devicesQuery } from './queries';
import styles from './AdminTablePage.module.css';

const codecList = (codecs: readonly string[] | undefined) =>
  codecs && codecs.length > 0 ? codecs.map((codec) => codec.toUpperCase()).join(', ') : undefined;

/**
 * Devices (W-7): every browser that signed in, with the owner of its last playback start, its
 * last contact and the capability profile it reported. "Vergessen" ends its streams and sessions.
 */
export function DevicesPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.devices.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const devices = useQuery({ ...devicesQuery, enabled: allowed });
  const [forgotten, setForgotten] = useState<string | null>(null);
  const nameOf = (device: AdminDevice) => device.name ?? t('admin.devices.unnamed');
  // The row of a forgotten device goes away with its button: the focus moves to the page heading.
  const header = useRef<HTMLDivElement>(null);
  const forget = useMutation({
    mutationFn: (device: AdminDevice) =>
      request(api.DELETE('/admin/devices/{id}', { params: { path: { id: device.id } } })),
    onSuccess: async (_data, device) => {
      setForgotten(nameOf(device));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: devicesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ['admin', 'streams'] }),
      ]);
      header.current?.focus();
    },
  });

  const unknown = t('admin.devices.unknown');
  const columns: DataColumn<AdminDevice>[] = [
    { key: 'name', header: t('admin.devices.columns.name'), rowHeader: true, cell: nameOf },
    {
      key: 'type',
      header: t('admin.devices.columns.type'),
      cell: (device) => (device.type ? t(`admin.devices.types.${device.type}`) : unknown),
    },
    {
      key: 'owner',
      header: t('admin.devices.columns.owner'),
      cell: (device) =>
        device.ownerEmail ? (
          t('admin.devices.owner', {
            email: device.ownerEmail,
            profile: device.ownerProfile ?? unknown,
          })
        ) : (
          <span className={styles.muted}>{t('admin.devices.noOwner')}</span>
        ),
    },
    {
      key: 'lastSeen',
      header: t('admin.devices.columns.lastSeen'),
      cell: (device) => (
        <time className={styles.nowrap} dateTime={device.lastSeenAt}>
          {formatDateTime(device.lastSeenAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'containers',
      header: t('admin.devices.columns.containers'),
      cell: (device) => codecList(device.capabilities?.containers) ?? unknown,
    },
    {
      key: 'video',
      header: t('admin.devices.columns.video'),
      cell: (device) => codecList(device.capabilities?.video) ?? unknown,
    },
    {
      key: 'audio',
      header: t('admin.devices.columns.audio'),
      cell: (device) => codecList(device.capabilities?.audio) ?? unknown,
    },
    {
      key: 'resolution',
      header: t('admin.devices.columns.resolution'),
      cell: ({ capabilities }) =>
        capabilities?.maxWidth && capabilities.maxHeight ? (
          <span className={styles.numbers}>
            {t('format.resolution', {
              width: capabilities.maxWidth,
              height: capabilities.maxHeight,
            })}
          </span>
        ) : (
          unknown
        ),
    },
    {
      key: 'hdr',
      header: t('admin.devices.columns.hdr'),
      cell: ({ capabilities }) =>
        capabilities?.hdr === undefined
          ? unknown
          : capabilities.hdr
            ? t('admin.devices.yes')
            : t('admin.devices.no'),
    },
    {
      key: 'streams',
      header: t('admin.devices.columns.streams'),
      align: 'end',
      cell: (device) => <span className={styles.numbers}>{device.activeStreams}</span>,
    },
    {
      key: 'actions',
      header: t('admin.devices.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (device) => (
        <ConfirmDialog
          trigger={
            <AriaButton
              variant="ghost"
              aria-label={t('admin.devices.forget.label', { name: nameOf(device) })}
            >
              {t('admin.devices.forget.action')}
            </AriaButton>
          }
          title={t('admin.devices.forget.title', { name: nameOf(device) })}
          body={t('admin.devices.forget.body')}
          confirmLabel={t('admin.devices.forget.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => forget.mutate(device)}
        />
      ),
    },
  ];

  return (
    <Page>
      <div ref={header} tabIndex={-1} className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.devices.title')}
        </Text>
        <Text tone="secondary">{t('admin.devices.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={devices.isPending}>
          {forget.error && <Notice tone="danger">{problemMessage(t, forget.error)}</Notice>}
          {forgotten && !forget.error && (
            <Notice>{t('admin.devices.forgotten', { name: forgotten })}</Notice>
          )}
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
          {devices.data?.length === 0 && <Notice>{t('admin.devices.empty')}</Notice>}
          {devices.data && devices.data.length > 0 && (
            <DataTable
              caption={t('admin.devices.list')}
              columns={columns}
              rows={devices.data}
              rowKey={(device) => device.id}
            />
          )}
        </div>
      )}
    </Page>
  );
}
