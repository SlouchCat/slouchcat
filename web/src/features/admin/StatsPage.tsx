import type { StatDevice, StatProfile, Stats, StatsDays, StatTitle } from '@slouchcat/api-client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatDuration } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text, TextLink } from '@/ui';
import { AriaButton, BarChart, DataTable, RadioGroup, type DataColumn } from '@/ui/aria';
import { statsQuery } from './queries';
import { DEFAULT_STATS_DAYS, STATS_DAYS } from './statsDays';
import data from './AdminData.module.css';
import styles from './AdminTablePage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/stats');

const deviceType = (t: TFunction, type: StatDevice['type']) =>
  type === 'unknown' ? t('admin.stats.devices.unknown') : t(`admin.devices.types.${type}`);

/** A block of the page: a heading and its content. */
function Block({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section className={data.block} aria-labelledby={id}>
      <h2 id={id} className={data.blockTitle}>
        {title}
      </h2>
      {children}
    </section>
  );
}

function TopTitles({ titles }: { titles: StatTitle[] }) {
  const { t } = useTranslation();
  const columns: DataColumn<StatTitle>[] = [
    {
      key: 'title',
      header: t('admin.stats.top.columns.title'),
      rowHeader: true,
      cell: (title) => (
        <TextLink href={paths.adminItem(title.itemId)}>
          {title.year
            ? t('admin.titleYear', { title: title.title, year: title.year })
            : title.title}
        </TextLink>
      ),
    },
    {
      key: 'kind',
      header: t('admin.stats.top.columns.kind'),
      cell: (title) => t(`kind.${title.kind}`),
    },
    {
      key: 'plays',
      header: t('admin.stats.columns.plays'),
      align: 'end',
      cell: (title) => <span className={styles.numbers}>{title.plays}</span>,
    },
    {
      key: 'watched',
      header: t('admin.stats.columns.watched'),
      align: 'end',
      cell: (title) => <span className={styles.numbers}>{formatDuration(t, title.watchedMs)}</span>,
    },
  ];
  return titles.length === 0 ? (
    <Notice>{t('admin.stats.top.empty')}</Notice>
  ) : (
    <DataTable
      caption={t('admin.stats.top.title')}
      columns={columns}
      rows={titles}
      rowKey={(title) => title.itemId}
    />
  );
}

function WatchTime({ profiles }: { profiles: StatProfile[] }) {
  const { t } = useTranslation();
  if (profiles.length === 0) return <Notice>{t('admin.stats.profiles.empty')}</Notice>;
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
      cell: (profile) => <span className={styles.numbers}>{profile.plays}</span>,
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
    <>
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
    </>
  );
}

/** Direct play, remux and transcode as numbers with their shares, and the GPU share. */
function Playback({ playback }: { playback: Stats['playback'] }) {
  const { t, i18n } = useTranslation();
  const percent = new Intl.NumberFormat(i18n.language, {
    style: 'percent',
    maximumFractionDigits: 0,
  });
  const total = playback.direct + playback.remux + playback.transcode;
  const share = (count: number) => percent.format(total === 0 ? 0 : count / total);
  const tiles = [
    { key: 'direct', count: playback.direct, note: share(playback.direct) },
    { key: 'remux', count: playback.remux, note: share(playback.remux) },
    { key: 'transcode', count: playback.transcode, note: percent.format(playback.transcodeShare) },
  ] as const;
  return (
    <dl className={data.tiles}>
      {tiles.map((tile) => (
        <div key={tile.key} className={data.tile}>
          <dt className={data.tileLabel}>{t(`admin.stats.playback.${tile.key}`)}</dt>
          <dd className={data.tileValue}>
            {t('admin.stats.playback.sessions', { count: tile.count })}
          </dd>
          <dd className={data.tileNote}>{t('admin.stats.playback.share', { share: tile.note })}</dd>
        </div>
      ))}
      <div className={data.tile}>
        <dt className={data.tileLabel}>{t('admin.stats.playback.hardware')}</dt>
        <dd className={data.tileValue}>{percent.format(playback.hardwareShare)}</dd>
        <dd className={data.tileNote}>{t('admin.stats.playback.hardwareNote')}</dd>
      </div>
    </dl>
  );
}

function Devices({ devices }: { devices: StatDevice[] }) {
  const { t } = useTranslation();
  if (devices.length === 0) return <Notice>{t('admin.stats.devices.empty')}</Notice>;
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
      cell: (device) => <span className={styles.numbers}>{device.count}</span>,
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
    <>
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
    </>
  );
}

/**
 * Statistics (A-8) over 7, 30, 90 or 365 days and every profile: the most watched titles, watch
 * time per profile, how playback ran (direct, remux, transcode, on the GPU) and the devices by
 * type. The server counts; the page shows the numbers.
 */
export function StatsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.stats.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const { days = DEFAULT_STATS_DAYS } = route.useSearch();
  const navigate = route.useNavigate();
  // The previous period stays on screen while the next one loads.
  const stats = useQuery({
    ...statsQuery(days),
    enabled: allowed,
    placeholderData: keepPreviousData,
  });

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.stats.title')}
        </Text>
        <Text tone="secondary">{t('admin.stats.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body}>
          <RadioGroup
            name="days"
            label={t('admin.stats.period')}
            value={String(days)}
            onChange={(value) =>
              void navigate({ search: { days: Number(value) as StatsDays }, replace: true })
            }
            options={STATS_DAYS.map((option) => ({
              value: String(option),
              label: t('admin.stats.days', { count: option }),
            }))}
          />
          <div className={data.blocks} aria-busy={stats.isFetching}>
            {stats.isPending && <Skeleton shape="block" />}
            {stats.isError && (
              <Notice
                tone="danger"
                title={problemMessage(t, stats.error)}
                actions={
                  <AriaButton icon="refresh" onPress={() => void stats.refetch()}>
                    {t('common.retry')}
                  </AriaButton>
                }
              />
            )}
            {stats.data && (
              <>
                <Block id="stats-top" title={t('admin.stats.top.title')}>
                  <TopTitles titles={stats.data.topTitles} />
                </Block>
                <Block id="stats-profiles" title={t('admin.stats.profiles.title')}>
                  <WatchTime profiles={stats.data.watchTime} />
                </Block>
                <Block id="stats-playback" title={t('admin.stats.playback.title')}>
                  <Playback playback={stats.data.playback} />
                </Block>
                <Block id="stats-devices" title={t('admin.stats.devices.title')}>
                  <Devices devices={stats.data.devices} />
                </Block>
              </>
            )}
          </div>
        </div>
      )}
    </Page>
  );
}
