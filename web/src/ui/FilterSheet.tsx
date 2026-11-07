import type { ReactNode } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import styles from './FilterSheet.module.css';
import { focusOnMount } from './focusOnMount';

// A sheet from the bottom edge for the filters of a page on a phone (concept 16, 10.4: «am Handy
// die Filter als Blatt von unten»). Modal (react-aria): Escape, the close button or a tap beside
// it close the sheet and the focus returns to the button that opened it.

interface FilterSheetProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: string;
  closeLabel: string;
  children: ReactNode;
  /** The row at the bottom: «Auswahl löschen», «n Sender anzeigen». */
  footer?: ReactNode;
}

export function FilterSheet({
  isOpen,
  onOpenChange,
  title,
  closeLabel,
  children,
  footer,
}: FilterSheetProps) {
  return (
    <ModalOverlay
      className={styles.overlay}
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      isDismissable
    >
      <Modal className={styles.sheet}>
        <Dialog className={styles.dialog}>
          <div className={styles.head}>
            <Heading slot="title" className={styles.title}>
              {title}
            </Heading>
            <AriaButton
              variant="ghost"
              icon="close"
              iconOnly
              aria-label={closeLabel}
              ref={focusOnMount}
              onPress={() => onOpenChange(false)}
            />
          </div>
          <div className={styles.body}>{children}</div>
          {footer && <div className={styles.footer}>{footer}</div>}
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
