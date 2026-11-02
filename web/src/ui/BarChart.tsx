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
  /**
   * `horizontal`: one row per bar with its `tick` before it and its `valueText` after it (the
   * days of the week, concept 12); the threshold line is drawn by vertical charts only.
   */
  orientation?: 'vertical' | 'horizontal';
  /**
   * Vertical charts label every so many bars (3: the hours 0, 3, 6 …); without it the first,
   * the middle and the last bar.
   */
  tickEvery?: number;
}

/**
 * Bars per category (days of a report, days of the week) with an optional threshold line. The
 * bars are decoration: the same numbers stand in a table for screen readers, so the chart needs
 * no focus stops of its own. Vertical by default; `horizontal` lists the bars as rows.
 */
export function BarChart({
  title,
  bars,
  columns,
  threshold,
  orientation = 'vertical',
  tickEvery,
}: BarChartProps) {
  const id = useId();
  const horizontal = orientation === 'horizontal';
  const max = Math.max(
    1,
    horizontal ? 0 : (threshold?.value ?? 0),
    ...bars.map((bar) => bar.value),
  );
  const percent = (value: number) => `${Math.min(100, (value / max) * 100)}%`;
  const ticks = new Set(
    tickEvery
      ? bars.flatMap((_bar, index) => (index % tickEvery === 0 ? [index] : []))
      : [0, Math.floor((bars.length - 1) / 2), bars.length - 1],
  );
  const table = (
    <div className={styles.table}>
      <table>
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
    </div>
  );
  if (horizontal) {
    return (
      <figure className={styles.chart} aria-labelledby={id}>
        <figcaption id={id} className={styles.title}>
          {title}
        </figcaption>
        <div className={styles.rows} aria-hidden="true">
          {bars.map((bar) => (
            <div key={bar.key} className={styles.barRow} title={bar.tooltip}>
              <span className={styles.rowLabel}>{bar.tick}</span>
              <span className={styles.track}>
                <span className={styles.hbar} style={{ width: percent(bar.value) }} />
              </span>
              <span className={styles.rowValue}>{bar.valueText}</span>
            </div>
          ))}
        </div>
        {table}
      </figure>
    );
  }
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
      {table}
    </figure>
  );
}
