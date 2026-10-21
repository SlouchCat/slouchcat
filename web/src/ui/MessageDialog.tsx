import { useEffect, type ReactNode } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import dialogStyles from './Dialog.module.css';
import styles from './MessageDialog.module.css';

interface MessageDialogProps {
  isOpen: boolean;
  /** Escape, Backspace (a remote's back key) and the dismiss button: closed, not confirmed. */
  onDismiss: () => void;
  /** Small label above the title ("Ankündigung"). */
  kicker?: string;
  title: string;
  /** Line under the title (a date). */
  meta?: string;
  /** Plain text: a blank line starts a paragraph, single line breaks are kept. */
  body: string;
  /** Polite status under the text, announced when it changes ("2 von 3"). */
  status?: string;
  /** An error of the confirm action. */
  error?: ReactNode;
  dismissLabel: string;
  confirmLabel: string;
  onConfirm: () => void;
  isPending?: boolean;
}

/**
 * A message the viewer confirms (react-aria modal): the confirm button has the focus, Tab stays
 * inside, Escape, Backspace and the dismiss button close it without confirming, and focus returns
 * to where it was. A click outside does nothing, so the message is not lost by accident.
 */
export function MessageDialog({
  isOpen,
  onDismiss,
  kicker,
  title,
  meta,
  body,
  status,
  error,
  dismissLabel,
  confirmLabel,
  onConfirm,
  isPending,
}: MessageDialogProps) {
  // react-aria closes on Escape only. Backspace is the back key of a remote (C-4); the capture
  // phase keeps the page behind from going back as well.
  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Backspace' || event.altKey || event.ctrlKey || event.metaKey) return;
      event.preventDefault();
      onDismiss();
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    return () => window.removeEventListener('keydown', onKeyDown, { capture: true });
  }, [isOpen, onDismiss]);

  const paragraphs = body
    .split(/\n\s*\n/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean);

  return (
    <ModalOverlay
      className={dialogStyles.overlay}
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
    >
      <Modal className={dialogStyles.modal}>
        <Dialog className={dialogStyles.dialog}>
          <div className={dialogStyles.head}>
            {kicker && <p className={styles.kicker}>{kicker}</p>}
            <Heading slot="title" className={dialogStyles.title}>
              {title}
            </Heading>
            {meta && <p className={styles.meta}>{meta}</p>}
          </div>
          <div className={styles.text}>
            {paragraphs.map((paragraph, index) => (
              <p key={index}>{paragraph}</p>
            ))}
          </div>
          {error}
          {status !== undefined && (
            <p role="status" className={styles.meta}>
              {status}
            </p>
          )}
          <div className={dialogStyles.actions}>
            <AriaButton variant="ghost" onPress={onDismiss}>
              {dismissLabel}
            </AriaButton>
            <AriaButton variant="primary" autoFocus isPending={isPending} onPress={onConfirm}>
              {confirmLabel}
            </AriaButton>
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
