import { type Ref, useId } from 'react';
import { AppLink } from './AppLink';
import { Icon, type IconName } from './Icon';
import styles from './RoundAction.module.css';
import { useSpatialItem } from './spatial';

interface RoundActionProps {
  /** The text under the circle; the action's name unless `aria-label` says more. */
  label: string;
  icon: IconName;
  /** A link (the player, an administration page); without it a button that calls `onPress`. */
  href?: string;
  onPress?: () => void;
  /** A switch (Merken, Favorit): `aria-pressed`, and the circle in the accent while on. */
  pressed?: boolean;
  isDisabled?: boolean;
  /** A press in flight: still focusable, but the press does nothing (`aria-disabled`). */
  busy?: boolean;
  /** A longer spoken name that contains the label ("Zu den User-Empfehlungen"). */
  'aria-label'?: string;
  /** Read after the name ("Bei neuen Folgen benachrichtigen"). */
  description?: string;
  focusKey?: string;
  onArrowPress?: (direction: string) => boolean;
}

/**
 * A round action with its label underneath (concept 17 D-2, variant A): a circle with the icon,
 * the text below it, one link or button. The row of the detail page under the big play button
 * (Trailer, Von vorne, Merken, Favorit, User-Empfehlungen, Download, Verwalten).
 */
export function RoundAction({
  label,
  icon,
  href,
  onPress,
  pressed,
  isDisabled = false,
  busy = false,
  'aria-label': ariaLabel,
  description,
  focusKey,
  onArrowPress,
}: RoundActionProps) {
  const { ref, onFocus } = useSpatialItem<HTMLElement>({ focusKey, onArrowPress });
  const descriptionId = useId();
  const describedBy = description ? descriptionId : undefined;
  const className = pressed ? `${styles.action} ${styles.on}` : styles.action;
  const content = (
    <>
      <span className={styles.circle}>
        <Icon name={pressed ? 'check' : icon} size="md" />
      </span>
      <span className={styles.label}>{label}</span>
    </>
  );
  const hint = description && (
    <span id={descriptionId} hidden>
      {description}
    </span>
  );
  if (href !== undefined && !isDisabled) {
    return (
      <>
        <AppLink
          ref={ref as Ref<HTMLAnchorElement>}
          href={href}
          className={className}
          aria-label={ariaLabel}
          aria-describedby={describedBy}
          onFocus={onFocus}
        >
          {content}
        </AppLink>
        {hint}
      </>
    );
  }
  return (
    <>
      <button
        ref={ref as Ref<HTMLButtonElement>}
        type="button"
        className={className}
        disabled={isDisabled}
        aria-label={ariaLabel}
        aria-describedby={describedBy}
        aria-pressed={pressed}
        aria-disabled={busy || undefined}
        onFocus={onFocus}
        onClick={() => {
          if (!busy) onPress?.();
        }}
      >
        {content}
      </button>
      {hint}
    </>
  );
}
