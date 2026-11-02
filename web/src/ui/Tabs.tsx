import type { Key, ReactNode } from 'react';
import { Tab, TabList, TabPanel, Tabs as AriaTabs } from 'react-aria-components';
import styles from './Tabs.module.css';
import { VisuallyHidden } from './VisuallyHidden';

export interface TabItem {
  id: string;
  label: string;
  /** Rendered only while the tab is selected. */
  content: ReactNode;
  /** A number after the label ("265"), formatted by the page. */
  count?: string;
  /** Spoken form of the number ("265 fehlgeschlagen"); without it the number is read as shown. */
  countLabel?: string;
  /** Colour of the number: `danger` for failures, `warning` for something to look at. */
  tone?: 'neutral' | 'warning' | 'danger';
}

export interface TabsProps {
  /** Spoken name of the tab list. */
  label: string;
  tabs: readonly TabItem[];
  selectedKey?: string;
  onSelectionChange?: (key: string) => void;
  /**
   * Content before the tab list in the same row (the title of a BandPanel), with the class of
   * that row; the list then drops its own underline (`variant="band"`).
   */
  head?: ReactNode;
  headClassName?: string;
  /** Class of the panels (the padding of a BandPanel's body). */
  panelClassName?: string;
}

/**
 * Tabs (react-aria): the arrow keys move between the tabs and select them, Tab moves on into the
 * panel. Used by the administration pages. A tab may carry a number with a tone (concept 12,
 * 11.2). Only the selected panel is mounted: what a reader typed in a tab lives in the page's
 * state (useDrafts, S-17), not in the panel. A page binds the selection to `?tab=` with
 * useSearchSelection.
 */
export function Tabs({
  label,
  tabs,
  selectedKey,
  onSelectionChange,
  head,
  headClassName,
  panelClassName,
}: TabsProps) {
  const list = (
    <div className={styles.scroller}>
      <TabList
        aria-label={label}
        className={`${styles.list} ${head === undefined ? '' : styles.inHead}`}
      >
        {tabs.map((tab) => (
          <Tab key={tab.id} id={tab.id} className={styles.tab}>
            {tab.label}
            {/* The space keeps label and number apart in the spoken name. */}
            {tab.count !== undefined && ' '}
            {tab.count !== undefined && (
              <span className={`${styles.count} ${styles[tab.tone ?? 'neutral']}`}>
                <span aria-hidden={tab.countLabel ? true : undefined}>{tab.count}</span>
                {tab.countLabel && <VisuallyHidden>{tab.countLabel}</VisuallyHidden>}
              </span>
            )}
          </Tab>
        ))}
      </TabList>
    </div>
  );
  return (
    <AriaTabs
      className={`${styles.tabs} ${head === undefined ? '' : styles.headed}`}
      selectedKey={selectedKey}
      onSelectionChange={(key: Key) => onSelectionChange?.(String(key))}
    >
      {head === undefined ? (
        list
      ) : (
        <div className={headClassName}>
          {head}
          {list}
        </div>
      )}
      {tabs.map((tab) => (
        <TabPanel key={tab.id} id={tab.id} className={`${styles.panel} ${panelClassName ?? ''}`}>
          {tab.content}
        </TabPanel>
      ))}
    </AriaTabs>
  );
}
