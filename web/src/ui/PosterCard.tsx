import { AppLink } from './AppLink';
import { itemAccentStyle } from './accent';
import { Chip } from './Chip';
import styles from './PosterCard.module.css';
import { PosterFallback } from './PosterFallback';
import { ProgressBar } from './ProgressBar';
import { useSpatialItem } from './spatial';

// Rendered width of a card per breakpoint (Row.module.css, size.poster.* tokens); lets the browser
// pick the w200 or w400 poster. w800 is left out on purpose: a phone at 3x would take it, and the
// start page must not get heavier (P-10).
const CARD_SIZES = '(width >= 1440px) 12.5rem, (width >= 768px) 10.5rem, 8.5rem';

interface PosterCardProps {
  href: string;
  title: string;
  /** Secondary line under the title (year, remaining time). */
  caption?: string;
  /** Same-origin poster URL (w400), or undefined for the lettered fallback tile. */
  posterUrl?: string;
  /** The same poster in w200, offered to the browser through `srcset`. */
  posterSmallUrl?: string;
  /** The title's accent colour (`#rrggbb`, ItemRef.accent): placeholder tint and focus glow. */
  accent?: string;
  /** Small label on the poster (the kind of an extra: "Trailer", "Featurette"). */
  badge?: string;
  /** Watch progress 0 … 100 with its accessible label. */
  progress?: { percent: number; label: string };
  /** Load the image eagerly (the first cards of the first row); every other poster is lazy. */
  eager?: boolean;
  /** Keep the image out of the document for now (the hero artwork goes first); the frame stays. */
  deferImage?: boolean;
  focusKey?: string;
}

/**
 * 2:3 poster with its title below, one link to the title. The link carries the focus ring and
 * scales on focus (focus.scale token); a soft glow in the title's accent colour lights up behind
 * the poster, and the same colour tints the frame while the image is still on its way.
 */
export function PosterCard({
  href,
  title,
  caption,
  posterUrl,
  posterSmallUrl,
  accent,
  badge,
  progress,
  eager = false,
  deferImage = false,
  focusKey,
}: PosterCardProps) {
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
      <span className={styles.frame}>
        {deferImage ? null : source ? (
          <img
            className={styles.image}
            src={source}
            srcSet={
              posterUrl && posterSmallUrl ? `${posterSmallUrl} 200w, ${posterUrl} 400w` : undefined
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
      <span className={styles.meta}>
        <span className={styles.title}>{title}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
      </span>
    </AppLink>
  );
}
