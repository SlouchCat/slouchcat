import { forwardRef, type ReactNode } from 'react';
import styles from './DisclosureTile.module.css';
import { Icon } from './Icon';

interface DisclosureTileProps {
  id?: string;
  /** The name, big and bold; also the start of the button's accessible name. */
  title: string;
  /** The secondary line below the name ("51 Länder · 26.395 Sender"). */
  subtitle?: ReactNode;
  /** A small badge at the end ("1 gewählt"); left out when there is nothing to say. */
  badge?: ReactNode;
  isExpanded: boolean;
  /** The id of the region the tile opens; set it only while that region is in the page. */
  controls?: string;
  onToggle: () => void;
}

/**
 * A tile that opens a region below it (concept 20, P-15): a `<button>` with `aria-expanded` and
 * `aria-controls`, left aligned, its height growing with two lines of content, the name over a
 * secondary line, a badge and a chevron at the end that turns while the tile is open. An open tile
 * carries the accent as its border on a tinted ground. For a button with one line of text use
 * `AriaButton`; a pill does not take two lines.
 */
export const DisclosureTile = forwardRef<HTMLButtonElement, DisclosureTileProps>(
  function DisclosureTile({ id, title, subtitle, badge, isExpanded, controls, onToggle }, ref) {
    return (
      <button
        ref={ref}
        id={id}
        type="button"
        className={styles.tile}
        aria-expanded={isExpanded}
        aria-controls={controls}
        onClick={onToggle}
      >
        {/* The spaces keep the parts apart in the accessible name; a flex box draws none. */}
        <span className={styles.text}>
          <span className={styles.title}>{title}</span>
          {subtitle !== undefined && ' '}
          {subtitle !== undefined && <span className={styles.subtitle}>{subtitle}</span>}
        </span>
        {badge !== undefined && ' '}
        {badge !== undefined && <span className={styles.badge}>{badge}</span>}
        <span className={styles.chevron} aria-hidden="true">
          <Icon name="chevronDown" size="sm" />
        </span>
      </button>
    );
  },
);
