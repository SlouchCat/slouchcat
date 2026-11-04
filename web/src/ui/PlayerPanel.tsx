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

/** A list of rows: one stop for ↑/↓ between the parts of the panel, its rows move by themselves. */
const COMPOSITE = '[role="listbox"], [role="radiogroup"]';
const ROW = '[role="option"], [role="radio"]';
/** Controls that take no ↑/↓ themselves: ↑/↓ go on to the next part of the panel. */
const PLAIN = 'button, a[href], input[type="checkbox"]';

const usable = (element: HTMLElement) =>
  !element.matches(':disabled') &&
  element.getAttribute('aria-disabled') !== 'true' &&
  element.closest('[hidden], [inert]') === null;

/**
 * The stops of the panel's body for ↑/↓ in DOM order: a list counts once, any other control that
 * is in the Tab order on its own (a roving group shows only its stop).
 */
function stopsIn(body: HTMLElement): HTMLElement[] {
  const stops: HTMLElement[] = [];
  for (const element of body.querySelectorAll<HTMLElement>(`${COMPOSITE}, ${FOCUSABLE}`)) {
    const list = element.matches(COMPOSITE) ? element : element.closest<HTMLElement>(COMPOSITE);
    if (list) {
      if (!stops.includes(list) && list.querySelector(ROW)) stops.push(list);
    } else if (element.tabIndex >= 0 && usable(element)) {
      stops.push(element);
    }
  }
  return stops;
}

/** The rows of a list that can take the focus. */
const rowsOf = (list: HTMLElement) =>
  [...list.querySelectorAll<HTMLElement>(ROW)].filter(
    (row) => usable(row) && row.closest(COMPOSITE) === list,
  );

/**
 * ↑/↓ from the end of a list, or from a button or switch, to the next part of the panel (11.2:
 * a remote has no Tab). Into a list the focus goes to its chosen row, else to its first row (↓)
 * or its last (↑). Returns whether the focus moved.
 */
function moveBetweenParts(body: HTMLElement, from: HTMLElement, down: boolean): boolean {
  const list = from.closest<HTMLElement>(COMPOSITE);
  if (list && body.contains(list)) {
    const rows = rowsOf(list);
    const at = rows.findIndex((row) => row === from || row.contains(from));
    // Inside the list the list moves the focus itself.
    if (at >= 0 && (down ? at < rows.length - 1 : at > 0)) return false;
  } else if (!from.matches(PLAIN)) {
    return false;
  }
  const stops = stopsIn(body);
  const index = stops.findIndex((stop) => stop === from || stop.contains(from));
  const next = stops[index + (down ? 1 : -1)];
  if (index < 0 || !next) return false;
  if (next.matches(COMPOSITE)) {
    const rows = rowsOf(next);
    const chosen = next.querySelector<HTMLElement>(CHOSEN);
    (chosen && usable(chosen) ? chosen : down ? rows[0] : rows.at(-1))?.focus();
  } else {
    next.focus();
  }
  return true;
}

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

/**
 * The opener of a panel that closed with the focus in it, for a panel that opens in the same
 * commit (T, U, Q or ? while another panel is open): the old panel's element had the focus and is
 * gone, so the new one takes over the button the old one was to give the focus back to. Cleared
 * after the commit's microtasks.
 */
let handedOver: HTMLElement | null = null;

interface PlayerPanelProps {
  /** Heading of the panel and its spoken name: «Ton», «Untertitel», «Einstellungen». */
  title: string;
  /** The id the button that opens it names in `aria-controls`. */
  id?: string;
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
export function PlayerPanel({ id, title, onClose, onBack, children }: PlayerPanelProps) {
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
    opener.current =
      active instanceof HTMLElement && active !== document.body && active.isConnected
        ? active
        : handedOver;
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
      handedOver = target;
      queueMicrotask(() => {
        handedOver = null;
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
      const body = event.currentTarget.querySelector<HTMLElement>('[data-panel-body]');
      const down = event.key === 'ArrowDown';
      if (
        (down || event.key === 'ArrowUp') &&
        !event.shiftKey &&
        event.target instanceof HTMLElement &&
        // A control that used the key itself marks it; a list at its end may mark it as well,
        // which moveBetweenParts checks by the rows.
        (!event.defaultPrevented || event.target.closest(COMPOSITE) !== null) &&
        body &&
        body.contains(event.target) &&
        moveBetweenParts(body, event.target, down)
      ) {
        event.preventDefault();
      }
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
      id={id}
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
