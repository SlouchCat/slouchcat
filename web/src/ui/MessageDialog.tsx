import { useEffect, type ReactNode } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { useDensityPortalProps } from './adminDensity';
import { AriaButton } from './AriaButton';
import { focusOnMount } from './focusOnMount';
import dialogStyles from './Dialog.module.css';
import styles from './MessageDialog.module.css';

interface MessageContentProps {
  /** Small label above the title ("Ankündigung"). */
  kicker?: string;
  title: string;
  /** Line under the title (a date). */
  meta?: string;
  /** Plain text: a blank line starts a paragraph, single line breaks are kept. */
  body: string;
  /**
   * Shown outside the dialog (a preview of it): the title is plain text, not the dialog's
   * heading, so the page's own headings stay in order.
   */
  preview?: boolean;
}

/**
 * The text of a message as the dialog shows it (kicker, title, date and paragraphs), without its
 * buttons: the dialog itself, and a page that previews it (concept 12, 11.3, «Ankündigungen»).
 */
export function MessageContent({
  kicker,
  title,
  meta,
  body,
  preview = false,
}: MessageContentProps) {
  const paragraphs = body
    .split(/\n\s*\n/)
    .map((paragraph) => paragraph.trim())
    .filter(Boolean);
  return (
    <>
      <div className={dialogStyles.head}>
        {kicker && <p className={styles.kicker}>{kicker}</p>}
        {preview ? (
          <p className={dialogStyles.title}>{title}</p>
        ) : (
          <Heading slot="title" className={dialogStyles.title}>
            {title}
          </Heading>
        )}
        {meta && <p className={styles.meta}>{meta}</p>}
      </div>
      <div className={styles.text}>
        {paragraphs.map((paragraph, index) => (
          <p key={index}>{paragraph}</p>
        ))}
      </div>
    </>
  );
}

interface MessageDialogProps extends Omit<MessageContentProps, 'preview'> {
  isOpen: boolean;
  /** Escape, Backspace (a remote's back key) and the dismiss button: closed, not confirmed. */
  onDismiss: () => void;
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
  // Today's size unless it opens from an administration page (ADR 0020); the viewers'
  // announcement dialog opens from the app shell, outside any AdminPage.
  const density = useDensityPortalProps();
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

  return (
    <ModalOverlay
      className={dialogStyles.overlay}
      {...density}
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onDismiss();
      }}
    >
      <Modal className={dialogStyles.modal}>
        <Dialog className={dialogStyles.dialog}>
          <MessageContent kicker={kicker} title={title} meta={meta} body={body} />
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
            <AriaButton
              variant="primary"
              ref={focusOnMount}
              isPending={isPending}
              onPress={onConfirm}
            >
              {confirmLabel}
            </AriaButton>
          </div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
