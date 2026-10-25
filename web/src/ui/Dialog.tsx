import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ReactElement,
  type ReactNode,
  type RefObject,
} from 'react';
import { Dialog, DialogTrigger, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import { TextField } from './TextField';
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

interface ConfirmInputDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: string;
  /** What the confirmation really does, in full sentences; several paragraphs are welcome. */
  children: ReactNode;
  /** The text that has to be typed, character for character. */
  expected: string;
  /** Label of the field ("Type the path to confirm"). */
  inputLabel: string;
  /** Under the field while what is typed is not `expected` yet. */
  mismatchText: string;
  confirmLabel: string;
  cancelLabel: string;
  /**
   * Gets what was typed, trimmed. The caller passes it on to the server, which compares it
   * against its own row: the gate then lives in one place instead of two, and a mistake in this
   * component cannot confirm anything by itself.
   */
  onConfirm: (typed: string) => void;
  isPending?: boolean;
}

/**
 * Confirmation that has to be typed out: the button stays disabled until the field repeats
 * `expected` exactly (idea 02, E12). ConfirmDialog is one keystroke away from done, which is
 * right for "accept the loss" and wrong for an action that takes thousands of titles out of a
 * library and deletes the offline packages on the devices.
 *
 * It slows a hand down; it stops nothing that means it. Anything that can send the request can
 * send the confirmation with it, and no server can tell whether a human typed it.
 *
 * It is an `alertdialog` like ConfirmDialog, opened and closed by the page (it needs to know
 * which row it asks about), and the focus starts in the field, not on the confirming button.
 */
export function ConfirmInputDialog({
  isOpen,
  onOpenChange,
  title,
  children,
  expected,
  inputLabel,
  mismatchText,
  confirmLabel,
  cancelLabel,
  onConfirm,
  isPending,
}: ConfirmInputDialogProps) {
  // A fresh field per opening and per row: a path left over from the last time would be a
  // confirmation nobody gave. Two things make that hold. The value is tied to what is being
  // confirmed, so switching rows empties it without an effect; and every way out clears it, so
  // reopening the *same* row starts empty too -- otherwise the administrator who typed the path,
  // read the consequences and pressed Escape would find the button already armed on his next
  // click, which is the very mis-aimed click this dialog exists for.
  const [entry, setEntry] = useState({ asked: '', typed: '' });
  const asked = isOpen ? expected : '';
  const typed = entry.asked === asked ? entry.typed : '';
  const matches = typed.trim() === expected;
  const clear = () => setEntry({ asked: '', typed: '' });
  const close = () => {
    clear();
    onOpenChange(false);
  };

  return (
    <ModalOverlay
      className={styles.overlay}
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (open) {
          onOpenChange(true);
          return;
        }
        close();
      }}
    >
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog} role="alertdialog">
          <div className={styles.head}>
            <Heading slot="title" className={styles.title}>
              {title}
            </Heading>
          </div>
          <div className={styles.body}>{children}</div>
          <TextField
            label={inputLabel}
            value={typed}
            onChange={(value) => setEntry({ asked, typed: value })}
            mono
            autoFocus
            autoComplete="off"
            spellCheck="false"
            description={matches ? undefined : mismatchText}
          />
          <div className={styles.actions}>
            <AriaButton variant="ghost" onPress={close}>
              {cancelLabel}
            </AriaButton>
            <AriaButton
              variant="danger"
              isDisabled={!matches}
              isPending={isPending}
              onPress={() => {
                const confirmed = typed.trim();
                clear();
                onConfirm(confirmed);
              }}
            >
              {confirmLabel}
            </AriaButton>
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
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
  /**
   * "wide" gives the dialog the room a list needs instead of the width of a form column: a
   * folder browser read through a letterbox is the slowest way to find a folder.
   */
  size?: 'regular' | 'wide';
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
  size = 'regular',
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
      <Modal className={`${styles.modal} ${size === 'wide' ? styles.wide : ''}`}>
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
