import { useCallback, type ReactElement, type ReactNode } from 'react';
import { Dialog, DialogTrigger, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import styles from './Dialog.module.css';

interface ConfirmDialogProps {
  /** The button that opens the dialog. */
  trigger: ReactElement;
  title: string;
  body: string;
  confirmLabel: string;
  cancelLabel: string;
  onConfirm: () => void;
  tone?: 'primary' | 'danger';
}

/**
 * Modal confirmation (react-aria): focus moves into the dialog, Escape and Cancel close it and
 * focus returns to the trigger.
 */
export function ConfirmDialog({
  trigger,
  title,
  body,
  confirmLabel,
  cancelLabel,
  onConfirm,
  tone = 'primary',
}: ConfirmDialogProps) {
  return (
    <DialogTrigger>
      {trigger}
      <ModalOverlay className={styles.overlay} isDismissable>
        <Modal className={styles.modal}>
          <Dialog className={styles.dialog} role="alertdialog">
            {({ close }) => (
              <>
                <Heading slot="title" className={styles.title}>
                  {title}
                </Heading>
                <p className={styles.body}>{body}</p>
                <div className={styles.actions}>
                  <AriaButton variant="ghost" onPress={close}>
                    {cancelLabel}
                  </AriaButton>
                  <AriaButton
                    variant={tone}
                    autoFocus
                    onPress={() => {
                      onConfirm();
                      close();
                    }}
                  >
                    {confirmLabel}
                  </AriaButton>
                </div>
              </>
            )}
          </Dialog>
        </Modal>
      </ModalOverlay>
    </DialogTrigger>
  );
}

interface FormDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: string;
  /** Short text under the title. */
  description?: string;
  /** The form. It closes the dialog itself (`onOpenChange(false)`) after a successful submit. */
  children: ReactNode;
}

/**
 * Modal dialog for a form (react-aria), opened and closed by the page: focus moves into the first
 * field, Tab stays inside, Escape closes and focus returns to the button that opened it. A click
 * outside does not close it, so a half-filled form is not lost by accident.
 */
export function FormDialog({
  isOpen,
  onOpenChange,
  title,
  description,
  children,
}: FormDialogProps) {
  // A dialog opened with Enter counts as a "virtual" click for react-aria, which then focuses the
  // dialog itself and skips `autoFocus` of a field inside. The first field gets the focus here,
  // before the dialog's own effect looks at the focus.
  const focusFirstField = useCallback((root: HTMLDivElement | null) => {
    if (!root) return;
    const field =
      root.querySelector<HTMLElement>(
        'input:not([type="radio"]):not([type="checkbox"]), textarea, select',
      ) ??
      root.querySelector<HTMLElement>('input:checked') ??
      root.querySelector<HTMLElement>('input, button');
    field?.focus();
  }, []);

  return (
    <ModalOverlay className={styles.overlay} isOpen={isOpen} onOpenChange={onOpenChange}>
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog}>
          <div className={styles.head}>
            <Heading slot="title" className={styles.title}>
              {title}
            </Heading>
            {description && <p className={styles.body}>{description}</p>}
          </div>
          <div ref={focusFirstField} className={styles.content}>
            {children}
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
