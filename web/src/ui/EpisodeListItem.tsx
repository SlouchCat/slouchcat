import { useEffect, useId, useRef, type MouseEvent, type ReactNode } from 'react';
import { itemAccentStyle } from './accent';
import { SpatialButtonLink } from './Button';
import styles from './EpisodeListItem.module.css';
import { Icon } from './Icon';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';
import { VisuallyHidden } from './VisuallyHidden';

interface EpisodeListItemProps {
  /** The episode's number as shown at the start of the line ("2", "3–4"), translated by the page. */
  number?: string;
  /** Episode code ("S01E02", "S01E03–E04"), translated by the page: spoken and in the open row. */
  code?: string;
  title: string;
  /** The episode's overview: one line in the closed row (from 700 px), more in the open one. */
  overview?: string;
  /** Runtime ("45 min"). */
  caption?: string;
  /** The episode the big button plays: this hint under the title ("Als Nächstes"). */
  highlight?: string;
  /** The row is open: still, longer text and the actions under the line. */
  expanded: boolean;
  /**
   * Called on a click on the line or Enter on it. A pointer toggles the row; the keyboard opens it
   * (`keyboard` true), and the focus then moves to the play button, so a second Enter plays
   * (concept 21 O-11).
   */
  onToggle: (keyboard: boolean) => void;
  /** The player of the episode, behind the open row's first button. */
  playHref: string;
  /** Visible text of that button ("Abspielen", "Weiterschauen"). */
  playLabel: string;
  /** Its spoken name, with the episode ("Abspielen: S01E02 · Die lange Nacht"). */
  playName: string;
  /**
   * Called on a click of the play button before it is followed: a started episode asks first
   * (ResumeChoice) and prevents the default.
   */
  onPlay?: (event: MouseEvent<HTMLAnchorElement>) => void;
  /** The episode's page (content, cast, versions, pen), behind «Details» in the open row. */
  detailsHref: string;
  /** Visible text of that button ("Details"). */
  detailsLabel: string;
  /** Its spoken name, with the episode ("Details: S01E02 · Die lange Nacht"). */
  detailsName: string;
  /** More actions after «Details» in the open row: «Herunterladen» where the device can. */
  actions?: ReactNode;
  /** 16:9 still (backdropSmall of the episode) in the open row, or undefined for a tinted frame. */
  stillUrl?: string;
  accent?: string;
  progress?: { percent: number; label: string };
  /**
   * The control at the far right of the line, outside the line's button: the check that sets and
   * clears «gesehen» (concept 20 P-5, WatchedToggle). Without it an empty ring or `watchedLabel`.
   */
  watchedControl?: ReactNode;
  /** A finished episode without `watchedControl`: a check mark with this spoken name ("Gesehen"). */
  watchedLabel?: string;
  /** Focus key of the line; the play button takes `<focusKey>-play`, «Details» `<focusKey>-details`. */
  focusKey: string;
}

/**
 * An episode in the compact list of a season (concept 21 P-6, variant C): one line over the whole
 * width with number, title (and the hint of the next episode), one line of overview from 700 px,
 * runtime and the check at the far right. The line is a button that opens the row: the still with
 * the progress, the code, a longer text, «Abspielen» or «Weiterschauen», «Details» and the further
 * actions. ↑ and ↓ move between the lines and, in an open row, through its buttons.
 */
export function EpisodeListItem({
  number,
  code,
  title,
  overview,
  caption,
  highlight,
  expanded,
  onToggle,
  playHref,
  playLabel,
  playName,
  onPlay,
  detailsHref,
  detailsLabel,
  detailsName,
  actions,
  stillUrl,
  accent,
  progress,
  watchedControl,
  watchedLabel,
  focusKey,
}: EpisodeListItemProps) {
  const panel = useRef<HTMLDivElement>(null);
  const row = useRef<HTMLDivElement>(null);
  const focusPlayButton = () => panel.current?.querySelector<HTMLElement>('a[href]')?.focus();
  // In an open row ↓ goes from the line to «Abspielen» and ↑ from the row's buttons back to the
  // line, whatever the geometry of the still beside them says (concept 21 O-11).
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({
    focusKey,
    onArrowPress: (direction) => {
      // The panel is in the page only while the row is open.
      if (direction !== 'down' || !panel.current) return true;
      focusPlayButton();
      return false;
    },
  });
  const upToLine = (direction: string) => {
    if (direction !== 'up') return true;
    row.current?.querySelector<HTMLElement>('button[aria-expanded]')?.focus();
    return false;
  };
  const panelId = useId();
  const overviewId = useId();
  // Set by Enter on the line: once the row is open, the focus moves to its play button (its own
  // focus handler tells the spatial navigation).
  const focusPlay = useRef(false);
  useEffect(() => {
    if (!expanded || !focusPlay.current) return;
    focusPlay.current = false;
    focusPlayButton();
  }, [expanded]);
  return (
    <div
      ref={row}
      className={expanded ? `${styles.row} ${styles.open}` : styles.row}
      style={itemAccentStyle(accent)}
    >
      <div className={styles.line}>
        <button
          ref={ref}
          type="button"
          className={styles.toggle}
          aria-expanded={expanded}
          aria-controls={expanded ? panelId : undefined}
          aria-describedby={overview && !expanded ? overviewId : undefined}
          onFocus={onFocus}
          onClick={(event) => {
            // Enter and Space click with detail 0, a pointer with its click count.
            const keyboard = event.detail === 0;
            focusPlay.current = keyboard;
            onToggle(keyboard);
            if (keyboard && expanded) {
              focusPlay.current = false;
              focusPlayButton();
            }
          }}
        >
          <span className={styles.number} aria-hidden="true">
            {number}
          </span>
          <span className={styles.heading}>
            {/* The spaces keep the parts apart in the accessible name; a flex box draws none. */}
            {code && <VisuallyHidden>{code}</VisuallyHidden>}
            {code && ' '}
            <span className={styles.title}>{title}</span>
            {highlight && (
              <>
                {' '}
                <span className={styles.highlight}>{highlight}</span>
              </>
            )}
          </span>
          <span id={overviewId} className={styles.summary} aria-hidden="true">
            {overview}
          </span>
          {caption && ' '}
          <span className={styles.caption}>{caption}</span>
        </button>
        <span className={styles.mark}>
          {watchedControl ??
            (watchedLabel ? (
              <span className={styles.check} role="img" aria-label={watchedLabel} data-watched="">
                <Icon name="check" size="sm" />
              </span>
            ) : (
              <span className={`${styles.check} ${styles.empty}`} aria-hidden="true" />
            ))}
        </span>
      </div>
      {expanded && (
        <div ref={panel} id={panelId} className={styles.panel}>
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
          <div className={styles.body}>
            {code && <span className={styles.code}>{code}</span>}
            {overview && <p className={styles.overview}>{overview}</p>}
            <div className={styles.actions}>
              <SpatialButtonLink
                focusKey={`${focusKey}-play`}
                onArrowPress={upToLine}
                href={playHref}
                variant="primary"
                icon="play"
                aria-label={playName}
                onClick={onPlay}
              >
                {playLabel}
              </SpatialButtonLink>
              <SpatialButtonLink
                focusKey={`${focusKey}-details`}
                onArrowPress={upToLine}
                href={detailsHref}
                variant="secondary"
                icon="info"
                aria-label={detailsName}
              >
                {detailsLabel}
              </SpatialButtonLink>
              {actions}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
