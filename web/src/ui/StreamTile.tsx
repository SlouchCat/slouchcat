import type { ReactNode } from 'react';
import { Chip } from './Chip';
import { Icon, type IconName } from './Icon';
import { ImageTile } from './ImageTile';
import { StreamProgress } from './StreamProgress';
import styles from './StreamTile.module.css';
import { VisuallyHidden } from './VisuallyHidden';

/** A line of the tile with an icon; the label is for screen readers ("Gerät"). */
export interface StreamFact {
  key: string;
  label: string;
  value: string;
  icon: IconName;
}

interface StreamTileProps {
  /** Accessible name of the tile, whose and what ("Marco: Oppenheimer"). */
  label: string;
  title: string;
  /** The backdrop across (for an episode the series'), or undefined for the fallback surface. */
  backdropUrl?: string;
  /** How it plays ("Transcode (HW)"), a pill over the image. */
  mode: string;
  /** What the mode costs the server: Direct success, Remux neutral, HW warning, SW danger. */
  modeTone: 'neutral' | 'success' | 'warning' | 'danger';
  profile: string;
  /** The account the profile belongs to (its e-mail). */
  account: string;
  /** Device and quality, each with its label for screen readers. */
  facts: readonly StreamFact[];
  /** 0 … 100; without it no bar (the length is unknown). */
  progress?: number;
  /** Accessible name of the progress bar ("Fortschritt Oppenheimer"). */
  progressLabel: string;
  /** The position as text ("1:05 von 10:00"). */
  position: string;
  /** Since when it plays, formatted by the page (a `<time>`). */
  since: ReactNode;
  /** The tile's action («Beenden»). */
  action: ReactNode;
}

/**
 * A running stream on the streams page (concept 12, 11.3, mockup Str-D2): the backdrop with the
 * mode pill, the title, profile and account, device and quality, the progress, since when and the
 * action. Nothing is clipped, every text stays readable without a tooltip. The progress is the
 * part it shares with the overview's StreamRow.
 */
export function StreamTile({
  label,
  title,
  backdropUrl,
  mode,
  modeTone,
  profile,
  account,
  facts,
  progress,
  progressLabel,
  position,
  since,
  action,
}: StreamTileProps) {
  return (
    <article className={styles.tile} aria-label={label}>
      <ImageTile src={backdropUrl} title={title}>
        <Chip tone={modeTone}>{mode}</Chip>
      </ImageTile>
      <h3 className={styles.title}>{title}</h3>
      <p className={styles.who}>
        <span className={styles.profile}>{profile}</span>
        <span className={styles.muted}>{account}</span>
      </p>
      <dl className={styles.facts}>
        {facts.map((fact) => (
          <div key={fact.key} className={styles.fact}>
            <dt className={styles.icon}>
              <Icon name={fact.icon} size="sm" />
              <VisuallyHidden>{fact.label}</VisuallyHidden>
            </dt>
            <dd className={styles.value}>{fact.value}</dd>
          </div>
        ))}
      </dl>
      <StreamProgress value={progress} label={progressLabel} text={position} />
      <div className={styles.footer}>
        <span className={styles.muted}>{since}</span>
        {action}
      </div>
    </article>
  );
}
