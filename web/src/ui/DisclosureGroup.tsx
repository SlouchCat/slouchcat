import type { ReactNode } from 'react';
import { Button, Disclosure, DisclosurePanel, Heading } from 'react-aria-components';
import styles from './DisclosureGroup.module.css';
import { Icon } from './Icon';
import { VisuallyHidden } from './VisuallyHidden';

interface DisclosureGroupProps {
  /** The heading: a library, a job group, a device type. */
  title: string;
  /** h3 inside a section of the page (the default), h2 when the groups are the sections. */
  level?: 2 | 3;
  /** A number after the title ("12"), formatted by the page. */
  count?: string;
  /** Spoken form of the number ("12 Titel"). */
  countLabel?: string;
  /** Small state at the end of the heading line, outside the button. */
  meta?: ReactNode;
  /** Open state when the page holds it; uncontrolled (open at first) without. */
  isExpanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  /** The group's own content, usually a DataTable. */
  children: ReactNode;
}

/**
 * A section that folds away (concept 12, 11.2): the heading is a button with `aria-expanded`
 * (react-aria Disclosure), the content (one table per library, type or kind) follows it. Folded
 * content stays mounted and hidden, so what was typed in it stays (S-13). Rows that fold out
 * inside a table use DataTable's `expand` instead: a `div` does not belong in a `<tr>`.
 */
export function DisclosureGroup({
  title,
  level = 3,
  count,
  countLabel,
  meta,
  isExpanded,
  onExpandedChange,
  children,
}: DisclosureGroupProps) {
  return (
    <Disclosure
      className={styles.group}
      defaultExpanded
      isExpanded={isExpanded}
      onExpandedChange={onExpandedChange}
    >
      <div className={styles.head}>
        <Heading level={level} className={styles.heading}>
          <Button slot="trigger" className={styles.trigger}>
            <span className={styles.chevron}>
              <Icon name="chevronRight" size="sm" />
            </span>
            {title}
            {count !== undefined && ' '}
            {count !== undefined && (
              <span className={styles.count}>
                <span aria-hidden={countLabel ? true : undefined}>{count}</span>
                {countLabel && <VisuallyHidden>{countLabel}</VisuallyHidden>}
              </span>
            )}
          </Button>
        </Heading>
        {meta && <div className={styles.meta}>{meta}</div>}
      </div>
      <DisclosurePanel className={styles.panel}>{children}</DisclosurePanel>
    </Disclosure>
  );
}
