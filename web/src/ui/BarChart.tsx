import { useId } from 'react';
import styles from './BarChart.module.css';

export interface BarDatum {
  key: string;
  /** Row header of the table ("19.10.2026"). */
  label: string;
  /** Short label under the bar; shown for the first, the middle and the last bar only. */
  tick: string;
  value: number;
  /** The value in words for the table ("42 min"). */
  valueText: string;
  /** Tooltip of the bar ("19.10.2026: 42 min"). */
  tooltip: string;
}

interface BarChartProps {
  /** Caption of the chart and of its table. */
  title: string;
  bars: readonly BarDatum[];
  /** Column headers of the table ("Tag", "Minuten"). */
  columns: [label: string, value: string];
  /** A line across the chart (a daily limit); bars above it use the warning colour. */
  threshold?: { value: number; label: string };
}

/**
 * Vertical bars per category (days of a report) with an optional threshold line. The bars are
 * decoration: the same numbers stand in a table for screen readers, so the chart needs no focus
 * stops of its own.
 */
export function BarChart({ title, bars, columns, threshold }: BarChartProps) {
  const id = useId();
  const max = Math.max(1, threshold?.value ?? 0, ...bars.map((bar) => bar.value));
  const percent = (value: number) => `${Math.min(100, (value / max) * 100)}%`;
  const ticks = new Set([0, Math.floor((bars.length - 1) / 2), bars.length - 1]);
  return (
    <figure className={styles.chart} aria-labelledby={id}>
      <figcaption id={id} className={styles.title}>
        {title}
      </figcaption>
      <div className={styles.plot} aria-hidden="true">
        {threshold && (
          <div className={styles.threshold} style={{ bottom: percent(threshold.value) }}>
            <span className={styles.thresholdLabel}>{threshold.label}</span>
          </div>
        )}
        {bars.map((bar) => (
          <div key={bar.key} className={styles.column} title={bar.tooltip}>
            <div
              className={styles.bar}
              data-over={threshold !== undefined && bar.value > threshold.value ? '' : undefined}
              style={{ height: percent(bar.value) }}
            />
          </div>
        ))}
      </div>
      <div className={styles.ticks} aria-hidden="true">
        {bars.map((bar, index) => (
          <span key={bar.key} className={styles.tick}>
            {ticks.has(index) ? bar.tick : ''}
          </span>
        ))}
      </div>
      <table className={styles.table}>
        <caption>{title}</caption>
        <thead>
          <tr>
            <th scope="col">{columns[0]}</th>
            <th scope="col">{columns[1]}</th>
          </tr>
        </thead>
        <tbody>
          {bars.map((bar) => (
            <tr key={bar.key}>
              <th scope="row">{bar.label}</th>
              <td>{bar.valueText}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
