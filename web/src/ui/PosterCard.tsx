import { Link } from 'react-aria-components';
import styles from './PosterCard.module.css';
import { PosterFallback } from './PosterFallback';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

interface PosterCardProps {
  href: string;
  title: string;
  /** Secondary line under the title (year, remaining time). */
  caption?: string;
  /** Same-origin poster URL (w500), or undefined for the lettered fallback tile. */
  posterUrl?: string;
  /** Watch progress 0 … 100 with its accessible label. */
  progress?: { percent: number; label: string };
  /** Load the image eagerly (the first cards of the first row); every other poster is lazy. */
  eager?: boolean;
  /** Keep the image out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  focusKey?: string;
}

/**
 * 2:3 poster with its title below, one link to the title. The link carries the focus ring and
 * scales on focus (focus.scale token).
 */
export function PosterCard({
  href,
  title,
  caption,
  posterUrl,
  progress,
  eager = false,
  deferImage = false,
  focusKey,
}: PosterCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <Link ref={ref} href={href} className={styles.card} onFocus={onFocus}>
      <span className={styles.frame}>
        {deferImage ? null : posterUrl ? (
          <img
            className={styles.image}
            src={posterUrl}
            alt=""
            width={500}
            height={750}
            loading={eager ? 'eager' : 'lazy'}
            decoding="async"
            draggable={false}
          />
        ) : (
          <PosterFallback />
        )}
        {progress && (
          <span className={styles.progress}>
            <ProgressBar value={progress.percent} label={progress.label} />
          </span>
        )}
      </span>
      <span className={styles.meta}>
        <span className={styles.title}>{title}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
      </span>
    </Link>
  );
}
