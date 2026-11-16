import styles from './Logo.module.css';

/**
 * The Slouchcat mark with the word mark next to it; `name` comes from the locale files. The
 * administration adds a small line below the name (`tagline`, «Control Center»).
 */
export function Logo({ name, tagline }: { name: string; tagline?: string }) {
  return (
    <span className={styles.logo}>
      <svg className={styles.mark} viewBox="0 0 512 512" aria-hidden="true" focusable="false">
        <path
          className={styles.head}
          d="M136 400V136l80 72h80l80-72v264a24 24 0 0 1-24 24H160a24 24 0 0 1-24-24z"
        />
        <path className={styles.play} d="M228 268v96l84-48z" />
      </svg>
      {tagline ? (
        <span className={styles.words}>
          <span className={styles.name}>{name}</span>
          <span className={styles.tagline}>{tagline}</span>
        </span>
      ) : (
        <span className={styles.name}>{name}</span>
      )}
    </span>
  );
}
