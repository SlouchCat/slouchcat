import { swatchStyle } from './accent';
import { Icon } from './Icon';
import styles from './ProfileTile.module.css';
import { useSpatialItem } from './spatial';
import { VisuallyHidden } from './VisuallyHidden';

interface ProfileAvatarProps {
  name: string;
  /** Accent token name of the profile (`teal`); the default accent when missing. */
  accent?: string;
  size?: 'md' | 'lg';
}

/** Round avatar: the first letter of the name on the profile's accent colour. */
export function ProfileAvatar({ name, accent, size = 'md' }: ProfileAvatarProps) {
  return (
    <span
      className={`${styles.avatar} ${styles[size]}`}
      style={swatchStyle(accent)}
      aria-hidden="true"
    >
      {name.slice(0, 1).toUpperCase()}
    </span>
  );
}

interface ProfileTileProps {
  name: string;
  accent?: string;
  hasPin: boolean;
  /** The profile the session runs with now. */
  isActive: boolean;
  /** Spoken label of the lock ("protected by a PIN"). */
  lockedLabel: string;
  /** Small line under the name ("Primary profile · active"), translated by the page. */
  caption?: string;
  onPress: () => void;
  focusKey?: string;
  /** Takes focus when the picker opens, so Enter or the arrow keys work right away. */
  autoFocus?: boolean;
}

/**
 * Tile of the profile picker: avatar, name, a lock when a PIN is set. One button; the arrow keys
 * move between tiles (spatial navigation), Enter picks the profile.
 */
export function ProfileTile({
  name,
  accent,
  hasPin,
  isActive,
  lockedLabel,
  caption,
  onPress,
  focusKey,
  autoFocus,
}: ProfileTileProps) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      // The picker is all there is on its page: focus belongs on a tile.
      // eslint-disable-next-line jsx-a11y/no-autofocus
      autoFocus={autoFocus}
      className={styles.tile}
      aria-current={isActive ? 'true' : undefined}
      data-active={isActive || undefined}
      onFocus={onFocus}
      onClick={onPress}
    >
      <span className={styles.avatarWrap}>
        <ProfileAvatar name={name} accent={accent} size="lg" />
        {hasPin && (
          <span className={styles.lock} aria-hidden="true">
            <Icon name="lock" size="sm" />
          </span>
        )}
      </span>
      <span className={styles.name}>{name}</span>
      {hasPin && <VisuallyHidden>{lockedLabel}</VisuallyHidden>}
      {caption && <span className={styles.caption}>{caption}</span>}
    </button>
  );
}
