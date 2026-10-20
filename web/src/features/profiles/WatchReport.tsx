import type { Profile } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, BarChart } from '@/ui/aria';
import { watchReportQuery } from './queries';
import styles from './ProfileSettingsPage.module.css';

/** Days of the report (K-5: the last 30 days including today). */
export const REPORT_DAYS = 30;

/** "2026-10-19" as a date of that calendar day, without a time-zone shift. */
const calendarDay = (date: string) => {
  const [year, month, day] = date.split('-').map(Number);
  return new Date(year ?? 1970, (month ?? 1) - 1, day ?? 1);
};

/**
 * Watch time of one profile per day (K-5): bars for the last 30 days, today and the limit. The
 * server counts the minutes; the page only shows them.
 */
export function WatchReport({ profile }: { profile: Profile }) {
  const { t, i18n } = useTranslation();
  const report = useQuery(watchReportQuery(profile.id, REPORT_DAYS));
  const headingId = `watch-report-${profile.id}`;
  const long = new Intl.DateTimeFormat(i18n.language, { dateStyle: 'medium' });
  const short = new Intl.DateTimeFormat(i18n.language, { day: 'numeric', month: 'numeric' });

  const days = report.data?.days ?? [];
  const today = days.at(-1)?.minutes ?? 0;
  const total = days.reduce((sum, day) => sum + day.minutes, 0);
  const limit = report.data?.limitMinutes;

  return (
    <section className={styles.report} aria-labelledby={headingId}>
      <h3 id={headingId} className={styles.reportTitle}>
        {t('profiles.settings.report.title', { name: profile.name })}
      </h3>
      <Text tone="secondary" variant="caption">
        {t('profiles.settings.report.intro')}
      </Text>
      {report.isPending && <Skeleton shape="block" />}
      {report.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, report.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void report.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {report.data && (
        <>
          <p className={styles.reportFacts}>
            <span>{t('profiles.settings.report.today', { count: today })}</span>
            <span>{t('profiles.settings.report.total', { count: total })}</span>
            {limit !== undefined && (
              <span>{t('profiles.settings.report.limit', { count: limit })}</span>
            )}
          </p>
          <BarChart
            title={t('profiles.settings.report.chart')}
            columns={[t('profiles.settings.report.date'), t('profiles.settings.report.minutes')]}
            bars={days.map((day) => {
              const label = long.format(calendarDay(day.date));
              const valueText = t('profiles.settings.report.value', { count: day.minutes });
              return {
                key: day.date,
                label,
                tick: short.format(calendarDay(day.date)),
                value: day.minutes,
                valueText,
                tooltip: t('profiles.settings.report.tooltip', { date: label, value: valueText }),
              };
            })}
            threshold={
              limit !== undefined
                ? { value: limit, label: t('profiles.settings.report.value', { count: limit }) }
                : undefined
            }
          />
        </>
      )}
    </section>
  );
}
