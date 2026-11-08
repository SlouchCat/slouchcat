import { useId } from 'react';
import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import styles from './EpisodeListItem.module.css';
import { Icon } from './Icon';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

interface EpisodeListItemProps {
  /** The episode's page: the whole row is one link. */
  href: string;
  /** Episode code ("S01E02", "S01E03–E04"), translated by the page. */
  code?: string;
  title: string;
  /** The episode's overview, at most two lines; read as the link's description, not its name. */
  overview?: string;
  /** Runtime ("45 min"). */
  caption?: string;
  /** 16:9 still (backdropSmall of the episode), or undefined for the tinted frame. */
  stillUrl?: string;
  accent?: string;
  progress?: { percent: number; label: string };
  /** A finished episode: a check mark, with this as its spoken name ("Gesehen"). */
  watchedLabel?: string;
  /** The episode the big button plays: the row is marked, with this hint after the code. */
  highlight?: string;
  focusKey?: string;
}

/**
 * An episode in the list of a season (concept 17 D-1, variant A): still with progress, code,
 * title, two lines of overview, runtime and a check mark; the whole row links to the episode.
 */
export function EpisodeListItem({
  href,
  code,
  title,
  overview,
  caption,
  stillUrl,
  accent,
  progress,
  watchedLabel,
  highlight,
  focusKey,
}: EpisodeListItemProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  const overviewId = useId();
  const kicker = [code, highlight].filter(Boolean);
  return (
    <AppLink
      ref={ref}
      href={href}
      className={highlight ? `${styles.row} ${styles.current}` : styles.row}
      style={itemAccentStyle(accent)}
      aria-describedby={overview ? overviewId : undefined}
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
      <span className={styles.text}>
        {kicker.length > 0 && (
          <span className={styles.code}>
            {kicker.map((part, index) => (
              <span key={part} className={index > 0 ? styles.highlight : undefined}>
                {part}
              </span>
            ))}
          </span>
        )}
        <span className={styles.title}>{title}</span>
        {overview && (
          <span id={overviewId} className={styles.overview} aria-hidden="true">
            {overview}
          </span>
        )}
      </span>
      <span className={styles.side}>
        {caption && <span>{caption}</span>}
        {watchedLabel ? (
          <span className={styles.check} role="img" aria-label={watchedLabel} data-watched="">
            <Icon name="check" size="sm" />
          </span>
        ) : (
          <span className={`${styles.check} ${styles.open}`} aria-hidden="true" />
        )}
      </span>
    </AppLink>
  );
}
