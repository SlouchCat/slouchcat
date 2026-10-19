import styles from './PosterFallback.module.css';

/** Neutral 2:3 tile with the Slouchcat mark, shown while a title has no poster (no TMDB match). */
export function PosterFallback() {
  return (
    <span className={styles.fallback} aria-hidden="true">
      <svg className={styles.mark} viewBox="0 0 512 512" focusable="false">
        <path d="M136 400V136l80 72h80l80-72v264a24 24 0 0 1-24 24H160a24 24 0 0 1-24-24z" />
      </svg>
    </span>
  );
}
