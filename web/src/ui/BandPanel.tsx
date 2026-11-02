import { useId, useState, type ReactNode } from 'react';
import styles from './BandPanel.module.css';
import { Icon, type IconName } from './Icon';
import { Tabs, type TabsProps } from './Tabs';

interface BandPanelProps {
  title: string;
  icon?: IconName;
  /** Small state at the end of the band ("3 von 4 eingerichtet", a Chip). */
  meta?: ReactNode;
  /** The title becomes a button that folds the body away (`aria-expanded`). */
  collapsible?: boolean;
  /** Open state when the page holds it; uncontrolled (open at first) without. */
  isExpanded?: boolean;
  onExpandedChange?: (expanded: boolean) => void;
  /** A tab bar in the band, after the title; the panels form the body (`children` is unused). */
  tabs?: Omit<TabsProps, 'head' | 'headClassName' | 'panelClassName'>;
  children?: ReactNode;
}

/**
 * A panel with a tinted head band (concept 12, 11.2): the profile's accent at 12 % behind the
 * title, the same `--sc-band-tint` technique as the bands of the administration's sidebar
 * (concept 11). The band may fold the body away or carry a tab bar. The body stays mounted while
 * folded, so nothing typed in it is lost (S-13).
 */
export function BandPanel({
  title,
  icon,
  meta,
  collapsible = false,
  isExpanded,
  onExpandedChange,
  tabs,
  children,
}: BandPanelProps) {
  const headingId = useId();
  const bodyId = useId();
  const [ownExpanded, setOwnExpanded] = useState(true);
  const expanded = !collapsible || (isExpanded ?? ownExpanded);
  const toggle = () => {
    setOwnExpanded(!expanded);
    onExpandedChange?.(!expanded);
  };

  const label = (
    <>
      {icon && (
        <span className={styles.icon}>
          <Icon name={icon} size="sm" />
        </span>
      )}
      {title}
    </>
  );
  const heading = (
    <h2 id={headingId} className={styles.title}>
      {collapsible ? (
        <button
          type="button"
          className={styles.toggle}
          aria-expanded={expanded}
          aria-controls={bodyId}
          onClick={toggle}
        >
          <span className={`${styles.chevron} ${expanded ? styles.open : ''}`}>
            <Icon name="chevronRight" size="sm" />
          </span>
          {label}
        </button>
      ) : (
        label
      )}
    </h2>
  );

  if (tabs) {
    return (
      <section className={styles.panel} aria-labelledby={headingId}>
        <Tabs
          {...tabs}
          head={
            <>
              {heading}
              {meta && <div className={styles.meta}>{meta}</div>}
            </>
          }
          headClassName={`${styles.band} ${styles.withTabs}`}
          panelClassName={styles.body}
        />
      </section>
    );
  }

  return (
    <section className={styles.panel} aria-labelledby={headingId}>
      <div className={styles.band}>
        {heading}
        {meta && <div className={styles.meta}>{meta}</div>}
      </div>
      <div id={bodyId} className={styles.body} hidden={!expanded}>
        {children}
      </div>
    </section>
  );
}
