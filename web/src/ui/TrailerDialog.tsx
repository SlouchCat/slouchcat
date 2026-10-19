import type { ReactNode } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import dialogStyles from './Dialog.module.css';
import { RadioGroup } from './RadioGroup';
import styles from './TrailerDialog.module.css';

interface TrailerDialogProps {
  isOpen: boolean;
  onOpenChange: (isOpen: boolean) => void;
  title: string;
  closeLabel: string;
  languageLabel: string;
  /** Languages of the trailers, the profile language first; the chooser shows with two or more. */
  languages: readonly { value: string; label: string }[];
  language: string;
  onLanguageChange: (language: string) => void;
  /** `youtube-nocookie.com` embed URL of the chosen trailer; nothing is embedded without it. */
  embedUrl?: string;
  /** Accessible name of the frame. */
  frameTitle: string;
  /** Line under the frame (where the video comes from). */
  note?: string;
  /** Loading or error state in place of the frame. */
  children?: ReactNode;
}

/**
 * A trailer in the YouTube IFrame player (W-10, B-8) inside a modal (react-aria): focus moves into
 * the dialog and stays there, Escape or a click outside closes it and focus returns to the button
 * that opened it. The video comes from youtube-nocookie.com; the server never downloads it.
 */
export function TrailerDialog({
  isOpen,
  onOpenChange,
  title,
  closeLabel,
  languageLabel,
  languages,
  language,
  onLanguageChange,
  embedUrl,
  frameTitle,
  note,
  children,
}: TrailerDialogProps) {
  return (
    <ModalOverlay
      className={dialogStyles.overlay}
      isOpen={isOpen}
      onOpenChange={onOpenChange}
      isDismissable
    >
      <Modal className={`${dialogStyles.modal} ${styles.modal}`}>
        <Dialog className={dialogStyles.dialog}>
          <div className={styles.head}>
            <Heading slot="title" className={dialogStyles.title}>
              {title}
            </Heading>
            <AriaButton variant="ghost" onPress={() => onOpenChange(false)}>
              {closeLabel}
            </AriaButton>
          </div>
          {languages.length > 1 && (
            <RadioGroup
              label={languageLabel}
              options={languages}
              value={language}
              onChange={onLanguageChange}
            />
          )}
          <div className={styles.frame}>
            {embedUrl ? (
              <iframe
                key={embedUrl}
                className={styles.iframe}
                src={embedUrl}
                title={frameTitle}
                allow="autoplay; encrypted-media; picture-in-picture"
                referrerPolicy="strict-origin-when-cross-origin"
                allowFullScreen
              />
            ) : (
              <div className={styles.status}>{children}</div>
            )}
          </div>
          {note && <p className={styles.note}>{note}</p>}
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
