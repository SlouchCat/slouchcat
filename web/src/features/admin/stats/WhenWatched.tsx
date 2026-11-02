import type { Stats } from '@slouchcat/api-client';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { formatDuration } from '@/lib/format';
import { Text } from '@/ui';
import { BarChart, DisclosureGroup, HeatGrid } from '@/ui/aria';
import styles from './Stats.module.css';
import {
  WEEKDAYS,
  byHour,
  byWeekday,
  hourGrid,
  peakIndex,
  peakOf,
  windowWeeks,
  type Weekday,
} from './when';

type WhenProps = Pick<Stats, 'watchTimeByQuarterHour' | 'timeWindowDays' | 'timeZone'>;

/**
 * «Wann geschaut wird» (concept 12, 10.2): the watch time of the last whole weeks by hour of the
 * day and by day of the week, side by side, and folded away below them the grid of 7 × 24 hours
 * with its peak in the heading line. The hours are those of the server's time zone, named under
 * the charts. Every chart carries its numbers as a table for screen readers.
 */
export function WhenWatched({ watchTimeByQuarterHour, timeWindowDays, timeZone }: WhenProps) {
  const { t } = useTranslation();
  const [gridOpen, setGridOpen] = useState(false);
  const grid = hourGrid(watchTimeByQuarterHour);
  const hours = byHour(grid);
  const days = byWeekday(grid);
  const peak = peakOf(grid);
  const hourSpan = (hour: number) => t('admin.stats.when.hourSpan', { from: hour, to: hour + 1 });
  const dayName = (day: Weekday) => t(`ui.weekdays.long.${day}`);
  const duration = (ms: number) => formatDuration(t, ms);
  const note = (
    <Text tone="secondary">
      {t('admin.stats.when.window', { count: windowWeeks(timeWindowDays) })}
    </Text>
  );
  if (!peak) {
    return (
      <div className={styles.when}>
        {note}
        <Text tone="secondary">{t('admin.stats.when.empty')}</Text>
      </div>
    );
  }
  const topHour = peakIndex(hours) ?? 0;
  const topDay = WEEKDAYS[peakIndex(days) ?? 0] ?? 'mon';

  return (
    <div className={styles.when}>
      {note}
      <div className={styles.whenCharts}>
        <div className={styles.whenChart}>
          <BarChart
            title={t('admin.stats.when.byHour')}
            columns={[t('admin.stats.when.columns.hour'), t('admin.stats.columns.watched')]}
            bars={hours.map((value, hour) => {
              const label = hourSpan(hour);
              const valueText = duration(value);
              return {
                key: String(hour),
                label,
                tick: String(hour),
                value,
                valueText,
                tooltip: t('admin.stats.tooltip', { label, value: valueText }),
              };
            })}
          />
          <Text variant="caption" tone="secondary">
            {t('admin.stats.when.topHour', { hour: hourSpan(topHour) })}
          </Text>
        </div>
        <div className={styles.whenChart}>
          <BarChart
            orientation="horizontal"
            title={t('admin.stats.when.byWeekday')}
            columns={[t('admin.stats.when.columns.day'), t('admin.stats.columns.watched')]}
            bars={WEEKDAYS.map((day, index) => {
              const label = dayName(day);
              const value = days[index] ?? 0;
              const valueText = duration(value);
              return {
                key: day,
                label,
                tick: t(`ui.weekdays.short.${day}`),
                value,
                valueText,
                tooltip: t('admin.stats.tooltip', { label, value: valueText }),
              };
            })}
          />
          <Text variant="caption" tone="secondary">
            {t('admin.stats.when.topDay', { day: dayName(topDay) })}
          </Text>
        </div>
      </div>
      <Text variant="caption" tone="muted">
        {t('admin.stats.when.zone', { zone: timeZone })}
      </Text>
      <DisclosureGroup
        title={t('admin.stats.when.grid')}
        meta={
          <Text as="span" variant="caption" tone="secondary">
            {t('admin.stats.when.peak', {
              day: dayName(peak.day),
              hour: hourSpan(peak.hour),
            })}
          </Text>
        }
        isExpanded={gridOpen}
        onExpandedChange={setGridOpen}
      >
        <HeatGrid title={t('admin.stats.when.gridChart')} values={grid} formatValue={duration} />
      </DisclosureGroup>
    </div>
  );
}
