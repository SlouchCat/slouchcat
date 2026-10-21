import type { ReactNode } from 'react';
import { Switch as AriaSwitch, type SwitchProps as AriaSwitchProps } from 'react-aria-components';
import styles from './Switch.module.css';

interface SwitchProps extends Omit<AriaSwitchProps, 'className' | 'children' | 'style'> {
  /** The visible label next to the track. */
  children: ReactNode;
}

/**
 * On/off switch (react-aria, role switch): Space toggles, Tab moves on; the label is part of the
 * control, so a click on it toggles as well.
 */
export function Switch({ children, ...rest }: SwitchProps) {
  return (
    <AriaSwitch className={styles.switch} {...rest}>
      <span className={styles.track} aria-hidden="true">
        <span className={styles.handle} />
      </span>
      {children}
    </AriaSwitch>
  );
}
