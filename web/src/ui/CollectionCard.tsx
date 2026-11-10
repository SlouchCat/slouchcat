import { useState } from 'react';
import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import { Chip } from './Chip';
import styles from './CollectionCard.module.css';
import { PosterFallback } from './PosterFallback';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

const CARD_SIZES = '(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, calc(33vw - 1.5rem)';

interface CollectionCardProps {
  href: string;
  /** Name of the collection. */
  name: string;
  /** Secondary line ("3 Titel"). */
  caption?: string;
  /**
   * The poster of the collection itself (TMDB, w400; only for a collection whose members the
   * profile may all see). It comes before the cover's poster; failing to load, that poster shows.
   */
  imageUrl?: string;
  /** Poster of the cover title (w400), or undefined for the lettered fallback tile. */
  posterUrl?: string;
  /** The same poster in w200. */
  posterSmallUrl?: string;
  /** Accent of the cover title: glow and placeholder tint. */
  accent?: string;
  eager?: boolean;
  /** Keep the image out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  /**
   * `square`: an audiobook series or collection (concept 19 D4.2), cover and sheets in square
   * frames; else the 2:3 frame of a film collection.
   */
  shape?: 'poster' | 'square';
  /** Small label over the cover's top left ("6 Bände", "2 von 6 Bänden"). */
  badge?: string;
  /** Share of the members the profile has finished, 0 … 100, with its accessible label. */
  progress?: { percent: number; label: string };
  focusKey?: string;
}

/**
 * A collection (E-6): the collection's own poster, else the cover title's, on a small stack of
 * sheets, so it reads as more than one title, with the name and the count below. Square for an
 * audiobook series or collection (concept 19 D4.2), with the number of volumes as a badge and a
 * bar of the volumes heard. A poster that
 * does not load gives way to the next one, last to the neutral tile. One link to the collection
 * page; focus and hover behave like a PosterCard.
 */
export function CollectionCard({
  href,
  name,
  caption,
  imageUrl,
  posterUrl,
  posterSmallUrl,
  accent,
  eager = false,
  deferImage = false,
  shape = 'poster',
  badge,
  progress,
  focusKey,
}: CollectionCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  // The sources that failed to load, so the next one in line takes over.
  const [failed, setFailed] = useState<readonly string[]>([]);
  const own = imageUrl && !failed.includes(imageUrl) ? imageUrl : undefined;
  const poster = posterUrl ?? posterSmallUrl;
  const source = own ?? (poster && !failed.includes(poster) ? poster : undefined);
  const responsive = !own && posterUrl && posterSmallUrl && source === poster;
  return (
    <AppLink
      ref={ref}
      href={href}
      className={shape === 'square' ? `${styles.card} ${styles.square}` : styles.card}
      style={itemAccentStyle(accent)}
      onFocus={onFocus}
    >
      <span className={styles.stack}>
        <span className={styles.frame}>
          {deferImage ? null : source ? (
            <img
              className={styles.image}
              src={source}
              srcSet={responsive ? `${posterSmallUrl} 200w, ${posterUrl} 400w` : undefined}
              sizes={responsive ? CARD_SIZES : undefined}
              alt=""
              width={400}
              height={shape === 'square' ? 400 : 600}
              loading={eager ? 'eager' : 'lazy'}
              decoding="async"
              draggable={false}
              onError={() => setFailed((previous) => [...previous, source])}
            />
          ) : (
            <PosterFallback />
          )}
          {badge && (
            <span className={styles.badge}>
              <Chip>{badge}</Chip>
            </span>
          )}
          {progress && (
            <span className={styles.progress}>
              <ProgressBar value={progress.percent} label={progress.label} />
            </span>
          )}
        </span>
      </span>
      <span className={styles.meta}>
        <span className={styles.title}>{name}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
      </span>
    </AppLink>
  );
}
