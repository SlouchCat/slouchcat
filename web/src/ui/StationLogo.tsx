import { accentNames } from '@slouchcat/design-tokens';
import { useState } from 'react';
import { swatchStyle } from './accent';
import styles from './StationLogo.module.css';

// Every station has a picture (concept 16, section 6): its logo, or its initials on one of the
// eight accent colours. The colour follows from the name, so a station keeps it everywhere.

/** Words that say nothing about a station and would make most initials an «R». */
const FILLER_WORDS = new Set(['radio', 'webradio', 'fm', 'the']);

/**
 * Up to three characters for a station without a logo: the first letters of its first words, a
 * number in full («Energy 105» → «E105» is cut to «E10»), an abbreviation in capitals as it is
 * («SRF» → «SRF»), a single word by its first two letters.
 */
export function stationInitials(name: string): string {
  const all = name.split(/[\s\-_/·|]+/).filter(Boolean);
  const words = all.filter((word) => !FILLER_WORDS.has(word.toLowerCase()));
  const chosen = words.length > 0 ? words : all;
  const [first] = chosen;
  if (!first) return '?';
  if (chosen.length === 1) {
    if (/^[\p{Lu}\p{N}]{2,}$/u.test(first)) return first.slice(0, 3);
    const letters = [...first];
    return (letters[0] ?? '').toUpperCase() + (letters[1] ?? '').toLowerCase();
  }
  return chosen
    .slice(0, 3)
    .map((word) => (/^\p{N}+$/u.test(word) ? word : ([...word][0] ?? '')))
    .join('')
    .toUpperCase()
    .slice(0, 3);
}

/** One of the eight accent colours for a name (a stable string hash). */
export function stationAccent(name: string): string {
  let hash = 5381;
  for (const char of name) hash = (hash * 33 + (char.codePointAt(0) ?? 0)) >>> 0;
  return accentNames[hash % accentNames.length] ?? 'ember';
}

interface StationLogoProps {
  name: string;
  /** URL of the logo; without one (or when it fails to load) the initials show. */
  src?: string;
  /**
   * The picture when `src` fails to load: a title's cover gives way to the station's logo
   * (concept 16, 8 point 6), the logo to the initials.
   */
  fallbackSrc?: string;
  /** `sm` 2.5 rem (lists, the mini player), `md` 4 rem, `lg` 8 rem, `fill` the container's width. */
  size?: 'sm' | 'md' | 'lg' | 'fill';
}

/** Square station picture; decorative, the surrounding control names the station. */
export function StationLogo({ name, src, fallbackSrc, size = 'md' }: StationLogoProps) {
  const [failed, setFailed] = useState<readonly string[]>([]);
  const image = [src, fallbackSrc].find((url) => url !== undefined && !failed.includes(url));
  return (
    <span
      className={`${styles.logo} ${styles[size]}`}
      style={image ? undefined : swatchStyle(stationAccent(name))}
      data-initials={image ? undefined : ''}
      aria-hidden="true"
    >
      {image ? (
        <img
          className={styles.image}
          src={image}
          alt=""
          loading="lazy"
          decoding="async"
          onError={() => setFailed((before) => [...before, image])}
        />
      ) : (
        <span className={styles.initials}>{stationInitials(name)}</span>
      )}
    </span>
  );
}
