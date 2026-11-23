import styles from './PosterFallback.module.css';
import { LOGO_HEAD, LOGO_VIEW_BOX } from './logoMark';

/** Neutral 2:3 tile with the Slouchcat mark, shown while a title has no poster (no TMDB match). */
export function PosterFallback() {
  return (
    <span className={styles.fallback} aria-hidden="true">
      <svg className={styles.mark} viewBox={LOGO_VIEW_BOX} focusable="false">
        <path d={LOGO_HEAD} />
      </svg>
    </span>
  );
}
