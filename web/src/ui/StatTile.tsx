import styles from './StatTile.module.css';

interface StatTileProps {
  /** What is counted ("Fehlgeschlagen"). */
  label: string;
  /** The number, formatted for the language. */
  value: string;
  /** A line below the number, such as its share ("50 % aller Wiedergaben"). */
  note?: string;
  /** Marks the tile's edge: `danger` for something that needs attention. */
  tone?: 'neutral' | 'success' | 'warning' | 'danger';
}

/**
 * A number with its label (dashboard counters, statistics): a description list of its own, so
 * the label is the term and the number its value; the number shows large, the label below it.
 */
export function StatTile({ label, value, note, tone = 'neutral' }: StatTileProps) {
  return (
    <dl className={`${styles.tile} ${styles[tone]}`}>
      <dt className={styles.label}>{label}</dt>
      <dd className={styles.value}>{value}</dd>
      {note && <dd className={styles.note}>{note}</dd>}
    </dl>
  );
}
