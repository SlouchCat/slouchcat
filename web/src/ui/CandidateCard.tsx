import type { ReactNode } from 'react';
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
  /** The picture's shape: a film poster, or the square cover of an audiobook (concept 19 D5.2). */
  shape?: 'portrait' | 'square';
  /** Further short lines under the title: authors and narrators, publisher, year and id. */
  details?: readonly string[];
  /** Something small at the end of the card: the length of an audiobook entry. */
  badge?: ReactNode;
  /** The picked candidate of a list whose pick opens a comparison (`aria-pressed`). */
  isSelected?: boolean;
  /** Spoken name of the button ("Alien (1979) zuordnen"). */
  'aria-label': string;
  isDisabled?: boolean;
  onPress: () => void;
}

/**
 * One search result of the metadata match (B-6): poster, title, original title and overview as
 * one button that picks it. An audiobook entry (concept 19 D5.2) has a square cover, the people
 * and facts as lines of their own and its length as a badge.
 */
export function CandidateCard({
  title,
  subtitle,
  overview,
  posterUrl,
  shape = 'portrait',
  details,
  badge,
  isSelected,
  'aria-label': ariaLabel,
  isDisabled = false,
  onPress,
}: CandidateCardProps) {
  const square = shape === 'square';
  return (
    <button
      type="button"
      className={styles.card}
      aria-label={ariaLabel}
      aria-pressed={isSelected}
      disabled={isDisabled}
      onClick={onPress}
    >
      {posterUrl ? (
        <img
          className={styles.poster}
          data-shape={shape}
          src={posterUrl}
          alt=""
          width={80}
          height={square ? 80 : 120}
          loading="lazy"
          referrerPolicy="no-referrer"
        />
      ) : (
        <span className={styles.poster} data-shape={shape} aria-hidden="true" />
      )}
      <span className={styles.text}>
        <span className={styles.title}>{title}</span>
        {subtitle && <span className={styles.subtitle}>{subtitle}</span>}
        {details?.map((line) => (
          <span key={line} className={styles.subtitle}>
            {line}
          </span>
        ))}
        {overview && <span className={styles.overview}>{overview}</span>}
      </span>
      {badge && <span className={styles.badge}>{badge}</span>}
    </button>
  );
}
