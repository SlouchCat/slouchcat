import { useId, type CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './HeatGrid.module.css';

const DAYS = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
/** Hours with a label above the grid. */
const TICKS = new Set([0, 6, 12, 18]);
/** Steps of the colour legend, in percent of the highest value. */
const LEGEND = [0, 25, 50, 75, 100];

interface HeatGridProps {
  /** Caption of the grid and of its table ("Wann geschaut wird"). */
  title: string;
  /** 7 × 24 values, Monday 00:00 first, then hour by hour (the order of the contract). */
  values: readonly number[];
  /** A value in words for the table and the tooltips ("1 h 20 min"). */
  formatValue: (value: number) => string;
}

/**
 * When something happens across the week (concept 12, 10.2: «Wann geschaut wird»): seven rows
 * Monday to Sunday, 24 hour cells each, the profile's accent the stronger the higher the value.
 * Like BarChart the cells are decoration: the same numbers stand in a table for screen readers
 * (weekdays as row headers, hours as column headers), so the grid has no focus stops.
 */
export function HeatGrid({ title, values, formatValue }: HeatGridProps) {
  const { t } = useTranslation();
  const id = useId();
  const max = Math.max(1, ...values);
  const value = (day: number, hour: number) => values[day * 24 + hour] ?? 0;
  const hourLabel = (hour: number) => t('ui.heatGrid.hour', { hour });
  return (
    <figure className={styles.chart} aria-labelledby={id}>
      <figcaption id={id} className={styles.title}>
        {title}
      </figcaption>
      <div className={styles.grid} aria-hidden="true">
        <span />
        {HOURS.map((hour) => (
          <span key={hour} className={styles.tick}>
            {TICKS.has(hour) ? hour : ''}
          </span>
        ))}
        {DAYS.map((day, row) => [
          <span key={day} className={styles.day}>
            {t(`ui.weekdays.short.${day}`)}
          </span>,
          ...HOURS.map((hour) => {
            const amount = value(row, hour);
            return (
              <span
                key={`${day}-${hour}`}
                className={styles.cell}
                title={`${t(`ui.weekdays.long.${day}`)}, ${hourLabel(hour)}: ${formatValue(amount)}`}
                style={{ '--heat': Math.round((amount / max) * 100) } as CSSProperties}
              />
            );
          }),
        ])}
      </div>
      <div className={styles.legend} aria-hidden="true">
        {t('ui.heatGrid.less')}
        {LEGEND.map((step) => (
          <span
            key={step}
            className={`${styles.cell} ${styles.swatch}`}
            style={{ '--heat': step } as CSSProperties}
          />
        ))}
        {t('ui.heatGrid.more')}
      </div>
      <table className={styles.table}>
        <caption>{title}</caption>
        <thead>
          <tr>
            <th scope="col">{t('ui.heatGrid.day')}</th>
            {HOURS.map((hour) => (
              <th key={hour} scope="col">
                {hourLabel(hour)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {DAYS.map((day, row) => (
            <tr key={day}>
              <th scope="row">{t(`ui.weekdays.long.${day}`)}</th>
              {HOURS.map((hour) => (
                <td key={hour}>{formatValue(value(row, hour))}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
