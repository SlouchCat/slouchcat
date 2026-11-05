import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { usePlayerKeys, type PlayerKeyActions } from './usePlayerKeys';

function actions(extra: Partial<PlayerKeyActions> = {}): PlayerKeyActions {
  return {
    togglePlay: vi.fn(),
    seekBy: vi.fn(),
    seekTo: vi.fn(),
    changeVolume: vi.fn(),
    changeRate: vi.fn(),
    toggleMute: vi.fn(),
    toggleFullscreen: vi.fn(),
    toggleHelp: vi.fn(),
    closeOpen: vi.fn(() => false),
    back: vi.fn(),
    ...extra,
  };
}

/** Presses a key on `target` and says whether the player took it (preventDefault). */
function press(key: string, target: EventTarget = document.body, init: KeyboardEventInit = {}) {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event.defaultPrevented;
}

describe('usePlayerKeys (11.2, shared with the offline player)', () => {
  afterEach(() => document.body.replaceChildren());

  it('runs the actions of the key map and leaves a key alone whose action is missing', () => {
    const map = actions({ openQuality: vi.fn() });
    renderHook(() =>
      usePlayerKeys({ hidden: false, durationMs: 600_000, onKey: vi.fn(), actions: map }),
    );
    expect(press('ArrowRight')).toBe(true);
    expect(map.seekBy).toHaveBeenCalledWith(10);
    expect(press('5')).toBe(true);
    expect(map.seekTo).toHaveBeenCalledWith(300_000);
    expect(press('q')).toBe(true);
    expect(map.openQuality).toHaveBeenCalled();
    // No chapters, no tracks to switch (offline): the keys stay the browser's.
    expect(press('ArrowRight', document.body, { shiftKey: true })).toBe(false);
    expect(press('t')).toBe(false);
    expect(press('c')).toBe(false);
  });

  it('Escape closes what is open first, then leaves the player', () => {
    let open = true;
    const map = actions({
      closeOpen: vi.fn(() => {
        const was = open;
        open = false;
        return was;
      }),
    });
    renderHook(() => usePlayerKeys({ hidden: false, durationMs: 0, onKey: vi.fn(), actions: map }));
    expect(press('Escape')).toBe(true);
    expect(map.back).not.toHaveBeenCalled();
    expect(press('Backspace')).toBe(true);
    expect(map.back).toHaveBeenCalledTimes(1);
  });

  it('reads the latest options without binding the listeners again', () => {
    const first = actions();
    const second = actions();
    const onKey = vi.fn();
    const hook = renderHook(
      ({ map }) => usePlayerKeys({ hidden: false, durationMs: 0, onKey, actions: map }),
      { initialProps: { map: first } },
    );
    hook.rerender({ map: second });
    press('m');
    expect(first.toggleMute).not.toHaveBeenCalled();
    expect(second.toggleMute).toHaveBeenCalled();
    expect(onKey).toHaveBeenCalledTimes(1);
  });

  it('with the controls hidden Enter on a button plays and pauses; during a countdown any key cancels it', () => {
    const button = document.createElement('button');
    document.body.append(button);
    const map = actions();
    const cancel = vi.fn();
    const hook = renderHook(
      ({ countdown }) =>
        usePlayerKeys({
          hidden: true,
          countdown: countdown
            ? { panel: { current: null }, cancel, playNext: vi.fn() }
            : undefined,
          durationMs: 0,
          onKey: vi.fn(),
          actions: map,
        }),
      { initialProps: { countdown: false } },
    );
    expect(press('Enter', button)).toBe(true);
    expect(map.togglePlay).toHaveBeenCalledTimes(1);
    hook.rerender({ countdown: true });
    expect(press('ArrowLeft')).toBe(true);
    expect(cancel).toHaveBeenCalled();
    expect(map.seekBy).not.toHaveBeenCalled();
  });

  it('while a navigation away loads, only Escape and Backspace act: back, not closing a tab', () => {
    const map = actions({ closeOpen: vi.fn(() => true), playNext: vi.fn() });
    const onKey = vi.fn();
    const cancel = vi.fn();
    renderHook(() =>
      usePlayerKeys({
        hidden: true,
        countdown: { panel: { current: null }, cancel, playNext: vi.fn() },
        durationMs: 600_000,
        onKey,
        leaving: () => true,
        actions: map,
      }),
    );
    expect(press('ArrowRight')).toBe(false);
    expect(press('n')).toBe(false);
    expect(press(' ')).toBe(false);
    expect(map.seekBy).not.toHaveBeenCalled();
    expect(map.togglePlay).not.toHaveBeenCalled();
    expect(cancel).not.toHaveBeenCalled();
    expect(onKey).not.toHaveBeenCalled();
    expect(press('Escape')).toBe(true);
    expect(press('Backspace')).toBe(true);
    expect(map.closeOpen).not.toHaveBeenCalled();
    expect(map.back).toHaveBeenCalledTimes(2);
  });
});
