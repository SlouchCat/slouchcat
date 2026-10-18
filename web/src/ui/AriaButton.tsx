import type { ReactNode, Ref } from 'react';
import { Button, type ButtonProps } from 'react-aria-components';
import { ButtonContent, buttonClass, type ButtonLook } from './Button';

interface AriaButtonProps
  extends Omit<ButtonProps, 'className' | 'children' | 'style'>, ButtonLook {
  children?: ReactNode;
  ref?: Ref<HTMLButtonElement>;
}

/**
 * The react-aria button with the look of Button: for forms (submit with validation, `isPending`),
 * dialog triggers and dialogs, and the player. Browsing pages use the plain Button from '@/ui'.
 */
export function AriaButton({
  variant,
  size,
  icon,
  iconOnly,
  block,
  children,
  ref,
  ...rest
}: AriaButtonProps) {
  return (
    <Button ref={ref} className={buttonClass({ variant, size, iconOnly, block })} {...rest}>
      <ButtonContent icon={icon}>{children}</ButtonContent>
    </Button>
  );
}
