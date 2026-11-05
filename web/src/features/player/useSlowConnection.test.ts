import type { PlaybackInfo } from '@slouchcat/api-client';
import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { playback } from '@/test/fixtures';
import { volumeIsReadOnly } from './layout';
import { chapterProgress } from './tabs/playerTabs';
import { readPlayerPrefs, writePlayerPrefs } from './usePlayerPrefs';
import { lowerQuality, STALL_MS, STALL_WINDOW_MS, useSlowConnection } from './useSlowConnection';

const rung = (quality: '1080p' | '720p' | '480p' | '360p', height: number, available = true) => ({
  quality,
  height,
  available,
  current: false,
});

describe('lowerQuality (V-18)', () => {
  const info = (height: number | undefined, transcodeHeight?: number): PlaybackInfo => ({
    ...playback,
    mediaFile: { ...playback.mediaFile, height },
    transcoding: transcodeHeight
      ? { mode: 'transcode', hardware: true, height: transcodeHeight, reason: 'quality' }
      : undefined,
    qualities: [
      { quality: 'auto', available: true, current: true },
      rung('1080p', 1080),
      rung('720p', 720),
      rung('480p', 480, false),
      rung('360p', 360),
    ],
  });

  it('is the highest available rung below the picture playing', () => {
    expect(lowerQuality(info(1080))?.quality).toBe('720p');
    // A transcode to 720p: one step down skips the rung that is ruled out.
    expect(lowerQuality(info(1080, 720))?.quality).toBe('360p');
    expect(lowerQuality(info(360))).toBeUndefined();
    expect(lowerQuality(info(undefined))).toBeUndefined();
  });
});

describe('useSlowConnection (V-18)', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  /** `started`: the element has played once already (its first `playing`). */
  function setup(enabled = true, started = true) {
    const media = { current: document.createElement('video') };
    let seeking = false;
    Object.defineProperty(media.current, 'seeking', { get: () => seeking });
    const hook = renderHook(({ on }) => useSlowConnection(media, on), {
      initialProps: { on: enabled },
    });
    const fire = (type: string) =>
      act(() => {
        media.current.dispatchEvent(new Event(type));
      });
    if (started) fire('playing');
    const stall = (ms: number) => {
      fire('waiting');
      act(() => {
        vi.advanceTimersByTime(ms);
      });
      fire('playing');
    };
    return { hook, fire, stall, setSeeking: (value: boolean) => (seeking = value) };
  }

  it('two waits over a second within a minute make it slow; short ones do not count', () => {
    const { hook, stall } = setup();
    stall(STALL_MS - 100);
    stall(STALL_MS - 100);
    expect(hook.result.current.slow).toBe(false);
    stall(STALL_MS + 100);
    expect(hook.result.current.slow).toBe(false);
    stall(STALL_MS + 100);
    expect(hook.result.current.slow).toBe(true);
  });

  it('a wait before the first playing does not count (review step 5)', () => {
    const { hook, fire } = setup(true, false);
    // The start fills the buffer: a long `waiting` before the first `playing`.
    fire('waiting');
    act(() => {
      vi.advanceTimersByTime(STALL_MS * 3);
    });
    fire('playing');
    expect(hook.result.current.slow).toBe(false);
    // From then on stalls count: one more makes it slow only together with a second.
    fire('waiting');
    act(() => {
      vi.advanceTimersByTime(STALL_MS + 100);
    });
    fire('playing');
    expect(hook.result.current.slow).toBe(false);
    fire('waiting');
    act(() => {
      vi.advanceTimersByTime(STALL_MS + 100);
    });
    expect(hook.result.current.slow).toBe(true);
  });

  it('stalls more than a minute apart do not add up, nor do waits for a seek', () => {
    const { hook, stall, fire, setSeeking } = setup();
    stall(STALL_MS + 100);
    act(() => {
      vi.advanceTimersByTime(STALL_WINDOW_MS);
    });
    stall(STALL_MS + 100);
    expect(hook.result.current.slow).toBe(false);
    setSeeking(true);
    fire('waiting');
    act(() => {
      vi.advanceTimersByTime(STALL_MS * 3);
    });
    expect(hook.result.current.slow).toBe(false);
  });

  it('dismissed, the hint stays away; switched off, it shows nothing', () => {
    const { hook, stall } = setup();
    stall(STALL_MS + 100);
    stall(STALL_MS + 100);
    expect(hook.result.current.slow).toBe(true);
    act(() => hook.result.current.dismiss());
    stall(STALL_MS + 100);
    stall(STALL_MS + 100);
    expect(hook.result.current.slow).toBe(false);

    const off = setup(false);
    off.stall(STALL_MS + 100);
    off.stall(STALL_MS + 100);
    expect(off.hook.result.current.slow).toBe(false);
  });
});

describe('step 5 helpers', () => {
  it('the iPhone keeps the volume read-only; an iPad and others do not', () => {
    expect(
      volumeIsReadOnly(
        'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 Mobile/15E148',
      ),
    ).toBe(true);
    expect(volumeIsReadOnly('Mozilla/5.0 (iPad; CPU OS 19_0 like Mac OS X)')).toBe(false);
    expect(volumeIsReadOnly('Mozilla/5.0 (Linux; Android 16; Pixel 10) Chrome/140')).toBe(false);
  });

  it('a chapter is full behind the position, partly played around it, empty ahead', () => {
    const chapter = { startMs: 10_000, endMs: 20_000 };
    expect(chapterProgress(chapter, 5000)).toBeUndefined();
    // At its very start nothing of it has played: no «0 % gesehen» (review step 5).
    expect(chapterProgress(chapter, 10_000)).toBeUndefined();
    expect(chapterProgress(chapter, 15_000)).toBe(50);
    expect(chapterProgress(chapter, 25_000)).toBe(100);
  });

  it('V-17: the device remembers a rung of the quality panel and drops junk', () => {
    localStorage.clear();
    writePlayerPrefs({ quality: '720p' });
    expect(readPlayerPrefs().quality).toBe('720p');
    localStorage.setItem('slouchcat.player', JSON.stringify({ quality: '4k' }));
    expect(readPlayerPrefs().quality).toBeUndefined();
  });
});
