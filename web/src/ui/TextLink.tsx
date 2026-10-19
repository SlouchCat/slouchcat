import type { ReactNode } from 'react';
import { AppLink } from './AppLink';
import { Icon } from './Icon';
import styles from './TextLink.module.css';

interface TextLinkProps {
  href: string;
  children: ReactNode;
  /** Opens in a new tab and shows the external icon. */
  external?: boolean;
}

/** Inline text link. */
export function TextLink({ href, children, external = false }: TextLinkProps) {
  return (
    <AppLink
      href={href}
      className={styles.link}
      target={external ? '_blank' : undefined}
      rel={external ? 'noreferrer noopener' : undefined}
    >
      {children}
      {external && <Icon name="external" size="sm" />}
    </AppLink>
  );
}
