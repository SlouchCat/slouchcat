import { useState } from 'react';
import styles from './TitleOrLogo.module.css';

interface TitleOrLogoProps {
  title: string;
  /** The title logo (ADR 0018); without one, or when it fails to load, the text title shows. */
  logoUrl?: string;
  /** Extra class on the logo image (width limits of the surrounding stage or heading). */
  className?: string;
}

/**
 * The text title, or the title logo in its place (concept 10, 7.5): the heading element stays the
 * caller's (`<h1>`, `<h2>`), screen readers read the title through `alt`, and a logo that does not
 * load (offline: logos are not in the offline package) falls back to the text.
 */
export function TitleOrLogo({ title, logoUrl, className }: TitleOrLogoProps) {
  const [failed, setFailed] = useState(false);
  if (!logoUrl || failed) return <>{title}</>;
  return (
    <img
      className={`${styles.logo} ${className ?? ''}`}
      src={logoUrl}
      alt={title}
      decoding="async"
      onError={() => setFailed(true)}
    />
  );
}
