import type { ReactNode } from 'react';
import { Dialog, Heading, Modal, ModalOverlay } from 'react-aria-components';
import { useDensityPortalProps } from './adminDensity';
import { AriaButton } from './AriaButton';
import styles from './EditorDialog.module.css';

interface EditorDialogProps {
  /** Escape and «Schliessen» call it; the holder unmounts the dialog. */
  onClose: () => void;
  /** The heading of the dialog (the title being edited). */
  title: string;
  /** The line under the heading: kind, library. */
  subtitle?: ReactNode;
  /** The small picture before the heading (a poster or a still), decorative. */
  imageUrl?: string;
  /** Shape of that picture. */
  imageShape?: 'portrait' | 'landscape';
  /** A link next to the heading («In der Verwaltung öffnen»). */
  link?: ReactNode;
  closeLabel: string;
  /** The body; it scrolls inside the dialog (tabs and their panels). */
  children: ReactNode;
}

/**
 * A large modal editor over a page (react-aria; the metadata editor of concept 15 7.4): up to
 * 1100 px wide and 90 % of the window high with its own scroll, the whole screen on phones. The
 * head carries a picture, the heading, a line and a link, and «Schliessen». Focus moves into the
 * dialog and stays there; Escape or «Schliessen» closes it and the focus returns to the button
 * that opened it. A click outside does not close it, so a half-done edit is not lost by accident.
 */
export function EditorDialog({
  onClose,
  title,
  subtitle,
  imageUrl,
  imageShape = 'portrait',
  link,
  closeLabel,
  children,
}: EditorDialogProps) {
  const density = useDensityPortalProps();
  return (
    <ModalOverlay
      className={styles.overlay}
      {...density}
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Modal className={styles.modal}>
        <Dialog className={styles.dialog}>
          <div className={styles.head}>
            <span className={styles.thumb} data-shape={imageShape} aria-hidden="true">
              {imageUrl && <img src={imageUrl} alt="" referrerPolicy="no-referrer" />}
            </span>
            <div className={styles.heading}>
              <Heading slot="title" className={styles.title}>
                {title}
              </Heading>
              {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
              {link}
            </div>
            <AriaButton variant="ghost" icon="close" onPress={onClose}>
              {closeLabel}
            </AriaButton>
          </div>
          <div className={styles.body}>{children}</div>
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
