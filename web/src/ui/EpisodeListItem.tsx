import { useId, type MouseEvent, type ReactNode } from 'react';
import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import { SpatialButtonLink } from './Button';
import styles from './EpisodeListItem.module.css';
import { Icon } from './Icon';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';
import { VisuallyHidden } from './VisuallyHidden';

interface EpisodeListItemProps {
  /** The player of the episode: still and text are one link that starts it (concept 20 P-4). */
  playHref: string;
  /** Spoken before the row's text, so the link says what it does ("Abspielen"). */
  playLabel: string;
  /**
   * Called on a click of the play link before it is followed: a started episode asks first
   * (ResumeChoice) and prevents the default.
   */
  onPlay?: (event: MouseEvent<HTMLAnchorElement>) => void;
  /** The episode's page (content, cast, versions, pen), behind the button «Details». */
  detailsHref: string;
  /** Visible text of that button ("Details"). */
  detailsLabel: string;
  /** Its spoken name, with the episode ("Details: S01E02 · Die lange Nacht"). */
  detailsName: string;
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
  /**
   * The control in place of the check mark at the far right, outside the play link: the button
   * that sets and clears «gesehen» (concept 20 P-5). Without it the check is a display only.
   */
  watchedControl?: ReactNode;
  /** The episode the big button plays: the row is marked, with this hint after the code. */
  highlight?: string;
  focusKey?: string;
}

/**
 * An episode in the list of a season (concept 17 D-1, variant A; concept 20 P-4): still with a
 * round play button and the progress, code, title, two lines of overview and the runtime form one
 * link that starts the episode; «Details» beside it opens the episode's page; the check mark
 * stands at the far right, outside both. ← and → move between the link and «Details», ↑ and ↓
 * between the episodes.
 */
export function EpisodeListItem({
  playHref,
  playLabel,
  onPlay,
  detailsHref,
  detailsLabel,
  detailsName,
  code,
  title,
  overview,
  caption,
  stillUrl,
  accent,
  progress,
  watchedLabel,
  watchedControl,
  highlight,
  focusKey,
}: EpisodeListItemProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  const overviewId = useId();
  const kicker = [code, highlight].filter(Boolean);
  return (
    <div
      className={highlight ? `${styles.row} ${styles.current}` : styles.row}
      style={itemAccentStyle(accent)}
    >
      <AppLink
        ref={ref}
        href={playHref}
        className={styles.link}
        aria-describedby={overview ? overviewId : undefined}
        onFocus={onFocus}
        onClick={onPlay}
      >
        <VisuallyHidden>{playLabel}</VisuallyHidden>
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
          <span className={styles.play} aria-hidden="true">
            <Icon name="play" size="md" />
          </span>
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
        {caption && <span className={styles.caption}>{caption}</span>}
      </AppLink>
      <SpatialButtonLink
        focusKey={focusKey ? `${focusKey}-details` : undefined}
        href={detailsHref}
        variant="secondary"
        icon="info"
        aria-label={detailsName}
      >
        {detailsLabel}
      </SpatialButtonLink>
      <span className={styles.mark}>
        {watchedControl ??
          (watchedLabel ? (
            <span className={styles.check} role="img" aria-label={watchedLabel} data-watched="">
              <Icon name="check" size="sm" />
            </span>
          ) : (
            <span className={`${styles.check} ${styles.open}`} aria-hidden="true" />
          ))}
      </span>
    </div>
  );
}
