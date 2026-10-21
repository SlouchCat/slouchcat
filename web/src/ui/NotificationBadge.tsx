import styles from './NotificationBadge.module.css';

/** Highest count the badge writes out. */
export const BADGE_MAX = 99;

interface NotificationBadgeProps {
  /** Unread entries; nothing is shown for 0. */
  count: number;
  /** Highest number written out. */
  max?: number;
  /** Shown above `max` ("99+", from the locale). */
  overflowLabel: string;
}

/**
 * Small counter in the profile accent (unread notifications in the header). Decorative: the
 * control it sits on names the count in its accessible label.
 */
export function NotificationBadge({
  count,
  max = BADGE_MAX,
  overflowLabel,
}: NotificationBadgeProps) {
  if (count <= 0) return null;
  return (
    <span className={styles.badge} aria-hidden="true">
      {count > max ? overflowLabel : count}
    </span>
  );
}
