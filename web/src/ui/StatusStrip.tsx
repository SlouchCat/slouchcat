import type { ComponentProps } from 'react';
import { StatTile } from './StatTile';
import styles from './StatusStrip.module.css';

type Tile = Omit<ComponentProps<typeof StatTile>, 'compact'> & { key: string };

interface StatusStripProps {
  /** Spoken name of the group ("Zusammenfassung"). */
  label: string;
  tiles: readonly Tile[];
}

/**
 * The counters of a page right below its header (concept 12, S-4): a row of compact StatTiles
 * (56 px), equally wide, wrapping on narrow screens.
 */
export function StatusStrip({ label, tiles }: StatusStripProps) {
  return (
    <div role="group" aria-label={label} className={styles.strip}>
      {tiles.map(({ key, ...tile }) => (
        <StatTile key={key} compact {...tile} />
      ))}
    </div>
  );
}
