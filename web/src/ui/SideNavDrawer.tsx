import { useCallback } from 'react';
import { Dialog, Modal, ModalOverlay } from 'react-aria-components';
import { SpatialButton } from './Button';
import { SideNavContent, type SideNavMenuProps } from './SideNav';
import styles from './SideNav.module.css';
import { SpatialGroup } from './spatial';

/**
 * The side navigation as a modal drawer from the left (react-aria, loaded by SideNavMenu): the
 * focus moves to the first entry, Tab and the arrow keys stay inside, the page behind does not
 * scroll, Escape, the close button and a press on the scrim close it, and the focus returns to the
 * menu button that opened it.
 */
export function SideNavDrawer({
  isOpen,
  onClose,
  id,
  menuLabel,
  closeLabel,
  show,
  ...props
}: SideNavMenuProps) {
  // A drawer opened with Enter counts as a "virtual" press for react-aria, which then focuses the
  // dialog itself. The first entry gets the focus here, before the dialog's own effect looks at
  // the focus (as FormDialog does with its first field).
  const focusFirstEntry = useCallback((nav: HTMLElement | null) => {
    nav?.querySelector<HTMLElement>('a[href], button')?.focus();
  }, []);

  return (
    <ModalOverlay
      className={styles.scrim}
      isOpen={isOpen}
      isDismissable
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Modal className={styles.drawer}>
        <Dialog id={id} className={styles.dialog} aria-label={menuLabel}>
          <SpatialGroup
            focusKey="sidenav-drawer"
            className={styles.drawerBody}
            tabIndex={-1}
            focusBoundary
          >
            <div className={styles.drawerHead}>
              <SpatialButton
                variant="ghost"
                icon="close"
                iconOnly
                aria-label={closeLabel}
                onPress={onClose}
              />
            </div>
            <nav ref={focusFirstEntry} aria-label={props.label} className={styles.drawerNav}>
              <SideNavContent
                {...props}
                prefix="drawer"
                last={show && { label: show.label, focusKey: 'drawer-show', onPress: show.onPress }}
              />
            </nav>
          </SpatialGroup>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
