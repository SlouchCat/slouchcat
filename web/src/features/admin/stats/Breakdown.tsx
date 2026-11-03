import type { StatDevice, StatProfile } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { formatDuration } from '@/lib/format';
import { Text } from '@/ui';
import { BarChart, DataTable, type DataColumn } from '@/ui/aria';
import styles from './Stats.module.css';

const deviceType = (t: TFunction, type: StatDevice['type']) =>
  type === 'unknown' ? t('admin.stats.devices.unknown') : t(`admin.devices.types.${type}`);

/** «Schauzeit je Profil» (STA-2): a chart of the watch time and the table with plays and time. */
export function WatchTime({ profiles }: { profiles: StatProfile[] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  if (profiles.length === 0) {
    return <Text tone="secondary">{t('admin.stats.profiles.empty')}</Text>;
  }
  const columns: DataColumn<StatProfile>[] = [
    {
      key: 'profile',
      header: t('admin.stats.profiles.columns.profile'),
      rowHeader: true,
      cell: (profile) => (
        <span className={styles.stack}>
          <span>{profile.profileName}</span>
          <span className={styles.muted}>{profile.accountEmail}</span>
        </span>
      ),
    },
    {
      key: 'plays',
      header: t('admin.stats.columns.plays'),
      align: 'end',
      cell: (profile) => <span className={styles.numbers}>{number.format(profile.plays)}</span>,
    },
    {
      key: 'watched',
      header: t('admin.stats.columns.watched'),
      align: 'end',
      cell: (profile) => (
        <span className={styles.numbers}>{formatDuration(t, profile.watchedMs)}</span>
      ),
    },
  ];
  return (
    <div className={styles.breakdown}>
      <BarChart
        title={t('admin.stats.profiles.chart')}
        columns={[t('admin.stats.profiles.columns.profile'), t('admin.stats.columns.watched')]}
        bars={profiles.map((profile) => {
          const label = t('admin.stats.profiles.label', {
            profile: profile.profileName,
            email: profile.accountEmail,
          });
          const valueText = formatDuration(t, profile.watchedMs);
          return {
            key: String(profile.profileId),
            label,
            tick: profile.profileName,
            value: profile.watchedMs,
            valueText,
            tooltip: t('admin.stats.tooltip', { label, value: valueText }),
          };
        })}
      />
      <DataTable
        caption={t('admin.stats.profiles.title')}
        columns={columns}
        rows={profiles}
        rowKey={(profile) => String(profile.profileId)}
      />
    </div>
  );
}

/** «Geräte» (STA-2): the watch time per device type as a chart and the table with the counts. */
export function Devices({ devices }: { devices: StatDevice[] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  if (devices.length === 0) {
    return <Text tone="secondary">{t('admin.stats.devices.empty')}</Text>;
  }
  const columns: DataColumn<StatDevice>[] = [
    {
      key: 'type',
      header: t('admin.stats.devices.columns.type'),
      rowHeader: true,
      cell: (device) => deviceType(t, device.type),
    },
    {
      key: 'count',
      header: t('admin.stats.devices.columns.count'),
      align: 'end',
      cell: (device) => <span className={styles.numbers}>{number.format(device.count)}</span>,
    },
    {
      key: 'watched',
      header: t('admin.stats.columns.watched'),
      align: 'end',
      cell: (device) => (
        <span className={styles.numbers}>{formatDuration(t, device.watchedMs)}</span>
      ),
    },
  ];
  return (
    <div className={styles.breakdown}>
      <BarChart
        title={t('admin.stats.devices.chart')}
        columns={[t('admin.stats.devices.columns.type'), t('admin.stats.columns.watched')]}
        bars={devices.map((device) => {
          const label = deviceType(t, device.type);
          const valueText = formatDuration(t, device.watchedMs);
          return {
            key: device.type,
            label,
            tick: label,
            value: device.watchedMs,
            valueText,
            tooltip: t('admin.stats.tooltip', { label, value: valueText }),
          };
        })}
      />
      <DataTable
        caption={t('admin.stats.devices.title')}
        columns={columns}
        rows={devices}
        rowKey={(device) => device.type}
      />
    </div>
  );
}
