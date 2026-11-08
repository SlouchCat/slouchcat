import styles from './CoverageBars.module.css';

// The coverage of the radio's facets (concept 16, 5.4 and 11): per facet the share of the
// stations with a value, as a bar, the percentage and the counts. The bar is decoration: the
// numbers stand beside it as text. Below 70 % in the warning tone, from 85 % in the success tone.

export interface CoverageRow {
  key: string;
  /** The facet («Genre»). */
  label: string;
  /** Stations with a value and the base, both counted by the server. */
  assigned: number;
  total: number;
  /** «91 %», formatted by the page. */
  percentText: string;
  /** «36'200 von 38'912 Sendern», formatted by the page. */
  countText: string;
}

interface CoverageBarsProps {
  /** Accessible name of the list («Abdeckung je Filter»). */
  label: string;
  rows: readonly CoverageRow[];
}

const toneOf = (share: number) => (share >= 0.85 ? 'success' : share < 0.7 ? 'warning' : undefined);

export function CoverageBars({ label, rows }: CoverageBarsProps) {
  return (
    <ul className={styles.list} aria-label={label} data-testid="radio-coverage">
      {rows.map((row) => {
        const share = row.total > 0 ? Math.min(1, row.assigned / row.total) : 0;
        return (
          <li key={row.key} className={styles.row} data-tone={toneOf(share)}>
            <span className={styles.label}>{row.label}</span>
            <span className={styles.bar} aria-hidden="true">
              <span className={styles.fill} style={{ width: `${share * 100}%` }} />
            </span>
            <span className={styles.percent}>{row.percentText}</span>
            <span className={styles.count}>{row.countText}</span>
          </li>
        );
      })}
    </ul>
  );
}
