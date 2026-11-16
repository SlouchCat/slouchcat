import { useId, type ComponentProps } from 'react';
import { StatTile } from './StatTile';
import { VisuallyHidden } from './VisuallyHidden';
import styles from './StatusStrip.module.css';

type Tile = Omit<ComponentProps<typeof StatTile>, 'compact'> & { key: string };

interface StatusStripProps {
  /** Spoken name of the group ("Zusammenfassung"). */
  label: string;
  tiles: readonly Tile[];
  /**
   * The label as a heading of level 2 for screen readers only, naming the group
   * (`aria-labelledby`): for a strip that stood under a heading of its own before (concept 12, S-4),
   * so the heading navigation still finds it. The look stays the same.
   */
  heading?: boolean;
}

/**
 * The counters of a page right below its header (concept 12, S-4): a row of compact StatTiles
 * (56 px), equally wide, wrapping on narrow screens.
 */
export function StatusStrip({ label, tiles, heading = false }: StatusStripProps) {
  const headingId = useId();
  return (
    <div
      role="group"
      aria-label={heading ? undefined : label}
      aria-labelledby={heading ? headingId : undefined}
      className={styles.strip}
    >
      {heading && (
        <VisuallyHidden as="h2" id={headingId}>
          {label}
        </VisuallyHidden>
      )}
      {tiles.map(({ key, ...tile }) => (
        <StatTile key={key} compact {...tile} />
      ))}
    </div>
  );
}
