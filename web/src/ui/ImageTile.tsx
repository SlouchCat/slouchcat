import { useState, type ReactNode } from 'react';
import styles from './ImageTile.module.css';
import { PosterFallback } from './PosterFallback';

interface ImageTileProps {
  /** Address of the image; without it, or when it fails to load, the fallback shows. */
  src?: string;
  /** The title the image belongs to: written on the fallback surface. */
  title: string;
  /** `landscape` 16:9 for backdrops (streams, top titles), `portrait` 2:3 for posters. */
  shape?: 'landscape' | 'portrait';
  /** Content laid over the bottom of the image (mode pill, progress). */
  children?: ReactNode;
}

/**
 * An image of a title (concept 12, 11.2: streams, statistics, collections), across or upright.
 * The image is decoration, the page names the title in text; without an image, or when it fails
 * to load, PosterFallback shows with the title written on it.
 */
export function ImageTile({ src, title, shape = 'landscape', children }: ImageTileProps) {
  const [failed, setFailed] = useState<string | null>(null);
  const showImage = src !== undefined && failed !== src;
  return (
    <div className={`${styles.tile} ${styles[shape]}`}>
      {showImage ? (
        <img
          className={styles.image}
          src={src}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed(src)}
        />
      ) : (
        <div className={styles.fallback}>
          <PosterFallback />
          <span className={styles.title} aria-hidden="true">
            {title}
          </span>
        </div>
      )}
      {children && <div className={styles.overlay}>{children}</div>}
    </div>
  );
}
