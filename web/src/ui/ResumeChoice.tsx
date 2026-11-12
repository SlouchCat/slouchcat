import { Dialog, Heading, Modal, ModalOverlay, type KeyboardEvent } from 'react-aria-components';
import { AriaButton } from './AriaButton';
import dialogStyles from './Dialog.module.css';
import { focusOnMount } from './focusOnMount';
import styles from './ResumeChoice.module.css';
import { useSpatialHold } from './spatial';

interface ResumeChoiceProps {
  isOpen: boolean;
  /** Escape, a click beside the dialog and every button close it through this. */
  onOpenChange: (isOpen: boolean) => void;
  /** The title that was started ("S01E04 · Die lange Nacht"). */
  title: string;
  /** One sentence under the title ("Du hast diese Folge angefangen."). */
  body: string;
  /** The default, which has the focus: "Weiterschauen ab 21:36". */
  resumeLabel: string;
  restartLabel: string;
  cancelLabel: string;
  onResume: () => void;
  onRestart: () => void;
}

/**
 * ↓ and ↑ move the focus to the next and the previous button, without wrapping. Every other key
 * goes on to the dialog (Escape) and the focus scope (Tab).
 */
function moveFocus(event: KeyboardEvent) {
  const step = event.key === 'ArrowDown' ? 1 : event.key === 'ArrowUp' ? -1 : 0;
  if (step === 0 || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) {
    event.continuePropagation();
    return;
  }
  event.preventDefault();
  const button = event.target as HTMLElement;
  const buttons = [...(button.parentElement?.querySelectorAll('button') ?? [])];
  const at = buttons.indexOf(button as HTMLButtonElement);
  buttons[Math.min(Math.max(at + step, 0), buttons.length - 1)]?.focus();
}

/**
 * The question before a started title plays from a single click (concept 20 P-4): resume where it
 * stopped (the default, focused), start over, or cancel. Escape and «Abbrechen» do nothing else,
 * and the focus goes back to where it was: the row or button that asked. The page's arrow-key
 * navigation holds while it is open; ↑ and ↓ move between its buttons only.
 */
export function ResumeChoice({
  isOpen,
  onOpenChange,
  title,
  body,
  resumeLabel,
  restartLabel,
  cancelLabel,
  onResume,
  onRestart,
}: ResumeChoiceProps) {
  useSpatialHold(isOpen);
  return (
    <ModalOverlay
      className={dialogStyles.overlay}
      isDismissable
      isOpen={isOpen}
      onOpenChange={onOpenChange}
    >
      <Modal className={dialogStyles.modal}>
        <Dialog className={dialogStyles.dialog}>
          {({ close }) => (
            <>
              <Heading slot="title" className={dialogStyles.title}>
                {title}
              </Heading>
              <p className={dialogStyles.body}>{body}</p>
              <div className={styles.choices}>
                <AriaButton
                  variant="primary"
                  size="lg"
                  icon="play"
                  ref={focusOnMount}
                  onKeyDown={moveFocus}
                  onPress={() => {
                    close();
                    onResume();
                  }}
                >
                  {resumeLabel}
                </AriaButton>
                <AriaButton
                  size="lg"
                  icon="replay"
                  onKeyDown={moveFocus}
                  onPress={() => {
                    close();
                    onRestart();
                  }}
                >
                  {restartLabel}
                </AriaButton>
                <AriaButton variant="ghost" onKeyDown={moveFocus} onPress={close}>
                  {cancelLabel}
                </AriaButton>
              </div>
            </>
          )}
        </Dialog>
      </Modal>
    </ModalOverlay>
  );
}
