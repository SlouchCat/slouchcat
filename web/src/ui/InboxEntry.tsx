import { AppLink } from './AppLink';
import styles from './InboxEntry.module.css';
import { useSpatialItem } from './spatial';

interface InboxEntryProps {
  href: string;
  /** What happened to what ("S01E03 · Pilot"). */
  title: string;
  /** Where it belongs ("Neue Folge von Serie X"). */
  context?: string;
  /** When, already formatted. */
  time: string;
  unread: boolean;
  /** Spoken state of an unread entry ("Ungelesen"); the dot shows it. */
  unreadLabel: string;
  focusKey?: string;
  /** Called on activation, before the link navigates (marks the entry read). */
  onOpen?: () => void;
}

/** One entry of the notification inbox: a link with an accent dot while unread. */
export function InboxEntry({
  href,
  title,
  context,
  time,
  unread,
  unreadLabel,
  focusKey,
  onOpen,
}: InboxEntryProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink
      ref={ref}
      href={href}
      className={styles.row}
      // One spoken line with separators; the dot alone shows "unread" on screen.
      aria-label={[unread ? `${unreadLabel}: ${title}` : title, context, time]
        .filter(Boolean)
        .join(', ')}
      data-unread={unread || undefined}
      onFocus={onFocus}
      onClick={onOpen}
    >
      <span className={styles.dot} aria-hidden="true" />
      <span className={styles.title}>{title}</span>
      {context && <span className={styles.context}>{context}</span>}
      <span className={styles.time}>{time}</span>
    </AppLink>
  );
}
