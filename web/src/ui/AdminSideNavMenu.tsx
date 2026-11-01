import { useCallback, useEffect, useId, useRef, useState, type MouseEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { Dialog, Modal, ModalOverlay } from 'react-aria-components';
import { AdminSideNavContent, type AdminSideNavProps } from './AdminSideNav';
import styles from './AdminSideNavMenu.module.css';
import { Button } from './Button';
import { Icon } from './Icon';
import sheet from './SideNav.module.css';

export interface AdminSideNavMenuProps extends Omit<AdminSideNavProps, 'focusCarry'> {
  /** First part of the button ("Verwaltung"). */
  buttonLabel: string;
  /** The current page's name after it ("Bibliotheken"). */
  pageLabel: string;
  /** Accessible name of the sheet ("Verwaltung, Seiten") and of its close button. */
  menuLabel: string;
  closeLabel: string;
}

/**
 * The administration's sidebar on a phone (concept 11, 11.1): a button "Verwaltung · page" that
 * opens the same entries as a modal sheet from the left, built like SideNavDrawer and with its
 * styles. While it is open the focus stays inside (react-aria), Escape, the close button and a
 * press on the scrim close it, and the focus returns to the button. Following an entry closes it
 * as well. Only the column carries the focus over between pages (`focusCarry`), not the sheet.
 */
export function AdminSideNavMenu({
  buttonLabel,
  pageLabel,
  menuLabel,
  closeLabel,
  ...props
}: AdminSideNavMenuProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const id = useId();
  const triggerRef = useRef<HTMLButtonElement>(null);
  const wasOpen = useRef(false);

  // react-aria would return the focus to what had it when the sheet mounted, but that is already
  // the entry focused below; so the button takes it back itself once the sheet has closed.
  useEffect(() => {
    if (wasOpen.current && !open) triggerRef.current?.focus();
    wasOpen.current = open;
  }, [open]);

  // A sheet opened with Enter counts as a "virtual" press for react-aria, which then focuses the
  // dialog itself. The entry with the Tab stop (the current page) gets the focus here, before the
  // dialog's own effect looks at the focus (as SideNavDrawer does with its first entry).
  const focusTabStop = useCallback((body: HTMLElement | null) => {
    body?.querySelector<HTMLElement>('a[tabindex="0"]')?.focus();
  }, []);

  const closeOnFollow = (event: MouseEvent<HTMLElement>) => {
    if (event.target instanceof Element && event.target.closest('a[href]')) setOpen(false);
  };

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        // One spoken line: the visible parts are separate boxes, whose spaces a name can lose.
        aria-label={[buttonLabel, pageLabel].join(t('format.separator'))}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        onClick={() => setOpen(true)}
      >
        <span className={styles.triggerLabel}>
          <Icon name="grid" />
          {buttonLabel}
          <span className={styles.page}>
            {t('format.separator')}
            {pageLabel}
          </span>
        </span>
        <Icon name="chevronDown" />
      </button>
      <ModalOverlay className={sheet.scrim} isOpen={open} isDismissable onOpenChange={setOpen}>
        <Modal className={sheet.drawer}>
          <Dialog id={id} className={sheet.dialog} aria-label={menuLabel}>
            {/* Clicks only bubble up from the links, which keep their own keyboard handling. */}
            {/* eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions */}
            <div ref={focusTabStop} className={sheet.drawerBody} onClick={closeOnFollow}>
              <div className={sheet.drawerHead}>
                <Button
                  variant="ghost"
                  icon="close"
                  iconOnly
                  aria-label={closeLabel}
                  onPress={() => setOpen(false)}
                />
              </div>
              <AdminSideNavContent {...props} />
            </div>
          </Dialog>
        </Modal>
      </ModalOverlay>
    </>
  );
}
