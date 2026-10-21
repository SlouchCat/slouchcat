import styles from './StatTile.module.css';

interface StatTileProps {
  /** What is counted ("Fehlgeschlagen"). */
  label: string;
  /** The number, formatted for the language. */
  value: string;
  /** Marks the tile's edge: `danger` for something that needs attention. */
  tone?: 'neutral' | 'success' | 'warning' | 'danger';
}

/** A number with its label (dashboard counters): the value large, the label below it. */
export function StatTile({ label, value, tone = 'neutral' }: StatTileProps) {
  return (
    <div className={`${styles.tile} ${styles[tone]}`}>
      <span className={styles.value}>{value}</span>
      <span className={styles.label}>{label}</span>
    </div>
  );
}
