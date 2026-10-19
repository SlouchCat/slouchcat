import type { Key, ReactNode } from 'react';
import { Tab, TabList, TabPanel, Tabs as AriaTabs } from 'react-aria-components';
import styles from './Tabs.module.css';

interface TabItem {
  id: string;
  label: string;
  /** Rendered only while the tab is selected. */
  content: ReactNode;
}

interface TabsProps {
  /** Spoken name of the tab list. */
  label: string;
  tabs: readonly TabItem[];
  selectedKey?: string;
  onSelectionChange?: (key: string) => void;
}

/**
 * Tabs (react-aria): the arrow keys move between the tabs and select them, Tab moves on into the
 * panel. Used by the administration pages.
 */
export function Tabs({ label, tabs, selectedKey, onSelectionChange }: TabsProps) {
  return (
    <AriaTabs
      className={styles.tabs}
      selectedKey={selectedKey}
      onSelectionChange={(key: Key) => onSelectionChange?.(String(key))}
    >
      <div className={styles.scroller}>
        <TabList aria-label={label} className={styles.list}>
          {tabs.map((tab) => (
            <Tab key={tab.id} id={tab.id} className={styles.tab}>
              {tab.label}
            </Tab>
          ))}
        </TabList>
      </div>
      {tabs.map((tab) => (
        <TabPanel key={tab.id} id={tab.id} className={styles.panel}>
          {tab.content}
        </TabPanel>
      ))}
    </AriaTabs>
  );
}
