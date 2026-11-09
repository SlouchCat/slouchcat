import { useState, type ReactNode } from 'react';
import { Chip } from './Chip';
import { Icon, type IconName } from './Icon';
import { ImageTile } from './ImageTile';
import { StationLogo } from './StationLogo';
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

/** What a tile plays; it colours the badge on the image (concept 18, 17.3). */
export type StreamKind = 'movie' | 'episode' | 'extra' | 'audiobook' | 'podcast' | 'radio';

/**
 * The picture of a tile: `backdrop` a backdrop across (film, episode, extra), `cover` a square
 * cover and `logo` a station logo, both in the middle on a blurred copy of themselves. Without a
 * `src` a backdrop shows the fallback surface, a cover or logo the initials of the title.
 */
export interface StreamImage {
  form: 'backdrop' | 'cover' | 'logo';
  src?: string;
}

interface StreamTileProps {
  /** Accessible name of the tile, whose and what ("Marco: Oppenheimer"). */
  label: string;
  title: string;
  /** What plays, as a badge on the image («Film», «Hörbuch», «Radio»). */
  kind: StreamKind;
  kindLabel: string;
  image: StreamImage;
  /** How it plays ("Transcode (HW)"), a pill over the image; none for the radio. */
  mode?: string;
  /** What the mode costs the server: Direct success, Remux neutral, HW warning, SW danger. */
  modeTone?: 'neutral' | 'success' | 'warning' | 'danger';
  profile: string;
  /** The account the profile belongs to (its e-mail). */
  account: string;
  /** Device, quality or format and the like, each with its label for screen readers. */
  facts: readonly StreamFact[];
  /** 0 … 100; without it no bar (the length is unknown, or the radio). */
  progress?: number;
  /** Accessible name of the progress bar ("Fortschritt Oppenheimer"). */
  progressLabel: string;
  /** The position as text ("1:05 von 10:00"), for the radio its state («Hört»). */
  position: string;
  /** Since when it plays, formatted by the page (a `<time>`). */
  since: ReactNode;
  /** The tile's action («Beenden»). */
  action: ReactNode;
}

/** A square cover or logo in the middle of the tile's image on a blurred, darkened copy. */
function SquareImage({ title, image }: { title: string; image: StreamImage }) {
  const [failed, setFailed] = useState<string | null>(null);
  const ground = image.src !== undefined && failed !== image.src ? image.src : undefined;
  return (
    <div className={styles.square}>
      {ground && (
        <img
          className={styles.ground}
          src={ground}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed(ground)}
        />
      )}
      <span className={styles.cover}>
        <StationLogo name={title} src={image.src} size="fill" />
      </span>
    </div>
  );
}

/**
 * A running stream on the streams page (concept 12, 11.3, mockup Str-D2; concept 18, 17.3): the
 * image with the kind badge and the mode pill, the title, profile and account, the facts, the
 * progress, since when and the action. Video shows its backdrop across, an audiobook or podcast
 * its square cover and the radio its station logo, both on a blurred ground. Nothing is clipped,
 * every text stays readable without a tooltip. The progress is the part it shares with the
 * overview's StreamRow.
 */
export function StreamTile({
  label,
  title,
  kind,
  kindLabel,
  image,
  mode,
  modeTone = 'neutral',
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
      <div className={styles.media}>
        {image.form === 'backdrop' ? (
          <ImageTile src={image.src} title={title} />
        ) : (
          <SquareImage title={title} image={image} />
        )}
        <span className={`${styles.kind} ${styles[kind]}`}>{kindLabel}</span>
        {mode && (
          <span className={styles.mode}>
            <Chip tone={modeTone}>{mode}</Chip>
          </span>
        )}
      </div>
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
