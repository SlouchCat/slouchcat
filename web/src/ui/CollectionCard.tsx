import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import styles from './CollectionCard.module.css';
import { PosterFallback } from './PosterFallback';
import { useSpatialItem } from './spatial';

const CARD_SIZES = '(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, 8.5rem';

interface CollectionCardProps {
  href: string;
  /** Name of the collection. */
  name: string;
  /** Secondary line ("3 Titel"). */
  caption?: string;
  /** Poster of the cover title (w400), or undefined for the lettered fallback tile. */
  posterUrl?: string;
  /** The same poster in w200. */
  posterSmallUrl?: string;
  /** Accent of the cover title: glow and placeholder tint. */
  accent?: string;
  eager?: boolean;
  /** Keep the image out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  focusKey?: string;
}

/**
 * A collection (E-6): the cover title's poster on a small stack of sheets, so it reads as more
 * than one title, with the name and the count below. One link to the collection page; focus and
 * hover behave like a PosterCard.
 */
export function CollectionCard({
  href,
  name,
  caption,
  posterUrl,
  posterSmallUrl,
  accent,
  eager = false,
  deferImage = false,
  focusKey,
}: CollectionCardProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  const source = posterUrl ?? posterSmallUrl;
  return (
    <AppLink
      ref={ref}
      href={href}
      className={styles.card}
      style={itemAccentStyle(accent)}
      onFocus={onFocus}
    >
      <span className={styles.stack}>
        <span className={styles.frame}>
          {deferImage ? null : source ? (
            <img
              className={styles.image}
              src={source}
              srcSet={
                posterUrl && posterSmallUrl
                  ? `${posterSmallUrl} 200w, ${posterUrl} 400w`
                  : undefined
              }
              sizes={posterUrl && posterSmallUrl ? CARD_SIZES : undefined}
              alt=""
              width={400}
              height={600}
              loading={eager ? 'eager' : 'lazy'}
              decoding="async"
              draggable={false}
            />
          ) : (
            <PosterFallback />
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
