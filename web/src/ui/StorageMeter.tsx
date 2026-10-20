import styles from './StorageMeter.module.css';

export interface StoragePart {
  key: string;
  /** Bytes (or any unit shared with `total`). */
  value: number;
  /** Legend text, e.g. "Downloads: 2,1 GB". */
  label: string;
  tone: 'accent' | 'neutral';
}

interface StorageMeterProps {
  /** Accessible name of the meter ("Speicherbelegung"). */
  label: string;
  /** Spoken value ("2,5 GB von 14 GB belegt"). */
  valueText: string;
  /** The whole capacity; the parts fill it from the left. */
  total: number;
  parts: readonly StoragePart[];
  /** Line under the legend (free space). */
  caption?: string;
}

/**
 * How full a store is (O-7): coloured parts of one bar (downloads in the accent, the rest of the
 * site's data neutral) with a legend. A meter for screen readers, with the total as its maximum.
 */
export function StorageMeter({ label, valueText, total, parts, caption }: StorageMeterProps) {
  const used = parts.reduce((sum, part) => sum + Math.max(0, part.value), 0);
  const share = (value: number) =>
    total > 0 ? Math.min(100, (Math.max(0, value) / total) * 100) : 0;
  return (
    <div className={styles.storage}>
      <div
        role="meter"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(share(used))}
        aria-valuetext={valueText}
        className={styles.track}
      >
        {parts.map((part) => (
          <span
            key={part.key}
            className={`${styles.part} ${styles[part.tone]}`}
            style={{ width: `${share(part.value)}%` }}
          />
        ))}
      </div>
      <ul className={styles.legend}>
        {parts.map((part) => (
          <li key={part.key} className={styles.entry}>
            <span className={`${styles.swatch} ${styles[part.tone]}`} aria-hidden="true" />
            {part.label}
          </li>
        ))}
      </ul>
      {caption && <p className={styles.caption}>{caption}</p>}
    </div>
  );
}
