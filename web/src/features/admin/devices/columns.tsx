import type { AdminDevice } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatDateTime } from '@/lib/format';
import { VisuallyHidden } from '@/ui';
import { AriaButton, type DataColumn } from '@/ui/aria';
import styles from './Devices.module.css';

/** The name of a device; a browser that never told one is «Unbekanntes Gerät». */
export const deviceName = (t: TFunction, device: AdminDevice) =>
  device.name ?? t('admin.devices.unnamed');

/** A value the device did not report: a dash on screen, a word for screen readers. */
function Unknown({ t }: { t: TFunction }) {
  return (
    <>
      <span aria-hidden="true">{t('admin.devices.unknown')}</span>
      <VisuallyHidden>{t('admin.devices.unknownSpoken')}</VisuallyHidden>
    </>
  );
}

const codecList = (t: TFunction, codecs: readonly string[] | undefined) =>
  codecs && codecs.length > 0 ? (
    codecs.map((codec) => codec.toUpperCase()).join(t('format.listSeparator'))
  ) : (
    <Unknown t={t} />
  );

/** The device's name in full, wrapped where it has to (S-5: reachable without a pointer). */
const nameColumn = (t: TFunction): DataColumn<AdminDevice> => ({
  key: 'name',
  header: t('admin.devices.columns.name'),
  rowHeader: true,
  cell: (device) => <span className={styles.wrap}>{deviceName(t, device)}</span>,
});

/**
 * The tab «Geräte» (GER-1, mockup Ger-C): the owner of the last playback start, the last
 * contact, the running streams and «Vergessen», which the page confirms in its own dialog.
 */
export function deviceColumns(
  t: TFunction,
  language: string,
  number: Intl.NumberFormat,
  onForget: (device: AdminDevice) => void,
): DataColumn<AdminDevice>[] {
  return [
    nameColumn(t),
    {
      key: 'owner',
      header: t('admin.devices.columns.owner'),
      cell: (device) =>
        device.ownerEmail ? (
          <span className={styles.wrap}>
            {t('admin.devices.owner', {
              email: device.ownerEmail,
              profile: device.ownerProfile ?? t('admin.devices.unknown'),
            })}
          </span>
        ) : (
          <span className={styles.muted}>{t('admin.devices.noOwner')}</span>
        ),
    },
    {
      key: 'lastSeen',
      header: t('admin.devices.columns.lastSeen'),
      cell: (device) => (
        <time className={styles.nowrap} dateTime={device.lastSeenAt}>
          {formatDateTime(device.lastSeenAt, language)}
        </time>
      ),
    },
    {
      key: 'streams',
      header: t('admin.devices.columns.streams'),
      align: 'end',
      cell: (device) => (
        <span className={styles.numbers}>{number.format(device.activeStreams)}</span>
      ),
    },
    {
      key: 'actions',
      header: t('admin.devices.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (device) => (
        <AriaButton
          variant="danger"
          aria-label={t('admin.devices.forget.label', { name: deviceName(t, device) })}
          onPress={() => onForget(device)}
        >
          {t('admin.devices.forget.action')}
        </AriaButton>
      ),
    },
  ];
}

/**
 * The tab «Fähigkeiten» (GER-1): the capability profile of the device's last playback start,
 * as the browser reported it; what it did not report is unknown.
 */
export function capabilityColumns(t: TFunction): DataColumn<AdminDevice>[] {
  return [
    nameColumn(t),
    {
      key: 'containers',
      header: t('admin.devices.columns.containers'),
      cell: (device) => codecList(t, device.capabilities?.containers),
    },
    {
      key: 'video',
      header: t('admin.devices.columns.video'),
      cell: (device) => codecList(t, device.capabilities?.video),
    },
    {
      key: 'audio',
      header: t('admin.devices.columns.audio'),
      cell: (device) => codecList(t, device.capabilities?.audio),
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
          <Unknown t={t} />
        ),
    },
    {
      key: 'hdr',
      header: t('admin.devices.columns.hdr'),
      cell: ({ capabilities }) =>
        capabilities?.hdr === undefined ? (
          <Unknown t={t} />
        ) : capabilities.hdr ? (
          t('admin.devices.yes')
        ) : (
          t('admin.devices.no')
        ),
    },
  ];
}
