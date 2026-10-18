import type { ReactNode } from 'react';
import { Link } from 'react-aria-components';
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
    <Link
      href={href}
      className={styles.link}
      target={external ? '_blank' : undefined}
      rel={external ? 'noreferrer noopener' : undefined}
    >
      {children}
      {external && <Icon name="external" size="sm" />}
    </Link>
  );
}
