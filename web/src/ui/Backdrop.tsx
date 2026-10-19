import { itemAccentStyle } from './accent';
import styles from './Backdrop.module.css';

interface BackdropProps {
  /** Same-origin backdrop URL (w1280). */
  url?: string;
  /** The same backdrop in w780 and w1920; the browser picks through `srcset`. */
  smallUrl?: string;
  largeUrl?: string;
  /** Poster URL, shown blurred when the title has no backdrop. */
  posterUrl?: string;
  /** The title's accent colour (`#rrggbb`, ItemRef.accent): tints the artwork under the scrim. */
  accent?: string;
}

/** Candidate list for the backdrop `srcset`, smallest first. */
function backdropSources({ url, smallUrl, largeUrl }: BackdropProps) {
  const candidates = [
    smallUrl && `${smallUrl} 780w`,
    url && `${url} 1280w`,
    largeUrl && `${largeUrl} 1920w`,
  ].filter(Boolean);
  return {
    src: url ?? smallUrl ?? largeUrl,
    srcSet: candidates.length > 1 ? candidates.join(', ') : undefined,
    sizes: candidates.length > 1 ? '100vw' : undefined,
  };
}

/**
 * Full-bleed artwork behind the top of a detail page: the backdrop (or the blurred poster), a scrim
 * that fades into the page and darkens the text side, and over both a tint in the title's accent.
 * Decorative; it sits behind the page content (z-index -1 in an isolated parent).
 */
export function Backdrop(props: BackdropProps) {
  const { posterUrl, accent } = props;
  const backdrop = backdropSources(props);
  const src = backdrop.src ?? posterUrl;
  return (
    <div className={styles.backdrop} style={itemAccentStyle(accent)} aria-hidden="true">
      {src && (
        <img
          className={`${styles.image} ${backdrop.src ? '' : styles.blurred}`}
          src={src}
          srcSet={backdrop.src ? backdrop.srcSet : undefined}
          sizes={backdrop.src ? backdrop.sizes : undefined}
          alt=""
          width={1280}
          height={720}
          fetchPriority="high"
        />
      )}
      <div className={styles.scrim} />
      <div className={styles.tint} />
    </div>
  );
}
