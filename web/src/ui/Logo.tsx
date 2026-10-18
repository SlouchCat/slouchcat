import styles from './Logo.module.css';

/** The Slouchcat mark with the word mark next to it; `name` comes from the locale files. */
export function Logo({ name }: { name: string }) {
  return (
    <span className={styles.logo}>
      <svg className={styles.mark} viewBox="0 0 512 512" aria-hidden="true" focusable="false">
        <path
          className={styles.head}
          d="M136 400V136l80 72h80l80-72v264a24 24 0 0 1-24 24H160a24 24 0 0 1-24-24z"
        />
        <path className={styles.play} d="M228 268v96l84-48z" />
      </svg>
      <span className={styles.name}>{name}</span>
    </span>
  );
}
