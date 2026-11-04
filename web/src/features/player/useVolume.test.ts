import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
  PLAYER_PREFS_KEY,
  readPlayerPrefs,
  usePlayerPrefs,
  writePlayerPrefs,
} from './usePlayerPrefs';
import { useVolume } from './useVolume';

function mount(element = document.createElement('video')) {
  const media = { current: element };
  const hook = renderHook(() => useVolume(media));
  return { element, ...hook };
}

describe('usePlayerPrefs', () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => vi.restoreAllMocks());

  it('reads nothing from an empty or broken store', () => {
    expect(readPlayerPrefs()).toEqual({});
    localStorage.setItem(PLAYER_PREFS_KEY, '{not json');
    expect(readPlayerPrefs()).toEqual({});
    localStorage.setItem(PLAYER_PREFS_KEY, JSON.stringify({ volume: 'loud', timeDisplay: 'x' }));
    expect(readPlayerPrefs()).toEqual({});
  });

  it('merges a change into what is stored and keeps the volume in 0 … 1', () => {
    writePlayerPrefs({ volume: 0.4 });
    writePlayerPrefs({ timeDisplay: 'remaining' });
    expect(readPlayerPrefs()).toEqual({ volume: 0.4, timeDisplay: 'remaining' });
    localStorage.setItem(PLAYER_PREFS_KEY, JSON.stringify({ volume: 3 }));
    expect(readPlayerPrefs()).toEqual({ volume: 1 });
  });

  it('works without storage: the state changes, nothing throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('blocked');
    });
    const { result } = renderHook(() => usePlayerPrefs());
    expect(result.current[0]).toEqual({});
    act(() => result.current[1]({ timeDisplay: 'remaining' }));
    expect(result.current[0]).toEqual({ timeDisplay: 'remaining' });
  });
});

describe('useVolume (V-3, F-2)', () => {
  beforeEach(() => localStorage.clear());

  it('starts at 100 % without a stored level and applies it to the element', () => {
    const { result, element } = mount();
    expect(result.current.volume).toBe(1);
    expect(element.volume).toBe(1);
    expect(element.muted).toBe(false);
  });

  it('starts at the level stored on this device and stores every change', () => {
    writePlayerPrefs({ volume: 0.35 });
    const { result, element } = mount();
    expect(result.current.volume).toBe(0.35);
    expect(element.volume).toBe(0.35);
    act(() => {
      result.current.setVolume(0.6);
    });
    expect(element.volume).toBe(0.6);
    expect(readPlayerPrefs().volume).toBe(0.6);
  });

  it('steps by the given delta within 0 … 100 % without float noise', () => {
    writePlayerPrefs({ volume: 0.1 });
    const { result } = mount();
    let level = 0;
    act(() => {
      level = result.current.step(0.05);
    });
    expect(level).toBe(0.15);
    act(() => {
      result.current.step(-0.05);
      result.current.step(-0.05);
      level = result.current.step(-0.05);
    });
    expect(level).toBe(0);
    act(() => {
      result.current.setVolume(0.98);
      level = result.current.step(0.05);
    });
    expect(level).toBe(1);
  });

  it('mutes without losing the level; an arrow key unmutes from the old level, not from 0', () => {
    writePlayerPrefs({ volume: 0.5 });
    const { result, element } = mount();
    act(() => {
      expect(result.current.toggleMute()).toBe(true);
    });
    expect(result.current.muted).toBe(true);
    expect(element.muted).toBe(true);
    expect(result.current.volume).toBe(0.5);
    act(() => {
      result.current.step(0.05);
    });
    expect(result.current.muted).toBe(false);
    expect(result.current.volume).toBe(0.55);
    expect(element.muted).toBe(false);
    expect(element.volume).toBe(0.55);
    act(() => {
      result.current.toggleMute();
    });
    act(() => {
      result.current.step(-0.05);
    });
    expect(result.current.volume).toBe(0.5);
    expect(result.current.muted).toBe(false);
  });

  it('does not remember a mute: the next player starts audible', () => {
    const first = mount();
    act(() => {
      first.result.current.toggleMute();
    });
    first.unmount();
    const second = mount();
    expect(second.result.current.muted).toBe(false);
  });
});
