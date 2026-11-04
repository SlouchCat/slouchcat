import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, vi } from 'vitest';

/**
 * The choices the video player offers: audio track, subtitles, quality, chapter and version (B-5).
 * Concept 13 replaces today's menus («Spuren», «Qualität», «Kapitel», «Version») with panels for
 * sound and subtitles, a gear and a chapter tab (plan 11.2). The player tests speak in choices and
 * options only; this driver alone knows how the player shows them, so the rebuild changes the
 * driver and not the tests.
 */
export type PlayerChoice = 'audio' | 'subtitle' | 'quality' | 'chapter' | 'version';

/**
 * How the player shows each choice (concept 13, step 4):
 * - Sound and subtitles are panels beside the video (PlayerPanel) with one list of rows (a listbox
 *   named like the panel); their buttons name the language chosen: «Ton: Deutsch».
 * - Quality and version are pages of the gear panel («Einstellungen»): its first page has a row
 *   per page that names the current pick («Qualität», described «480p»).
 * - The chapters are still a react-aria menu behind the button «Kapitel» (a tab in step 5).
 */
type Shape =
  | { kind: 'panel'; control: RegExp; title: string }
  | { kind: 'gear'; row: string }
  | { kind: 'menu'; control: string };

const shapes: Record<PlayerChoice, Shape> = {
  audio: { kind: 'panel', control: /^Ton: /, title: 'Ton' },
  subtitle: { kind: 'panel', control: /^Untertitel: /, title: 'Untertitel' },
  quality: { kind: 'gear', row: 'Qualität' },
  version: { kind: 'gear', row: 'Version' },
  chapter: { kind: 'menu', control: 'Kapitel' },
};

const GEAR = 'Einstellungen';
/** The gear button names the rung after its name where there is a choice: «Einstellungen · 480p». */
const GEAR_BUTTON = /^Einstellungen( · .+)?$/;

/** The list of an open panel: the listbox named like the panel. */
function panelList(title: string): HTMLElement | null {
  const panel = screen.queryByRole('dialog', { name: title });
  return panel && within(panel).queryByRole('listbox', { name: title });
}

/** The open list of a choice: a panel's listbox, a gear page's listbox, or the menu. */
function queryOpenList(choice: PlayerChoice): HTMLElement | null {
  const shape = shapes[choice];
  if (shape.kind === 'menu') return screen.queryByRole('menu', { name: shape.control });
  return panelList(shape.kind === 'panel' ? shape.title : shape.row);
}

/** Rows of a panel's list are options, those of a menu menu items with a radio tick. */
const isPanelList = (from: HTMLElement) => from.getAttribute('role') === 'listbox';
const rowRole = (from: HTMLElement) => (isPanelList(from) ? 'option' : 'menuitemradio');
const chosenState = (from: HTMLElement) =>
  isPanelList(from) ? { selected: true } : { checked: true };

/** The text of the element an option is described by: the value of a gear row. */
function descriptionOf(element: HTMLElement): string {
  const ids = element.getAttribute('aria-describedby')?.split(/\s+/) ?? [];
  return ids.map((id) => document.getElementById(id)?.textContent ?? '').join(' ');
}

/**
 * Drives the video player's choices. `user` clicks through the fake timers of the player tests;
 * the tests may use it for their own clicks as well.
 */
export function playerDriver() {
  const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });

  /** The open list of a choice, or null. */
  const queryList = (choice: PlayerChoice) => queryOpenList(choice);

  /** The open list of a choice; throws while it is closed. */
  const list = (choice: PlayerChoice) => {
    const found = queryList(choice);
    if (!found) throw new Error(`the ${choice} choice is not open`);
    return found;
  };

  /** Waits until the list of a choice is open (after a shortcut key, say). */
  const findList = (choice: PlayerChoice) =>
    waitFor(() => {
      const found = queryOpenList(choice);
      if (!found) throw new Error(`the ${choice} choice is not open`);
      return found;
    });

  /** The control that opens a choice (the gear for quality and version); throws without one. */
  const control = (choice: PlayerChoice) => {
    const shape = shapes[choice];
    const name =
      shape.kind === 'gear' ? GEAR_BUTTON : shape.kind === 'panel' ? shape.control : shape.control;
    return screen.getByRole('button', { name });
  };

  /** Opens the first page of the gear panel and resolves with its list. */
  const openGear = async () => {
    const open = panelList(GEAR);
    if (open) return open;
    // A page of the gear is open: back to its first page; else open the gear.
    const page = screen.queryByRole('dialog', { name: /^(Qualität|Version|Geschwindigkeit)$/ });
    const back = page && within(page).queryByRole('button', { name: 'Zurück' });
    if (page && back) await user.click(back);
    else {
      if (page) fireEvent.keyDown(page, { key: 'Escape' });
      await user.click(screen.getByRole('button', { name: GEAR_BUTTON }));
    }
    return waitFor(() => {
      const found = panelList(GEAR);
      if (!found) throw new Error('the gear panel is not open');
      return found;
    });
  };

  /** Closes the gear panel with Escape. */
  const closeGear = async () => {
    const panel = screen.queryByRole('dialog', { name: GEAR });
    if (!panel) return;
    fireEvent.keyDown(document.activeElement ?? panel, { key: 'Escape' });
    if (screen.queryByRole('dialog', { name: GEAR })) fireEvent.keyDown(panel, { key: 'Escape' });
    await waitFor(() => expect(screen.queryByRole('dialog', { name: GEAR })).toBeNull());
  };

  /** The row of the gear's first page that leads to a choice, or null. */
  const gearRow = (gear: HTMLElement, row: string) =>
    within(gear).queryByRole('option', { name: row });

  /** Waits until the player offers a choice (versions come with the title's details). */
  const findChoice = async (choice: PlayerChoice) => {
    const shape = shapes[choice];
    if (shape.kind !== 'gear') {
      await screen.findByRole('button', { name: shape.control });
      return;
    }
    const gear = await openGear();
    await within(gear).findByRole('option', { name: shape.row });
    await closeGear();
  };

  /** Whether the player offers a choice at all (the version only for two or more files). */
  const hasChoice = async (choice: PlayerChoice) => {
    const shape = shapes[choice];
    if (shape.kind !== 'gear') {
      return screen.queryByRole('button', { name: shape.control }) !== null;
    }
    const gear = await openGear();
    const offered = gearRow(gear, shape.row) !== null;
    await closeGear();
    return offered;
  };

  /**
   * The current pick of a choice as the player shows it ("480p", "Deutsch"); waits until the
   * player offers the choice. A panel's button names it after a colon, a gear row as its value.
   */
  const current = async (choice: PlayerChoice) => {
    const shape = shapes[choice];
    if (shape.kind === 'gear') {
      const gear = await openGear();
      const row = await within(gear).findByRole('option', { name: shape.row });
      const value = descriptionOf(row);
      await closeGear();
      return value;
    }
    const button = await screen.findByRole('button', { name: shape.control });
    expect(button).toBeVisible();
    const name = button.getAttribute('aria-label') ?? '';
    const colon = name.indexOf(': ');
    if (colon < 0) throw new Error(`the ${choice} control shows no current pick: "${name}"`);
    return name.slice(colon + 2);
  };

  /** Opens a choice with its control, unless it is open already; resolves with its list. */
  const open = async (choice: PlayerChoice) => {
    const already = queryList(choice);
    if (already) return already;
    const shape = shapes[choice];
    if (shape.kind === 'gear') {
      const gear = await openGear();
      await user.click(await within(gear).findByRole('option', { name: shape.row }));
    } else {
      await user.click(control(choice));
    }
    return findList(choice);
  };

  /** Shows or hides the stream info through the switch on the gear's first page. */
  const toggleStreamInfo = async () => {
    const gear = await openGear();
    const panel = gear.closest('[role="dialog"]') as HTMLElement;
    await user.click(within(panel).getByRole('switch', { name: 'Stream-Info' }));
  };

  /** One option of an open list, by its spoken name. */
  const option = (from: HTMLElement, name: string | RegExp) =>
    within(from).getByRole(rowRole(from), { name });

  /** All options of an open list. */
  const options = (from: HTMLElement) => within(from).getAllByRole(rowRole(from));

  /** The one option of an open list that is the current pick; throws for none or several. */
  const chosen = (from: HTMLElement) => within(from).getByRole(rowRole(from), chosenState(from));

  /** Asserts that an option is shown but cannot be picked (a rung the source or limits rule out). */
  const expectUnavailable = (element: HTMLElement) => {
    if (element.hasAttribute('aria-disabled')) {
      expect(element).toHaveAttribute('aria-disabled', 'true');
    } else {
      expect(element).toBeDisabled();
    }
  };

  /** Asserts that an option can be picked. */
  const expectAvailable = (element: HTMLElement) => {
    expect(element).not.toHaveAttribute('aria-disabled', 'true');
    expect(element).toBeEnabled();
  };

  /** Opens a choice if needed and picks the option with that name once it shows. */
  const choose = async (choice: PlayerChoice, name: string | RegExp) => {
    const from = await open(choice);
    await user.click(await within(from).findByRole(rowRole(from), { name }));
  };

  /**
   * Presses a key inside an open list (Escape, Backspace), on its focused option or else its
   * current pick or first option.
   */
  const pressIn = (from: HTMLElement, key: string) => {
    const active = document.activeElement;
    const target =
      active instanceof HTMLElement && from.contains(active)
        ? active
        : (within(from).queryByRole(rowRole(from), chosenState(from)) ?? options(from)[0]!);
    fireEvent.keyDown(target, { key });
  };

  /** The open choice of any kind (a menu or a panel), or null when none is open. */
  const openChoice = () => screen.queryByRole('menu') ?? screen.queryByRole('dialog');

  return {
    user,
    control,
    findChoice,
    hasChoice,
    current,
    open,
    list,
    queryList,
    findList,
    option,
    options,
    chosen,
    expectUnavailable,
    expectAvailable,
    pressIn,
    openChoice,
    toggleStreamInfo,
    choose,
    chooseAudioTrack: (name: string | RegExp) => choose('audio', name),
    chooseSubtitle: (name: string | RegExp) => choose('subtitle', name),
    chooseQuality: (name: string | RegExp) => choose('quality', name),
    chooseChapter: (name: string | RegExp) => choose('chapter', name),
    chooseVersion: (name: string | RegExp) => choose('version', name),
  };
}
