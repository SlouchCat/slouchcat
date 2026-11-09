import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import styles from './LibraryTile.module.css';
import { useSpatialItem } from './spatial';

// The posters of the collage come in the srcset of a poster card (CARD_SIZES in PosterCard.tsx):
// a slice is as tall as the tile, so the cover it shows is about as wide as a card. The server's
// prepaint of «Meine Medien» copies these attributes (tilePrepaint in api/preload.go).
const CARD_SIZES = '(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, calc(33vw - 1.5rem)';

/** One poster of the collage. */
export interface LibraryTileCover {
  /** Poster (w400), or undefined for an empty slice. */
  posterUrl?: string;
  /** The same poster in w200. */
  posterSmallUrl?: string;
}

interface LibraryTileProps {
  href: string;
  /** Name of the library. */
  name: string;
  /** The number of its titles ("1234 Titel"). */
  caption: string;
  /** The newest titles, at most four, side by side. */
  covers: readonly LibraryTileCover[];
  /** Accent of the newest title: glow and the tint of the empty slices. */
  accent?: string;
  /** Load the images eagerly (the first tiles of the first row). */
  eager?: boolean;
  /** Fetch the first image with high priority (the page's first image, preloaded by the server). */
  priority?: boolean;
  /** Called once the first image is on screen (loaded and painted) or has failed. */
  onImageShown?: () => void;
  /** Keep the images out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  focusKey?: string;
}

/**
 * A library on the start page (concept 15 P-7, «Meine Medien»): a collage of the posters of its
 * newest titles, two cards wide, with the name (26 px, concept 18 C-2) and the number of titles
 * on the darkened collage. One link to the library page; focus and hover behave like a PosterCard.
 */
export function LibraryTile({
  href,
  name,
  caption,
  covers,
  accent,
  eager = false,
  priority = false,
  onImageShown,
  deferImage = false,
  focusKey,
}: LibraryTileProps) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey });
  return (
    <AppLink
      ref={ref}
      href={href}
      className={styles.tile}
      style={itemAccentStyle(accent)}
      onFocus={onFocus}
    >
      <span className={styles.collage} aria-hidden="true">
        {covers.map((cover, index) => {
          const source = cover.posterUrl ?? cover.posterSmallUrl;
          const first = index === 0;
          return deferImage || !source ? (
            <span key={index} className={styles.slice} />
          ) : (
            <img
              key={index}
              className={styles.slice}
              src={source}
              srcSet={
                cover.posterUrl && cover.posterSmallUrl
                  ? `${cover.posterSmallUrl} 200w, ${cover.posterUrl} 400w`
                  : undefined
              }
              sizes={cover.posterUrl && cover.posterSmallUrl ? CARD_SIZES : undefined}
              alt=""
              width={400}
              height={600}
              loading={eager ? 'eager' : 'lazy'}
              fetchPriority={priority && first ? 'high' : undefined}
              decoding="async"
              draggable={false}
              onLoad={
                first && onImageShown
                  ? () => requestAnimationFrame(() => requestAnimationFrame(onImageShown))
                  : undefined
              }
              onError={first ? onImageShown : undefined}
            />
          );
        })}
      </span>
      <span className={styles.meta}>
        <span className={styles.name}>{name}</span>
        <span className={styles.caption}>{caption}</span>
      </span>
    </AppLink>
  );
}
