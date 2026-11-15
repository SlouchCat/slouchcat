import { AppLink } from './AppLink';
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
  /** 56 px high, label above the number: a tile of a StatusStrip (concept 12, S-4). */
  compact?: boolean;
  /**
   * The tile is a link to the page behind the number (the key figures of the admin overview,
   * build plan 3); the label, number and note make its name.
   */
  href?: string;
}

/**
 * A number with its label (dashboard counters, statistics): a description list of its own, so
 * the label is the term and the number its value; the number shows large, the label below it.
 */
export function StatTile({ label, value, note, tone = 'neutral', compact, href }: StatTileProps) {
  const tile = (
    <dl className={`${styles.tile} ${styles[tone]} ${compact ? styles.compact : ''}`}>
      <dt className={styles.label}>{label}</dt>
      <dd className={styles.value}>{value}</dd>
      {note && <dd className={styles.note}>{note}</dd>}
    </dl>
  );
  if (href === undefined) return tile;
  return (
    <AppLink href={href} className={styles.link}>
      {tile}
    </AppLink>
  );
}
