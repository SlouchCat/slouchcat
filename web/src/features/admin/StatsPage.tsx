import type { Stats, StatsDays } from '@slouchcat/api-client';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDuration } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, VisuallyHidden } from '@/ui';
import { AdminPage, AriaButton, BandPanel, RadioGroup, StatusStrip } from '@/ui/aria';
import { statsQuery } from './queries';
import { Devices, WatchTime } from './stats/Breakdown';
import styles from './stats/Stats.module.css';
import { TopTitles } from './stats/TopTitles';
import { WhenWatched } from './stats/WhenWatched';
import { DEFAULT_STATS_DAYS, STATS_DAYS } from './statsDays';

const route = getRouteApi('/_authed/_active/_chrome/admin/stats');

/** The server sends at most this many most watched titles (`Stats.topTitles`). */
const TOP_TITLES = 20;

/**
 * «Wiedergabe» (STA-2) as the page's status strip: direct play, remux and transcode with their
 * shares of all plays, and the share of the transcodes that ran on the GPU.
 */
function playbackTiles(t: TFunction, language: string, playback: Stats['playback']) {
  const percent = new Intl.NumberFormat(language, { style: 'percent', maximumFractionDigits: 0 });
  const total = playback.direct + playback.remux + playback.transcode;
  const share = (count: number) => percent.format(total === 0 ? 0 : count / total);
  const counted = [
    { key: 'direct', count: playback.direct, note: share(playback.direct) },
    { key: 'remux', count: playback.remux, note: share(playback.remux) },
    { key: 'transcode', count: playback.transcode, note: percent.format(playback.transcodeShare) },
  ] as const;
  return [
    ...counted.map((tile) => ({
      key: tile.key,
      label: t(`admin.stats.playback.${tile.key}`),
      value: t('admin.stats.playback.sessions', { count: tile.count }),
      note: t('admin.stats.playback.share', { share: tile.note }),
    })),
    {
      key: 'hardware',
      label: t('admin.stats.playback.hardware'),
      value: percent.format(playback.hardwareShare),
      note: t('admin.stats.playback.hardwareNote'),
    },
  ];
}

/**
 * Statistics (A-8; concept 12, mockup Sta-B «Raster») over 7, 30, 90 or 365 days and every
 * profile. The period sits in the header and in the address (`?days=`); below it «Wiedergabe» as
 * the status strip, «Meistgesehen» with places 1 to 3 as tiles, the watch time per profile and per
 * device type side by side, and «Wann geschaut wird». The server counts; the page shows the
 * numbers and only adds up the quarter hours of the week.
 */
export function StatsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.stats.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.stats.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <StatsContent />;
}

function StatsContent() {
  const { t, i18n } = useTranslation();
  const { days = DEFAULT_STATS_DAYS } = route.useSearch();
  const navigate = route.useNavigate();
  // The previous period stays on screen while the next one loads.
  const stats = useQuery({ ...statsQuery(days), placeholderData: keepPreviousData });
  // After a switch of the period the status region says once the new numbers are there.
  const [switched, setSwitched] = useState(false);
  const loaded = switched && stats.isSuccess && !stats.isFetching;
  const data = stats.data;

  return (
    <AdminPage
      title={t('admin.stats.title')}
      help={t('admin.stats.intro')}
      actions={
        <RadioGroup
          name="days"
          label={t('admin.stats.period')}
          inline
          value={String(days)}
          onChange={(value) => {
            setSwitched(true);
            void navigate({ search: { days: Number(value) as StatsDays }, replace: true });
          }}
          options={STATS_DAYS.map((option) => ({
            value: String(option),
            label: t('admin.stats.days', { count: option }),
          }))}
        />
      }
    >
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
      {data && (
        <div
          className={styles.content}
          aria-busy={stats.isFetching}
          data-switching={stats.isPlaceholderData || undefined}
        >
          <StatusStrip
            label={t('admin.stats.playback.title')}
            tiles={playbackTiles(t, i18n.language, data.playback)}
          />
          <BandPanel
            title={t('admin.stats.top.title')}
            icon="star"
            meta={
              data.topTitles.length >= TOP_TITLES
                ? t('admin.stats.top.full', { count: TOP_TITLES })
                : t('admin.stats.top.count', { count: data.topTitles.length })
            }
          >
            <TopTitles titles={data.topTitles} />
          </BandPanel>
          <div className={styles.pair}>
            <BandPanel
              title={t('admin.stats.profiles.title')}
              icon="user"
              meta={
                data.watchTime.length > 0
                  ? t('admin.stats.profiles.summary', {
                      count: data.watchTime.length,
                      watched: formatDuration(
                        t,
                        data.watchTime.reduce((sum, profile) => sum + profile.watchedMs, 0),
                      ),
                    })
                  : undefined
              }
            >
              <WatchTime profiles={data.watchTime} />
            </BandPanel>
            <BandPanel
              title={t('admin.stats.devices.title')}
              icon="monitor"
              meta={
                data.devices.length > 0
                  ? t('admin.stats.devices.summary', {
                      count: data.devices.reduce((sum, device) => sum + device.count, 0),
                      watched: formatDuration(
                        t,
                        data.devices.reduce((sum, device) => sum + device.watchedMs, 0),
                      ),
                    })
                  : undefined
              }
            >
              <Devices devices={data.devices} />
            </BandPanel>
          </div>
          <BandPanel title={t('admin.stats.when.title')} icon="grid">
            <WhenWatched
              watchTimeByQuarterHour={data.watchTimeByQuarterHour}
              timeWindowDays={data.timeWindowDays}
              timeZone={data.timeZone}
            />
          </BandPanel>
        </div>
      )}
      <VisuallyHidden as="div" role="status">
        {loaded && t('admin.stats.loaded', { period: t('admin.stats.days', { count: days }) })}
      </VisuallyHidden>
    </AdminPage>
  );
}
