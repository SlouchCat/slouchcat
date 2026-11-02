import { useRef, type Key } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Header,
  Menu as AriaMenu,
  MenuItem,
  MenuSection,
  MenuTrigger,
  Popover,
} from 'react-aria-components';
import { useDensityPortalProps } from './adminDensity';
import { AriaButton } from './AriaButton';
import { Icon, type IconName } from './Icon';
import styles from './ActionMenu.module.css';

export interface ActionMenuItem {
  id: string;
  label: string;
  icon?: IconName;
  /** Destructive: listed under «Gefahr» at the bottom, in the danger colour. */
  danger?: boolean;
  disabled?: boolean;
}

interface ActionMenuProps {
  /** Name of the row the menu acts on: the button is «Weitere Aktionen für <name>». */
  rowName: string;
  items: readonly ActionMenuItem[];
  /**
   * Gets the picked item's id after the menu closed and the focus is back on its button. A
   * destructive action opens its ConfirmDialog (`isOpen`) here; that dialog returns the focus to
   * the button as well. When the row is gone afterwards, the page moves the focus to its h1.
   */
  onAction: (id: string) => void;
  isDisabled?: boolean;
}

/**
 * «Weitere Aktionen» of a row (concept 12, S-6): an icon button that opens a menu of actions
 * (react-aria Menu). Enter, Space or ↓ open it, the arrow keys move, Enter picks, Escape closes;
 * the focus returns to the button. Destructive actions stand in a section «Gefahr» at the bottom.
 * Unlike MenuButton, which picks one of several values, nothing in it stays checked.
 */
export function ActionMenu({ rowName, items, onAction, isDisabled }: ActionMenuProps) {
  const { t } = useTranslation();
  const density = useDensityPortalProps();
  const button = useRef<HTMLButtonElement>(null);
  const safe = items.filter((item) => !item.danger);
  const danger = items.filter((item) => item.danger);
  const pick = (key: Key) => {
    // react-aria closes the menu and returns the focus to the button after this handler; the
    // page's reaction (a dialog that remembers the focused element) has to come after that.
    requestAnimationFrame(() => {
      button.current?.focus();
      onAction(String(key));
    });
  };
  const item = (entry: ActionMenuItem) => (
    <MenuItem
      key={entry.id}
      id={entry.id}
      textValue={entry.label}
      isDisabled={entry.disabled}
      className={`${styles.item} ${entry.danger ? styles.danger : ''}`}
    >
      {entry.icon && <Icon name={entry.icon} size="sm" />}
      {entry.label}
    </MenuItem>
  );
  return (
    <MenuTrigger>
      <AriaButton
        ref={button}
        variant="ghost"
        icon="more"
        iconOnly
        aria-label={t('ui.actionMenu.label', { name: rowName })}
        isDisabled={isDisabled}
      />
      <Popover className={styles.popover} placement="bottom end" offset={4} {...density}>
        <AriaMenu className={styles.menu} onAction={pick}>
          {safe.length > 0 && (
            <MenuSection className={styles.section}>{safe.map(item)}</MenuSection>
          )}
          {danger.length > 0 && (
            <MenuSection className={styles.section}>
              <Header className={styles.header}>{t('ui.actionMenu.danger')}</Header>
              {danger.map(item)}
            </MenuSection>
          )}
        </AriaMenu>
      </Popover>
    </MenuTrigger>
  );
}
