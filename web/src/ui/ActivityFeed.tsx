import styles from './ActivityFeed.module.css';
import { Icon, type IconName } from './Icon';

/** Colour of an entry's icon: neutral, the profile's accent, or a state. */
export type ActivityTone = 'neutral' | 'accent' | 'success' | 'warning' | 'danger';

export interface ActivityEntry {
  id: string;
  icon: IconName;
  tone?: ActivityTone;
  /** The sentence, built by the page from the locale ("Anna hat Oppenheimer gestartet"). */
  text: string;
  /** Relative time as shown ("vor 5 min"). */
  time: string;
  /** The moment as ISO 8601, for the <time> element. */
  dateTime?: string;
}

interface ActivityFeedProps {
  /** Accessible name of the list ("Letzte Aktivität"). */
  label: string;
  entries: readonly ActivityEntry[];
}

/**
 * The newest entries of the activity log (concept 11, 11.4): one line each with icon, sentence and
 * relative time. A sentence too long for the line is cut and shown whole as its title.
 */
export function ActivityFeed({ label, entries }: ActivityFeedProps) {
  return (
    <ul className={styles.feed} aria-label={label}>
      {entries.map((entry) => (
        <li key={entry.id} className={styles.entry}>
          <span className={`${styles.icon} ${styles[entry.tone ?? 'neutral']}`}>
            <Icon name={entry.icon} size="sm" />
          </span>
          <span className={styles.text} title={entry.text}>
            {entry.text}
          </span>
          <time className={styles.time} dateTime={entry.dateTime}>
            {entry.time}
          </time>
        </li>
      ))}
    </ul>
  );
}
