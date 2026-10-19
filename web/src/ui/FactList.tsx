import styles from './FactList.module.css';

interface FactListProps {
  /** Term and value per fact, in display order; terms are unique. */
  facts: readonly (readonly [string, string])[];
  /**
   * `grid`: the facts side by side, the term small above its value (detail and administration
   * pages); `rows`: term and value on one line, the value in monospace (the player's stream info).
   */
  layout?: 'grid' | 'rows';
}

/** A description list of short facts about a title or a file (container, codecs, ids). */
export function FactList({ facts, layout = 'grid' }: FactListProps) {
  return (
    <dl className={`${styles.list} ${styles[layout]}`}>
      {facts.map(([term, value]) => (
        <div key={term} className={styles.fact}>
          <dt className={styles.term}>{term}</dt>
          <dd className={styles.value}>{value}</dd>
        </div>
      ))}
    </dl>
  );
}
