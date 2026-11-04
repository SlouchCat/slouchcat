import { useCallback, useId, useRef, type KeyboardEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { focusOnMount } from './focusOnMount';
import styles from './PlayerPanel.module.css';

/** The chosen row of a list or group inside the panel. */
const CHOSEN = '[aria-selected="true"], [aria-checked="true"], [aria-current="true"]';
const FOCUSABLE =
  'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), [tabindex]:not([tabindex="-1"]), [role="option"]';

/** Backspace in a text field deletes a character; everywhere else it is the remote's back key. */
const editable = (target: EventTarget) =>
  target instanceof HTMLElement &&
  (target.isContentEditable ||
    target instanceof HTMLTextAreaElement ||
    (target instanceof HTMLInputElement &&
      !['checkbox', 'radio', 'range', 'button'].includes(target.type)));

interface PlayerPanelProps {
  /** Heading of the panel and its spoken name: «Ton», «Untertitel», «Einstellungen». */
  title: string;
  /** Escape, Backspace or the close button. The page unmounts the panel. */
  onClose: () => void;
  children: ReactNode;
}

/**
 * A panel of the player (concept 13, 11.2): on the right over the video, a sheet from the bottom
 * below 768 px and on phones held sideways. The video keeps playing beside it.
 *
 * Mounted, it moves the focus to the chosen row (or to the first thing to focus) with
 * focusOnMount, not react-aria's autoFocus, which can wait 500 ms for a transition (11.13).
 * Escape and Backspace close it and give the focus back to whatever had it before (the button that
 * opened the panel); the keys go no further, so the player does not also leave the page. It is a
 * non-modal dialog: Tab walks out of it like out of any other part of the page, no focus trap.
 */
export function PlayerPanel({ title, onClose, children }: PlayerPanelProps) {
  const { t } = useTranslation();
  const headingId = useId();
  const opener = useRef<HTMLElement | null>(null);

  // Runs once, when the panel mounts (the callback never changes): the element that has the focus
  // now is the one to return to.
  const enter = useCallback((panel: HTMLElement | null) => {
    if (!panel) return;
    const active = document.activeElement;
    opener.current = active instanceof HTMLElement && active !== document.body ? active : null;
    const body = panel.querySelector<HTMLElement>('[data-panel-body]');
    focusOnMount(
      body?.querySelector<HTMLElement>(CHOSEN) ??
        body?.querySelector<HTMLElement>(FOCUSABLE) ??
        panel,
    );
  }, []);

  const close = () => {
    const target = opener.current;
    if (target?.isConnected) target.focus();
    onClose();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    const back = event.key === 'Escape' || (event.key === 'Backspace' && !editable(event.target));
    if (!back || event.altKey || event.ctrlKey || event.metaKey) return;
    event.preventDefault();
    event.stopPropagation();
    close();
  };

  return (
    // The keys of the rows bubble up to here; the panel itself is a dialog, not a control.
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <section
      ref={enter}
      className={styles.panel}
      role="dialog"
      aria-labelledby={headingId}
      tabIndex={-1}
      onKeyDown={onKeyDown}
    >
      <header className={styles.header}>
        <h2 id={headingId} className={styles.title}>
          {title}
        </h2>
        <AriaButton
          variant="ghost"
          icon="close"
          iconOnly
          aria-label={t('common.close')}
          onPress={close}
        />
      </header>
      <div className={styles.body} data-panel-body="">
        {children}
      </div>
    </section>
  );
}
