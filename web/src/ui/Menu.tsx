import type { Key } from 'react';
import {
  Menu as AriaMenu,
  MenuItem,
  MenuTrigger,
  Popover,
  type Selection,
} from 'react-aria-components';
import { AriaButton } from './AriaButton';
import type { ButtonVariant } from './Button';
import { Icon, type IconName } from './Icon';
import styles from './Menu.module.css';

interface MenuOption {
  id: string;
  label: string;
}

interface MenuButtonProps {
  /** Text of the button (the current choice, "1080p"). */
  label: string;
  /** Spoken name of the menu ("Version"). */
  'aria-label': string;
  /** Spoken name of the button, with the current choice ("Version: 1080p"). */
  buttonLabel: string;
  options: readonly MenuOption[];
  selectedKey: string;
  onSelect: (id: string) => void;
  icon?: IconName;
  variant?: ButtonVariant;
}

/**
 * A button that opens a single-choice menu (react-aria): Enter or Space opens it, the arrow keys
 * move, Enter picks, Escape closes; focus returns to the button. The current choice carries a tick.
 */
export function MenuButton({
  label,
  'aria-label': ariaLabel,
  buttonLabel,
  options,
  selectedKey,
  onSelect,
  icon,
  variant = 'ghost',
}: MenuButtonProps) {
  const onSelectionChange = (selection: Selection) => {
    if (selection === 'all') return;
    const [key] = [...selection] as Key[];
    if (key !== undefined && String(key) !== selectedKey) onSelect(String(key));
  };
  return (
    <MenuTrigger>
      <AriaButton variant={variant} icon={icon} aria-label={buttonLabel}>
        {label}
      </AriaButton>
      <Popover className={styles.popover} placement="top end" offset={8}>
        <AriaMenu
          aria-label={ariaLabel}
          className={styles.menu}
          selectionMode="single"
          selectedKeys={[selectedKey]}
          onSelectionChange={onSelectionChange}
        >
          {options.map((option) => (
            <MenuItem
              key={option.id}
              id={option.id}
              textValue={option.label}
              className={styles.item}
            >
              {({ isSelected }) => (
                <>
                  <span className={styles.tick} aria-hidden="true">
                    {isSelected && <Icon name="check" size="sm" />}
                  </span>
                  {option.label}
                </>
              )}
            </MenuItem>
          ))}
        </AriaMenu>
      </Popover>
    </MenuTrigger>
  );
}
