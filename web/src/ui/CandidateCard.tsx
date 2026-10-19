import styles from './CandidateCard.module.css';

interface CandidateCardProps {
  title: string;
  /** Second line, the original title. */
  subtitle?: string;
  overview?: string;
  /**
   * Poster from the metadata provider (an external URL): loaded without a referrer, so the
   * provider does not learn the address of the server.
   */
  posterUrl?: string;
  /** Spoken name of the button ("Alien (1979) zuordnen"). */
  'aria-label': string;
  isDisabled?: boolean;
  onPress: () => void;
}

/**
 * One search result of the metadata match (B-6): poster, title, original title and overview as
 * one button that picks it.
 */
export function CandidateCard({
  title,
  subtitle,
  overview,
  posterUrl,
  'aria-label': ariaLabel,
  isDisabled = false,
  onPress,
}: CandidateCardProps) {
  return (
    <button
      type="button"
      className={styles.card}
      aria-label={ariaLabel}
      disabled={isDisabled}
      onClick={onPress}
    >
      {posterUrl ? (
        <img
          className={styles.poster}
          src={posterUrl}
          alt=""
          width={80}
          height={120}
          loading="lazy"
          referrerPolicy="no-referrer"
        />
      ) : (
        <span className={styles.poster} aria-hidden="true" />
      )}
      <span className={styles.text}>
        <span className={styles.title}>{title}</span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
        {overview && <span className={styles.overview}>{overview}</span>}
      </span>
    </button>
  );
}
