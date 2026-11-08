import { Icon } from './Icon';
import { useSpatialItem } from './spatial';
import { StationLogo } from './StationLogo';
import styles from './StationTile.module.css';

// A station in the radio page's rows and grid (concept 16, 10.1): its picture, name and place,
// the heart of «Meine Sender» over the picture. A press plays the station; the station that is
// chosen already pauses or plays on. Plain buttons with arrow-key navigation (norigin).

interface StationTileProps {
  name: string;
  logoUrl?: string;
  /** The line under the name: country and region. */
  caption?: string;
  /** A note on the station's state («Nicht mehr im Katalog, verschwindet am …»). */
  note?: string;
  /** The station is the radio's chosen one; `playing` while it sounds. */
  state?: 'playing' | 'paused';
  /** The badge on the picture for `state` («Läuft», «Pausiert»). */
  stateLabel?: string;
  /** Accessible name of the tile: what a press does («Radio Swiss Pop abspielen»). */
  playLabel: string;
  onPlay: () => void;
  favorite: { pressed: boolean; label: string; onPress: () => void };
  focusKey?: string;
}

export function StationTile({
  name,
  logoUrl,
  caption,
  note,
  state,
  stateLabel,
  playLabel,
  onPlay,
  favorite,
  focusKey,
}: StationTileProps) {
  const { ref: mainRef, onFocus: onMainFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  const { ref: heartRef, onFocus: onHeartFocus } = useSpatialItem<HTMLButtonElement>({
    focusKey: focusKey ? `${focusKey}-favorite` : undefined,
  });
  return (
    <div className={styles.tile} data-state={state}>
      <button
        ref={mainRef}
        type="button"
        className={styles.main}
        aria-label={playLabel}
        onFocus={onMainFocus}
        onClick={onPlay}
      >
        <span className={styles.picture}>
          <StationLogo name={name} src={logoUrl} size="fill" />
          {state && stateLabel && (
            <span className={styles.badge}>
              <Icon name={state === 'playing' ? 'volume' : 'pause'} size="sm" />
              {stateLabel}
            </span>
          )}
        </span>
        <span className={styles.name}>{name}</span>
        {caption && <span className={styles.caption}>{caption}</span>}
        {note && <span className={styles.note}>{note}</span>}
      </button>
      <button
        ref={heartRef}
        type="button"
        className={styles.favorite}
        aria-label={favorite.label}
        aria-pressed={favorite.pressed}
        title={favorite.label}
        onFocus={onHeartFocus}
        onClick={favorite.onPress}
      >
        <Icon name="heart" size="sm" />
      </button>
    </div>
  );
}
