import type { CSSProperties } from 'react';
import styles from './Logo.module.css';
import {
  LOGO_HEAD,
  LOGO_PLAY,
  LOGO_SIGN_HEIGHT,
  LOGO_SIGN_VIEW_BOX,
  LOGO_SIGN_WIDTH,
} from './logoMark';

/**
 * The Slouchcat mark with the word mark next to it; `name` comes from the locale files. The
 * administration adds a small line below the name (`tagline`, «Control Center»).
 */
export function Logo({ name, tagline }: { name: string; tagline?: string }) {
  return (
    <span className={styles.logo}>
      <svg
        className={styles.mark}
        viewBox={LOGO_SIGN_VIEW_BOX}
        style={{ '--logo-sign-ratio': LOGO_SIGN_WIDTH / LOGO_SIGN_HEIGHT } as CSSProperties}
        aria-hidden="true"
        focusable="false"
      >
        <path className={styles.head} d={LOGO_HEAD} />
        <path className={styles.play} d={LOGO_PLAY} />
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
