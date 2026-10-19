import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import styles from './EpisodeCard.module.css';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

interface EpisodeCardProps {
  href: string;
  /** Episode code ("S01E02", "S01E03–E04"), translated by the page. */
  code: string;
  title: string;
  /** Secondary line (runtime). */
  caption?: string;
  /** 16:9 still (backdropSmall of the episode), or undefined for the empty frame. */
  stillUrl?: string;
  accent?: string;
  progress?: { percent: number; label: string };
  focusKey?: string;
}

/**
 * Episode in a season list: 16:9 still, code, title and runtime, one link to the episode. Focus
 * shows the ring and the card's accent like a poster card.
 */
export function EpisodeCard({
  href,
  code,
  title,
  caption,
  stillUrl,
  accent,
  progress,
  focusKey,
}: EpisodeCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink
      ref={ref}
      href={href}
      className={styles.card}
      style={itemAccentStyle(accent)}
      onFocus={onFocus}
    >
      <span className={styles.frame}>
        {stillUrl && (
          <img
            className={styles.image}
            src={stillUrl}
            alt=""
            width={780}
            height={439}
            loading="lazy"
            decoding="async"
            draggable={false}
          />
        )}
        {progress && (
          <span className={styles.progress}>
            <ProgressBar value={progress.percent} label={progress.label} />
          </span>
        )}
      </span>
      <span className={styles.meta}>
        <span className={styles.code}>{code}</span>
        <span className={styles.title}>{title}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
      </span>
    </AppLink>
  );
}
