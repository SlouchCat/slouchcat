import { afterEach, describe, expect, it, vi } from 'vitest';
import { isTypedSpeedKey, playerKeyAction, stepPlaybackRate } from './keymap';

const press = (key: string, extra: Partial<KeyboardEvent> = {}) =>
  playerKeyAction({
    key,
    shiftKey: false,
    altKey: false,
    ctrlKey: false,
    metaKey: false,
    ...extra,
  });

describe('playerKeyAction (concept 13, 11.2)', () => {
  it('maps the keys of the table', () => {
    expect(press(' ')).toEqual({ kind: 'playPause', activates: true });
    expect(press('Enter')).toEqual({ kind: 'playPause', activates: true });
    expect(press('k')).toEqual({ kind: 'playPause', activates: false });
    expect(press('m')).toEqual({ kind: 'mute' });
    expect(press('t')).toEqual({ kind: 'tracks' });
    expect(press('u')).toEqual({ kind: 'subtitles' });
    expect(press('c')).toEqual({ kind: 'chapters' });
    expect(press('q')).toEqual({ kind: 'quality' });
    expect(press('[')).toEqual({ kind: 'rate', delta: -0.25 });
    expect(press(']')).toEqual({ kind: 'rate', delta: 0.25 });
    expect(press('f')).toEqual({ kind: 'fullscreen' });
    expect(press('i')).toEqual({ kind: 'info' });
    expect(press('s')).toEqual({ kind: 'skipIntro' });
    expect(press('n')).toEqual({ kind: 'nextEpisode' });
    expect(press('?')).toEqual({ kind: 'help' });
    expect(press('Escape')).toEqual({ kind: 'back' });
    expect(press('Backspace')).toEqual({ kind: 'back' });
  });

  it('counts letters in either case (Shift, Caps Lock)', () => {
    expect(press('M', { shiftKey: true })).toEqual({ kind: 'mute' });
    expect(press('T')).toEqual({ kind: 'tracks' });
  });

  it('jumps to 0–90 % with the digits', () => {
    expect(press('0')).toEqual({ kind: 'jump', fraction: 0 });
    expect(press('5')).toEqual({ kind: 'jump', fraction: 0.5 });
    expect(press('9')).toEqual({ kind: 'jump', fraction: 0.9 });
  });

  it('seeks 10 s with ←/→, a chapter with Shift, and changes the volume by 5 % with ↑/↓', () => {
    expect(press('ArrowLeft')).toEqual({ kind: 'seek', seconds: -10 });
    expect(press('ArrowRight')).toEqual({ kind: 'seek', seconds: 10 });
    expect(press('ArrowLeft', { shiftKey: true })).toEqual({ kind: 'chapter', step: -1 });
    expect(press('ArrowRight', { shiftKey: true })).toEqual({ kind: 'chapter', step: 1 });
    expect(press('ArrowUp')).toEqual({ kind: 'volume', delta: 0.05 });
    expect(press('ArrowDown')).toEqual({ kind: 'volume', delta: -0.05 });
    expect(press('ArrowUp', { shiftKey: true })).toBeNull();
  });

  it('leaves keys with Alt, Ctrl or Meta and unknown keys to the browser', () => {
    expect(press('f', { ctrlKey: true })).toBeNull();
    expect(press('ArrowLeft', { altKey: true })).toBeNull();
    expect(press('r', { metaKey: true })).toBeNull();
    expect(press('[', { ctrlKey: true })).toBeNull();
    expect(press(']', { metaKey: true })).toBeNull();
    expect(press('x')).toBeNull();
    expect(press('Tab')).toBeNull();
  });

  describe('[ and ] typed with AltGr or Option (Swiss and German layouts)', () => {
    const altGraph = (on: boolean) => (key: string) => on && key === 'AltGraph';
    afterEach(() => vi.restoreAllMocks());

    it('counts AltGr, which Windows reports as Ctrl+Alt', () => {
      const typed = { ctrlKey: true, altKey: true, getModifierState: altGraph(true) };
      expect(press('[', typed)).toEqual({ kind: 'rate', delta: -0.25 });
      expect(press(']', typed)).toEqual({ kind: 'rate', delta: 0.25 });
      // Ctrl+Alt without AltGr is a shortcut.
      expect(press('[', { ...typed, getModifierState: altGraph(false) })).toBeNull();
    });

    it('counts Option on a Mac, but not Alt elsewhere', () => {
      const userAgent = vi.spyOn(navigator, 'userAgent', 'get');
      userAgent.mockReturnValue('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)');
      expect(press('[', { altKey: true })).toEqual({ kind: 'rate', delta: -0.25 });
      expect(press(']', { altKey: true, ctrlKey: true })).toBeNull();
      userAgent.mockReturnValue('Mozilla/5.0 (X11; Linux x86_64)');
      expect(press('[', { altKey: true })).toBeNull();
    });

    it('is only about [ and ]', () => {
      expect(
        isTypedSpeedKey({
          key: 'q',
          shiftKey: false,
          altKey: true,
          ctrlKey: true,
          metaKey: false,
          getModifierState: altGraph(true),
        }),
      ).toBe(false);
    });
  });
});

describe('stepPlaybackRate (V-7)', () => {
  it('steps by 0.25 within 0.5× … 2×', () => {
    expect(stepPlaybackRate(1, 0.25)).toBe(1.25);
    expect(stepPlaybackRate(1, -0.25)).toBe(0.75);
    expect(stepPlaybackRate(2, 0.25)).toBe(2);
    expect(stepPlaybackRate(0.5, -0.25)).toBe(0.5);
  });

  it('lands on the grid from an odd rate (set elsewhere)', () => {
    expect(stepPlaybackRate(1.1, 0.25)).toBe(1.25);
    expect(stepPlaybackRate(3, -0.25)).toBe(2);
  });
});
