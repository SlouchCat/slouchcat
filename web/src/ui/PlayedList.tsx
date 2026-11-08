import { useId } from 'react';
import styles from './PlayedList.module.css';
import { StationLogo } from './StationLogo';

// «Zuvor gespielt» of «Jetzt läuft» (concept 16, 7.6 and 8): the titles before the one that plays,
// newest first, each with its cover (the station's logo while it has none) and the time it began.
// The texts and the times come from the app (rule 9).

export interface PlayedEntry {
  key: string;
  title: string;
  /** The artist, when the stream named one. */
  subtitle?: string;
  /** When it began, as shown («14:32»), and as `dateTime` of the `<time>`. */
  time: string;
  dateTime: string;
  coverUrl?: string;
}

interface PlayedListProps {
  heading: string;
  /** The text when the station sent no earlier titles. */
  empty: string;
  stationName: string;
  logoUrl?: string;
  entries: readonly PlayedEntry[];
}

export function PlayedList({ heading, empty, stationName, logoUrl, entries }: PlayedListProps) {
  const headingId = useId();
  return (
    <section className={styles.played} aria-labelledby={headingId} data-testid="radio-played">
      <h3 id={headingId} className={styles.heading}>
        {heading}
      </h3>
      {entries.length === 0 ? (
        <p className={styles.empty}>{empty}</p>
      ) : (
        <ol className={styles.list}>
          {entries.map((entry) => (
            <li key={entry.key} className={styles.entry}>
              <StationLogo
                name={stationName}
                src={entry.coverUrl ?? logoUrl}
                fallbackSrc={entry.coverUrl ? logoUrl : undefined}
                size="sm"
              />
              <span className={styles.text}>
                <span className={styles.title}>{entry.title}</span>
                {entry.subtitle && <span className={styles.subtitle}>{entry.subtitle}</span>}
              </span>
              <time className={styles.time} dateTime={entry.dateTime}>
                {entry.time}
              </time>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
