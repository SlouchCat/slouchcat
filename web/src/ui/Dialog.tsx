import {
  useCallback,
  useEffect,
  useRef,
  type ReactElement,
  type ReactNode,
  type RefObject,
} from 'react';
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
  /** Told when the dialog opens and closes (a page pauses its polling meanwhile). */
  onOpenChange?: (isOpen: boolean) => void;
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
  onOpenChange,
}: ConfirmDialogProps) {
  return (
    <DialogTrigger onOpenChange={onOpenChange}>
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
  /**
   * Focused when the dialog opens, instead of the first field: a CSS selector inside the dialog,
   * or a ref. A dialog whose content is a list has no field for `focusFirstField` to find.
   */
  initialFocus?: string | RefObject<HTMLElement | null>;
  /**
   * Focused after the dialog closed, instead of the button that opened it. react-aria returns
   * the focus to the opener, which is right for Escape and Cancel; a dialog that wrote something
   * into a field wants the focus in that field, so the user can correct it without tabbing back.
   * The holder decides per close by setting the ref before it closes the dialog, and clearing it
   * when the focus should go back to the opener.
   */
  returnFocus?: RefObject<HTMLElement | null>;
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
  initialFocus,
  returnFocus,
  children,
}: FormDialogProps) {
  // A dialog opened with Enter counts as a "virtual" click for react-aria, which then focuses the
  // dialog itself and skips `autoFocus` of a field inside. The first field gets the focus here,
  // before the dialog's own effect looks at the focus.
  const focusFirstField = useCallback(
    (root: HTMLDivElement | null) => {
      if (!root) return;
      if (initialFocus) {
        const wanted =
          typeof initialFocus === 'string'
            ? root.querySelector<HTMLElement>(initialFocus)
            : initialFocus.current;
        if (wanted) {
          wanted.focus();
          return;
        }
      }
      // The first field in document order: a text field, or the checked radio of a group that
      // comes first (a dialog may start with radios and have text fields further down).
      const field =
        root.querySelector<HTMLElement>(
          'input:not([type="radio"]):not([type="checkbox"]):not([type="hidden"]), textarea, select, input:checked',
        ) ?? root.querySelector<HTMLElement>('input, button');
      field?.focus();
    },
    [initialFocus],
  );

  // react-aria restores the focus to the opener when the overlay unmounts; the next frame takes
  // it over to the element the holder named for this close.
  const wasOpen = useRef(false);
  useEffect(() => {
    if (isOpen) {
      wasOpen.current = true;
      return;
    }
    if (!wasOpen.current) return;
    wasOpen.current = false;
    const target = returnFocus?.current;
    if (!target) return;
    const frame = requestAnimationFrame(() => target.focus());
    return () => cancelAnimationFrame(frame);
  }, [isOpen, returnFocus]);

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
