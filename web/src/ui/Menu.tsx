import type { Key } from 'react';
import {
  Header,
  Menu as AriaMenu,
  MenuItem,
  MenuSection,
  MenuTrigger,
  Popover,
} from 'react-aria-components';
import { AriaButton } from './AriaButton';
import type { ButtonVariant } from './Button';
import { Icon, type IconName } from './Icon';
import styles from './Menu.module.css';

export interface MenuOption {
  id: string;
  label: string;
  /** Shown but not selectable (a quality the source or the limits rule out). */
  disabled?: boolean;
}

/** A titled group of options with a choice of its own ("Ton", "Untertitel"). */
export interface MenuGroup {
  id: string;
  label: string;
  options: readonly MenuOption[];
  selectedKey: string;
}

interface MenuButtonProps {
  /** Text of the button (the current choice, "1080p"). */
  label: string;
  /** Spoken name of the menu ("Version"). */
  'aria-label': string;
  /** Spoken name of the button, with the current choice ("Version: 1080p"). */
  buttonLabel: string;
  /** One list with one choice; or `groups`. */
  options?: readonly MenuOption[];
  selectedKey?: string;
  /** Several titled lists, each with its own choice; ids are unique across groups. */
  groups?: readonly MenuGroup[];
  /** Called with the picked option's id; with `group` the id of its group. */
  onSelect: (id: string, group?: string) => void;
  /** Also report a pick of the current choice (a chapter jumps to its start again). */
  reselect?: boolean;
  icon?: IconName;
  variant?: ButtonVariant;
  /**
   * Element the menu renders into instead of the document body: an element in full screen (the
   * player) shows only its own subtree, a menu outside it would be invisible.
   */
  portalContainer?: Element;
  /** Open state when the page opens the menu itself (a shortcut key); uncontrolled without. */
  isOpen?: boolean;
  onOpenChange?: (isOpen: boolean) => void;
}

function Item({ option }: { option: MenuOption }) {
  return (
    <MenuItem
      id={option.id}
      textValue={option.label}
      className={styles.item}
      isDisabled={option.disabled}
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
  );
}

/**
 * A button that opens a single-choice menu (react-aria): Enter or Space opens it, the arrow keys
 * move, Enter picks, Escape closes; focus returns to the button. The current choice carries a tick;
 * with `groups` every group has a choice of its own under its title.
 */
export function MenuButton({
  label,
  'aria-label': ariaLabel,
  buttonLabel,
  options = [],
  selectedKey,
  groups,
  onSelect,
  reselect = false,
  icon,
  variant = 'ghost',
  portalContainer,
  isOpen,
  onOpenChange,
}: MenuButtonProps) {
  const groupOf = (key: string) =>
    groups?.find((group) => group.options.some((option) => option.id === key));
  const onAction = (key: Key) => {
    const id = String(key);
    const group = groupOf(id);
    const current = group ? group.selectedKey : selectedKey;
    if (reselect || id !== current) onSelect(id, group?.id);
  };
  return (
    <MenuTrigger isOpen={isOpen} onOpenChange={onOpenChange}>
      <AriaButton variant={variant} icon={icon} aria-label={buttonLabel}>
        {label}
      </AriaButton>
      <Popover
        className={styles.popover}
        placement="top end"
        offset={8}
        UNSTABLE_portalContainer={portalContainer}
      >
        {groups ? (
          <AriaMenu
            aria-label={ariaLabel}
            className={styles.menu}
            onAction={onAction}
            // The sections hold the selection, so the menu has no selected key to land on: the
            // first item takes the focus (and its ring) when the menu opens.
            autoFocus="first"
          >
            {groups.map((group) => (
              <MenuSection
                key={group.id}
                id={group.id}
                className={styles.section}
                selectionMode="single"
                selectedKeys={[group.selectedKey]}
              >
                <Header className={styles.header}>{group.label}</Header>
                {group.options.map((option) => (
                  <Item key={option.id} option={option} />
                ))}
              </MenuSection>
            ))}
          </AriaMenu>
        ) : (
          <AriaMenu
            aria-label={ariaLabel}
            className={styles.menu}
            selectionMode="single"
            selectedKeys={selectedKey === undefined ? [] : [selectedKey]}
            onAction={onAction}
          >
            {options.map((option) => (
              <Item key={option.id} option={option} />
            ))}
          </AriaMenu>
        )}
      </Popover>
    </MenuTrigger>
  );
}
