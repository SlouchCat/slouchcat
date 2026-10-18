import type { AnchorHTMLAttributes, ButtonHTMLAttributes, ReactNode, Ref } from 'react';
import { AppLink } from './AppLink';
import styles from './Button.module.css';
import { Icon, type IconName } from './Icon';
import { useSpatialItem } from './spatial';

// Plain buttons and button-styled links for the browsing pages (start, detail, header, error
// pages): native <button> and <a> without react-aria, so the start page does not load it (P-10).
// Forms, dialogs and the player use AriaButton from '@/ui/aria' with the same look.

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

export interface ButtonLook {
  variant?: ButtonVariant;
  size?: 'md' | 'lg';
  icon?: IconName;
  /** Square button that shows only the icon; pass `aria-label`. */
  iconOnly?: boolean;
  /** Stretch to the width of the container (forms on phones). */
  block?: boolean;
}

export function buttonClass({
  variant = 'secondary',
  size = 'md',
  iconOnly,
  block,
}: ButtonLook): string {
  return [
    styles.button,
    styles[variant],
    styles[size],
    iconOnly && styles.iconOnly,
    block && styles.block,
  ]
    .filter(Boolean)
    .join(' ');
}

export function ButtonContent({ icon, children }: { icon?: IconName; children?: ReactNode }) {
  return (
    <>
      {icon && <Icon name={icon} size="md" />}
      {children}
    </>
  );
}

interface ButtonProps
  extends
    Omit<ButtonHTMLAttributes<HTMLButtonElement>, 'className' | 'style' | 'onClick' | 'disabled'>,
    ButtonLook {
  onPress?: () => void;
  isDisabled?: boolean;
  /** Reachable by pointer only (paging buttons whose content arrow keys already reach). */
  excludeFromTabOrder?: boolean;
  children?: ReactNode;
  ref?: Ref<HTMLButtonElement>;
}

/** Action button: primary for the one main action, secondary and ghost otherwise. */
export function Button({
  variant,
  size,
  icon,
  iconOnly,
  block,
  onPress,
  isDisabled,
  excludeFromTabOrder,
  type = 'button',
  children,
  ref,
  ...rest
}: ButtonProps) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClass({ variant, size, iconOnly, block })}
      disabled={isDisabled}
      tabIndex={excludeFromTabOrder ? -1 : undefined}
      onClick={onPress}
      {...rest}
    >
      <ButtonContent icon={icon}>{children}</ButtonContent>
    </button>
  );
}

interface ButtonLinkProps
  extends Omit<AnchorHTMLAttributes<HTMLAnchorElement>, 'className' | 'style'>, ButtonLook {
  href: string;
  children?: ReactNode;
  ref?: Ref<HTMLAnchorElement>;
}

/** A link that looks like a button; navigation goes through the app router. */
export function ButtonLink({
  variant,
  size,
  icon,
  iconOnly,
  block,
  children,
  ref,
  ...rest
}: ButtonLinkProps) {
  return (
    <AppLink ref={ref} className={buttonClass({ variant, size, iconOnly, block })} {...rest}>
      <ButtonContent icon={icon}>{children}</ButtonContent>
    </AppLink>
  );
}

interface SpatialOptions {
  /** norigin focus key; generated when omitted. */
  focusKey?: string;
  /** Return false to consume an arrow press. */
  onArrowPress?: (direction: string) => boolean;
}

/** Button that takes part in arrow-key navigation on browsing pages. */
export function SpatialButton({
  focusKey,
  onArrowPress,
  ...rest
}: Omit<ButtonProps, 'ref'> & SpatialOptions) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey, onArrowPress });
  return (
    <Button
      ref={ref}
      {...rest}
      onFocus={(event) => {
        onFocus();
        rest.onFocus?.(event);
      }}
    />
  );
}

/** ButtonLink that takes part in arrow-key navigation on browsing pages. */
export function SpatialButtonLink({
  focusKey,
  onArrowPress,
  ...rest
}: Omit<ButtonLinkProps, 'ref'> & SpatialOptions) {
  const { ref, onFocus } = useSpatialItem<HTMLAnchorElement>({ focusKey, onArrowPress });
  return (
    <ButtonLink
      ref={ref}
      {...rest}
      onFocus={(event) => {
        onFocus();
        rest.onFocus?.(event);
      }}
    />
  );
}
