import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import styles from './Dialog.module.css';
import { focusOnMount } from './focusOnMount';
import { useSpatialHold } from './spatial';

interface WatchedChoiceProps {
  isOpen: boolean;
  /** Escape, a click beside the dialog and every button close it through this. */
  onOpenChange: (isOpen: boolean) => void;
  /** «Nordlicht» ist teilweise gesehen / «Staffel 1» als ungesehen markieren? */
  title: string;
  /** One or two sentences: how many episodes and that a removal cannot be undone. */
  body: string;
  cancelLabel: string;
  /** «Alle Haken entfernen»: a danger button. */
  removeLabel: string;
  onRemove: () => void;
  /**
   * «Alle Folgen als gesehen markieren»: only for a half check mark; it then has the focus. Without
   * it the dialog only confirms the removal and «Abbrechen» has the focus.
   */
  markAllLabel?: string;
  onMarkAll?: () => void;
}

/**
 * The question of the check mark of a season, series or podcast (concept 20 P-5): a full check
 * asks before every episode loses it, a half one offers to mark all or to remove all. Escape and
 * «Abbrechen» do nothing else; the focus goes back to the check mark that asked. The page's
 * arrow-key navigation holds while it is open.
 */
export function WatchedChoice({
  isOpen,
  onOpenChange,
  title,
  body,
  cancelLabel,
  removeLabel,
  onRemove,
  markAllLabel,
  onMarkAll,
}: WatchedChoiceProps) {
  useSpatialHold(isOpen);
  const mixed = markAllLabel !== undefined && onMarkAll !== undefined;
  return (
    <ModalOverlay
      className={styles.overlay}
      isDismissable
      isOpen={isOpen}
      onOpenChange={onOpenChange}
    >
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog} role="alertdialog">
          {({ close }) => (
            <>
              <Heading slot="title" className={styles.title}>
                {title}
              </Heading>
              <p className={styles.body}>{body}</p>
              <div className={styles.actions}>
                <AriaButton variant="ghost" ref={mixed ? undefined : focusOnMount} onPress={close}>
                  {cancelLabel}
                </AriaButton>
                {mixed && (
                  <AriaButton
                    ref={focusOnMount}
                    onPress={() => {
                      close();
                      onMarkAll();
                    }}
                  >
                    {markAllLabel}
                  </AriaButton>
                )}
                <AriaButton
                  variant="danger"
                  onPress={() => {
                    close();
                    onRemove();
                  }}
                >
                  {removeLabel}
                </AriaButton>
              </div>
            </>
          )}
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
