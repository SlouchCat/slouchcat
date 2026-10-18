import type { ReactNode, Ref } from 'react';
import {
  Button as AriaButton,
  Link as AriaLink,
  type ButtonProps as AriaButtonProps,
  type LinkProps as AriaLinkProps,
} from 'react-aria-components';
import styles from './Button.module.css';
import { Icon, type IconName } from './Icon';
import { useSpatialItem } from './spatial';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger';

interface Look {
  variant?: ButtonVariant;
  size?: 'md' | 'lg';
  icon?: IconName;
  /** Square button that shows only the icon; pass `aria-label`. */
  iconOnly?: boolean;
  /** Stretch to the width of the container (forms on phones). */
  block?: boolean;
}

function lookClass({ variant = 'secondary', size = 'md', iconOnly, block }: Look): string {
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

function Content({ icon, children }: { icon?: IconName; children?: ReactNode }) {
  return (
    <>
      {icon && <Icon name={icon} size="md" />}
      {children}
    </>
  );
}

interface ButtonProps extends Omit<AriaButtonProps, 'className' | 'children' | 'style'>, Look {
  children?: ReactNode;
  ref?: Ref<HTMLButtonElement>;
}

/** Action button (react-aria): primary for the one main action, secondary and ghost otherwise. */
export function Button({
  variant,
  size,
  icon,
  iconOnly,
  block,
  children,
  ref,
  ...rest
}: ButtonProps) {
  return (
    <AriaButton ref={ref} className={lookClass({ variant, size, iconOnly, block })} {...rest}>
      <Content icon={icon}>{children}</Content>
    </AriaButton>
  );
}

interface ButtonLinkProps extends Omit<AriaLinkProps, 'className' | 'children' | 'style'>, Look {
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
    <AriaLink ref={ref} className={lookClass({ variant, size, iconOnly, block })} {...rest}>
      <Content icon={icon}>{children}</Content>
    </AriaLink>
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
