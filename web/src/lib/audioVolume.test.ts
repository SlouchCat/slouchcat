import { act, renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PLAYER_PREFS_KEY } from '@/features/player/usePlayerPrefs';
import { AUDIO_VOLUME_KEY, readAudioVolume, useAudioVolume, writeAudioVolume } from './audioVolume';

afterEach(() => {
  vi.restoreAllMocks();
  writeAudioVolume(1);
  localStorage.clear();
});

describe('the shared volume of everything heard (concept 19 E3, OE-1)', () => {
  it('starts at 100 % and does not take the video player’s value', () => {
    localStorage.setItem(PLAYER_PREFS_KEY, JSON.stringify({ volume: 0.3 }));
    expect(readAudioVolume()).toBe(1);
  });

  it('stores one value per device under its own key, clamped to 0 … 1', () => {
    expect(writeAudioVolume(0.4)).toBe(0.4);
    expect(localStorage.getItem(AUDIO_VOLUME_KEY)).toBe('0.4');
    expect(readAudioVolume()).toBe(0.4);
    expect(writeAudioVolume(1.3)).toBe(1);
    expect(writeAudioVolume(-0.2)).toBe(0);
    expect(writeAudioVolume(0.1 + 0.05)).toBe(0.15);
    // The video player keeps its own value.
    expect(localStorage.getItem(PLAYER_PREFS_KEY)).toBeNull();
  });

  it('reads junk as 100 %', () => {
    for (const junk of ['', 'loud', 'NaN', '{}']) {
      localStorage.setItem(AUDIO_VOLUME_KEY, junk);
      expect(readAudioVolume()).toBe(1);
    }
    localStorage.setItem(AUDIO_VOLUME_KEY, '7');
    expect(readAudioVolume()).toBe(1);
  });

  it('without storage (a private window) starts at 100 % and keeps a value for the visit', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new DOMException('denied', 'SecurityError');
    });
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new DOMException('denied', 'SecurityError');
    });
    expect(readAudioVolume()).toBe(1);
    expect(writeAudioVolume(0.25)).toBe(0.25);
    expect(readAudioVolume()).toBe(0.25);
  });

  it('re-renders its users on a change in this tab and in another', () => {
    const { result } = renderHook(() => useAudioVolume());
    expect(result.current).toBe(1);
    act(() => {
      writeAudioVolume(0.4);
    });
    expect(result.current).toBe(0.4);
    // Another tab stored 60 %.
    act(() => {
      localStorage.setItem(AUDIO_VOLUME_KEY, '0.6');
      window.dispatchEvent(new StorageEvent('storage', { key: AUDIO_VOLUME_KEY }));
    });
    expect(result.current).toBe(0.6);
  });
});
