import type { ReactNode } from 'react';
import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import { SpatialButtonLink } from './Button';
import { Icon } from './Icon';
import { PosterFallback } from './PosterFallback';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';
import styles from './VolumeRow.module.css';

interface VolumeRowProps {
  /** The volume's detail page: number, cover and text are one link. */
  href: string;
  /** The number of the volume as shown ("0,5", "3"); absent without a number. */
  number?: string;
  /** Spoken name of the link when the number and the title alone do not say enough. */
  label?: string;
  title: string;
  /** The line under the title: the narrator, "Gehört", the time left. */
  caption?: string;
  /** Same-origin square cover (posterSmall), or undefined for the lettered tile. */
  coverUrl?: string;
  accent?: string;
  /** Listening progress 0 … 100 with its accessible label. */
  progress?: { percent: number; label: string };
  /**
   * A finished volume: the cover a little dimmed and, without `watchedControl`, a check mark on it
   * with this as its spoken name ("Gehört").
   */
  watchedLabel?: string;
  /** The length of the volume ("11 h 11 min"). */
  duration?: string;
  /** The player of the volume, and the name of that link ("Abspielen: Die Geisel"). */
  playHref: string;
  playLabel: string;
  /**
   * A control after the play button: the pen of the metadata editor (concept 19 D5.1). A plain
   * button, so Tab reaches it and the arrow keys pass it by.
   */
  tool?: ReactNode;
  /**
   * The check mark «gehört» as a button at the end of the row (concept 20 P-5, WatchedToggle),
   * after the pen: Tab reaches it, the arrow keys pass it by. It replaces the check on the cover.
   */
  watchedControl?: ReactNode;
  focusKey?: string;
}

/**
 * A volume in the list of an audiobook series (concept 19 D4.3): number, square cover, title, a
 * line under it and the progress as one link to the volume, then its length, a play button of its
 * own, the pen and the check mark. ← and → move between link and play button, ↑ and ↓ between the volumes.
 */
export function VolumeRow({
  href,
  number,
  label,
  title,
  caption,
  coverUrl,
  accent,
  progress,
  watchedLabel,
  duration,
  playHref,
  playLabel,
  tool,
  watchedControl,
  focusKey,
}: VolumeRowProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <div className={styles.row} style={itemAccentStyle(accent)}>
      <AppLink ref={ref} href={href} className={styles.link} aria-label={label} onFocus={onFocus}>
        <span className={styles.number} aria-hidden={label ? 'true' : undefined}>
          {number}
        </span>
        <span className={watchedLabel ? `${styles.cover} ${styles.heard}` : styles.cover}>
          {coverUrl ? (
            <img
              className={styles.image}
              src={coverUrl}
              alt=""
              width={200}
              height={200}
              loading="lazy"
              decoding="async"
              draggable={false}
            />
          ) : (
            <PosterFallback />
          )}
          {watchedLabel && !watchedControl && (
            <span className={styles.check} role="img" aria-label={watchedLabel}>
              <Icon name="check" size="sm" />
            </span>
          )}
        </span>
        <span className={styles.text}>
          <span className={styles.title}>{title}</span>
          {caption && <span className={styles.caption}>{caption}</span>}
          {progress && (
            <span className={styles.progress}>
              <ProgressBar value={progress.percent} label={progress.label} />
            </span>
          )}
        </span>
      </AppLink>
      {duration && <span className={styles.duration}>{duration}</span>}
      <SpatialButtonLink
        focusKey={focusKey ? `${focusKey}-play` : undefined}
        href={playHref}
        variant="secondary"
        icon="play"
        iconOnly
        aria-label={playLabel}
      />
      {tool}
      {watchedControl}
    </div>
  );
}
