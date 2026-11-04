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
 * Sound and subtitles are panels beside the video (PlayerPanel), each with one list of rows (a
 * listbox named like the panel); their buttons name the language chosen: «Ton: Deutsch»,
 * «Untertitel: Aus». Quality, chapter and version are still react-aria menus behind a button, and
 * the menu is labelled by its button, which names the current pick: «Qualität: 480p».
 */
const controlName: Record<PlayerChoice, string | RegExp> = {
  audio: /^Ton: /,
  subtitle: /^Untertitel: /,
  quality: /^Qualität/,
  chapter: 'Kapitel',
  version: /^Version/,
};
const panelName: Partial<Record<PlayerChoice, string>> = { audio: 'Ton', subtitle: 'Untertitel' };

/** The open list of a choice: the panel's listbox, or the menu. */
function queryOpenList(choice: PlayerChoice): HTMLElement | null {
  const name = panelName[choice];
  if (name === undefined) return screen.queryByRole('menu', { name: controlName[choice] });
  const panel = screen.queryByRole('dialog', { name });
  return panel && within(panel).queryByRole('listbox', { name });
}

/** Rows of a panel's list are options, those of a menu menu items with a radio tick. */
const isPanelList = (from: HTMLElement) => from.getAttribute('role') === 'listbox';
const rowRole = (from: HTMLElement) => (isPanelList(from) ? 'option' : 'menuitemradio');
const chosenState = (from: HTMLElement) =>
  isPanelList(from) ? { selected: true } : { checked: true };

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

  /** The control that opens a choice; throws when the player offers none. */
  const control = (choice: PlayerChoice) =>
    screen.getByRole('button', { name: controlName[choice] });

  /** Waits until the player offers a choice (versions come with the title's details). */
  const findChoice = async (choice: PlayerChoice) => {
    await screen.findByRole('button', { name: controlName[choice] });
  };

  /** Whether the player offers a choice at all (the version only for two or more files). */
  const hasChoice = (choice: PlayerChoice) =>
    Promise.resolve(screen.queryByRole('button', { name: controlName[choice] }) !== null);

  /**
   * The current pick of a choice as the player shows it ("480p", "1080p"); waits until the player
   * offers the choice. Today the visible button names it after a colon.
   */
  const current = async (choice: PlayerChoice) => {
    const button = await screen.findByRole('button', { name: controlName[choice] });
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
    await user.click(control(choice));
    return findList(choice);
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
    choose,
    chooseAudioTrack: (name: string | RegExp) => choose('audio', name),
    chooseSubtitle: (name: string | RegExp) => choose('subtitle', name),
    chooseQuality: (name: string | RegExp) => choose('quality', name),
    chooseChapter: (name: string | RegExp) => choose('chapter', name),
    chooseVersion: (name: string | RegExp) => choose('version', name),
  };
}
