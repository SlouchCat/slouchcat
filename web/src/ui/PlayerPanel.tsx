import { useCallback, useId, useRef, type KeyboardEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton } from './AriaButton';
import { focusOnMount } from './focusOnMount';
import styles from './PlayerPanel.module.css';

/** The chosen row of a list or a radio group inside the panel (not a switch or a checkbox). */
const CHOSEN = '[role="option"][aria-selected="true"], [role="radio"][aria-checked="true"]';
const FOCUSABLE =
  'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), [tabindex]:not([tabindex="-1"]), [role="option"]';
/** Keys that move inside the panel and must not reach the player's key map (±10 s, volume). */
const MOVES = new Set([
  'ArrowUp',
  'ArrowDown',
  'ArrowLeft',
  'ArrowRight',
  'Home',
  'End',
  'PageUp',
  'PageDown',
]);

/** Backspace in a text field deletes a character; everywhere else it is the remote's back key. */
const editable = (target: EventTarget) =>
  target instanceof HTMLElement &&
  (target.isContentEditable ||
    target instanceof HTMLTextAreaElement ||
    (target instanceof HTMLInputElement &&
      !['checkbox', 'radio', 'range', 'button'].includes(target.type)));

/**
 * The element to focus in the panel's body: the chosen row, else the first thing to focus. The
 * rows of a react-aria collection are not there yet when the content mounts: it builds them in a
 * hidden document first and renders them in a later pass (seen in Chromium; jsdom has them at
 * once). An empty list is therefore no target yet.
 */
function targetIn(body: HTMLElement): HTMLElement | null {
  const found =
    body.querySelector<HTMLElement>(CHOSEN) ?? body.querySelector<HTMLElement>(FOCUSABLE);
  const pending =
    found?.getAttribute('role') === 'listbox' && !found.querySelector('[role="option"]');
  return pending ? null : found;
}

interface PlayerPanelProps {
  /** Heading of the panel and its spoken name: «Ton», «Untertitel», «Einstellungen». */
  title: string;
  /** Escape, Backspace or the close button. The page unmounts the panel. */
  onClose: () => void;
  /**
   * A level up, for the pages inside the gear panel (quality, speed …): a back button in the head,
   * and Backspace goes there instead of closing. The page swaps the content.
   */
  onBack?: () => void;
  children: ReactNode;
}

/**
 * A panel of the player (concept 13, 11.2): on the right over the video, a sheet from the bottom
 * below 768 px and on phones held sideways. The video keeps playing beside it.
 *
 * Mounted, it moves the focus to the chosen row (or to the first thing to focus) with
 * focusOnMount, not react-aria's autoFocus, which can wait 500 ms for a transition (11.13). When
 * the page swaps the content (a level down or up) and the focused element goes with it, the focus
 * moves the same way into the new content.
 *
 * Escape and Backspace close it and give the focus back to whatever had it before (the button that
 * opened the panel); so does a close by the page while the focus is in the panel. The arrow keys,
 * Home, End, PageUp and PageDown stay in the panel, also at the end of a list: they never reach the
 * player's key map. It is a non-modal dialog: Tab walks out of it like out of any other part of
 * the page, no focus trap.
 */
export function PlayerPanel({ title, onClose, onBack, children }: PlayerPanelProps) {
  const { t } = useTranslation();
  const headingId = useId();
  const opener = useRef<HTMLElement | null>(null);
  // Whether the focus is in the panel: only then does a swap of the content move it.
  const inside = useRef(false);

  // Runs once, when the panel mounts (the callback never changes): the element that has the focus
  // now is the one to return to. Its cleanup runs when the panel goes, before it leaves the page.
  const enter = useCallback((panel: HTMLElement | null) => {
    if (!panel) return;
    const active = document.activeElement;
    opener.current = active instanceof HTMLElement && active !== document.body ? active : null;
    const content = panel.querySelector<HTMLElement>('[data-panel-body]');
    if (!content) return;
    const focusTarget = () => {
      const target = targetIn(content);
      if (target) focusOnMount(target);
      return target !== null;
    };
    // The first focus may have to wait for the rows; later, a change of the content moves the
    // focus only when the focused element went with it (the focus fell to the body).
    let waiting = !focusTarget();
    const observer = new MutationObserver(() => {
      const lost = document.activeElement === null || document.activeElement === document.body;
      if (!waiting && !(lost && inside.current)) return;
      if (focusTarget()) waiting = false;
    });
    observer.observe(content, { childList: true, subtree: true });
    return () => {
      observer.disconnect();
      // Closed by the page (a choice, a key of its own) with the focus inside: it would fall to the
      // body, so it goes back to the opener -- after the commit, and only if nothing took it.
      if (!panel.contains(document.activeElement)) return;
      const target = opener.current;
      queueMicrotask(() => {
        const lost = document.activeElement === null || document.activeElement === document.body;
        if (lost && target?.isConnected) target.focus();
      });
    };
  }, []);

  const close = () => {
    const target = opener.current;
    if (target?.isConnected) target.focus();
    onClose();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.altKey || event.ctrlKey || event.metaKey) return;
    // With Shift as well: Shift+←/→ would jump a chapter in the player.
    if (MOVES.has(event.key)) {
      // The list or the field has used the key already; the player must not see it.
      event.stopPropagation();
      return;
    }
    const backspace = event.key === 'Backspace' && !editable(event.target);
    if (event.key !== 'Escape' && !backspace) return;
    event.preventDefault();
    event.stopPropagation();
    if (backspace && onBack) onBack();
    else close();
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
      onFocus={() => {
        inside.current = true;
      }}
      onBlur={(event) => {
        const to = event.relatedTarget;
        if (to instanceof Node && !event.currentTarget.contains(to)) inside.current = false;
      }}
    >
      <header className={styles.header} data-back={onBack ? '' : undefined}>
        {onBack && (
          <AriaButton
            variant="ghost"
            icon="back"
            iconOnly
            aria-label={t('common.back')}
            onPress={onBack}
          />
        )}
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
