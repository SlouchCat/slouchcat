import styles from './NotificationBadge.module.css';

interface NotificationBadgeProps {
  /** Unread entries; nothing is shown for 0. */
  count: number;
  /** Highest number written out; above it the badge says "99+". */
  max?: number;
}

/**
 * Small counter in the profile accent (unread notifications in the header). Decorative: the
 * control it sits on names the count in its accessible label.
 */
export function NotificationBadge({ count, max = 99 }: NotificationBadgeProps) {
  if (count <= 0) return null;
  return (
    <span className={styles.badge} aria-hidden="true">
      {count > max ? `${max}+` : count}
    </span>
  );
}
