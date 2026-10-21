import { AppLink } from './AppLink';
import { SpatialButtonLink } from './Button';
import styles from './EpisodeRow.module.css';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

interface EpisodeRowProps {
  /** The episode's detail page. */
  href: string;
  title: string;
  /** Date and duration ("13. Okt. 2025 · 45 min"). */
  caption?: string;
  /** Listening progress 0 … 100 with its accessible label. */
  progress?: { percent: number; label: string };
  /** The player of the episode, and the name of that link ("Folge 3 abspielen"). */
  playHref: string;
  playLabel: string;
  focusKey?: string;
}

/**
 * A podcast episode in the list of its podcast (M6): title, date and duration and the progress,
 * a link to the episode and a play button of its own. Both take part in the arrow navigation;
 * ← and → move between them, ↑ and ↓ between the episodes.
 */
export function EpisodeRow({
  href,
  title,
  caption,
  progress,
  playHref,
  playLabel,
  focusKey,
}: EpisodeRowProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <div className={styles.row}>
      <AppLink ref={ref} href={href} className={styles.link} onFocus={onFocus}>
        <span className={styles.title}>{title}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
        {progress && (
          <span className={styles.progress}>
            <ProgressBar value={progress.percent} label={progress.label} />
          </span>
        )}
      </AppLink>
      <SpatialButtonLink
        focusKey={focusKey ? `${focusKey}-play` : undefined}
        href={playHref}
        variant="secondary"
        icon="play"
        iconOnly
        aria-label={playLabel}
      />
    </div>
  );
}
