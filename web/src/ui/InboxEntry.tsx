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
  /**
   * The whole entry as one spoken line, from the locale ("Ungelesen: S01E03 · Pilot, Neue Folge
   * von …, 21.10.2026"); on screen the dot shows "unread".
   */
  label: string;
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
  label,
  focusKey,
  onOpen,
}: InboxEntryProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink
      ref={ref}
      href={href}
      className={styles.row}
      aria-label={label}
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
