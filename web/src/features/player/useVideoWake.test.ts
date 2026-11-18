import { renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useVideoWake, VIDEO_WAKE_CHECK_MS } from './useVideoWake';

function setVisibility(state: DocumentVisibilityState) {
  Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
  document.dispatchEvent(new Event('visibilitychange'));
}

interface FakeVideo {
  element: HTMLVideoElement;
  /** Seeks the hook made: assignments to `currentTime`. */
  seeks: number[];
  /** The media clock runs on by this many seconds. */
  run: (seconds: number) => void;
  /** The element shows a frame (calls the frame callbacks, counts it). */
  showFrame: () => void;
  set: (state: Partial<{ paused: boolean; ended: boolean; seeking: boolean }>) => void;
  /** Frame callbacks requested and neither called nor cancelled. */
  pendingFrameCallbacks: () => number;
}

function fakeVideo({ frameCallbacks = true } = {}): FakeVideo {
  let time = 10;
  let frames = 250;
  const state = { paused: false, ended: false, seeking: false };
  const seeks: number[] = [];
  const callbacks = new Map<number, () => void>();
  let nextHandle = 1;
  const element = {
    get paused() {
      return state.paused;
    },
    get ended() {
      return state.ended;
    },
    get seeking() {
      return state.seeking;
    },
    get currentTime() {
      return time;
    },
    set currentTime(value: number) {
      seeks.push(value);
      time = value;
    },
    getVideoPlaybackQuality: () => ({ totalVideoFrames: frames, droppedVideoFrames: 0 }),
    ...(frameCallbacks
      ? {
          requestVideoFrameCallback: (callback: () => void) => {
            const handle = nextHandle++;
            callbacks.set(handle, callback);
            return handle;
          },
          cancelVideoFrameCallback: (handle: number) => callbacks.delete(handle),
        }
      : {}),
  } as unknown as HTMLVideoElement;
  return {
    element,
    seeks,
    run: (seconds) => {
      time += seconds;
    },
    showFrame: () => {
      frames += 1;
      const pending = [...callbacks.values()];
      callbacks.clear();
      for (const callback of pending) callback();
    },
    set: (next) => Object.assign(state, next),
    pendingFrameCallbacks: () => callbacks.size,
  };
}

const wake = (video: FakeVideo) => renderHook(() => useVideoWake({ current: video.element }));

describe('useVideoWake', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    setVisibility('visible');
    vi.useRealTimers();
  });

  it('seeks to the position when the clock runs on without a new frame', () => {
    const video = fakeVideo();
    wake(video);
    setVisibility('hidden');
    setVisibility('visible');
    video.run(1.5);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS - 1);
    expect(video.seeks).toEqual([]);
    vi.advanceTimersByTime(1);
    expect(video.seeks).toEqual([11.5]);
  });

  it('leaves a video alone that shows frames again', () => {
    const video = fakeVideo();
    wake(video);
    setVisibility('hidden');
    setVisibility('visible');
    video.run(1.5);
    video.showFrame();
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);
  });

  it('leaves a video alone that waits for data (the clock stands as well)', () => {
    const video = fakeVideo();
    wake(video);
    setVisibility('hidden');
    setVisibility('visible');
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);
  });

  it.each([
    ['paused', { paused: true }],
    ['ended', { ended: true }],
    ['seeking', { seeking: true }],
  ])('does not check a video that is %s when the page returns', (_name, state) => {
    const video = fakeVideo();
    wake(video);
    video.set(state);
    setVisibility('hidden');
    setVisibility('visible');
    video.run(1.5);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);
  });

  it('gives up when the viewer pauses or the page hides again during the check', () => {
    const video = fakeVideo();
    wake(video);
    setVisibility('visible');
    video.run(1.5);
    video.set({ paused: true });
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);

    video.set({ paused: false });
    setVisibility('visible');
    video.run(1);
    setVisibility('hidden');
    video.run(1);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);
  });

  it('counts presented frames where the browser has no frame callback', () => {
    const stuck = fakeVideo({ frameCallbacks: false });
    const { unmount } = wake(stuck);
    setVisibility('visible');
    stuck.run(1.5);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(stuck.seeks).toEqual([11.5]);
    unmount();

    const running = fakeVideo({ frameCallbacks: false });
    wake(running);
    setVisibility('visible');
    running.run(1.5);
    running.showFrame();
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(running.seeks).toEqual([]);
  });

  it('leaves the old element alone when the player swapped its video during the check', () => {
    const first = fakeVideo();
    const second = fakeVideo();
    const ref = { current: first.element as HTMLVideoElement | null };
    renderHook(() => useVideoWake(ref));
    setVisibility('visible');
    ref.current = second.element;
    first.run(1.5);
    second.run(1.5);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(first.seeks).toEqual([]);
    expect(second.seeks).toEqual([]);
  });

  it('stops listening when the player unmounts', () => {
    const video = fakeVideo();
    const { unmount } = wake(video);
    setVisibility('visible');
    expect(video.pendingFrameCallbacks()).toBe(1);
    unmount();
    expect(video.pendingFrameCallbacks()).toBe(0);
    video.run(1.5);
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    setVisibility('visible');
    vi.advanceTimersByTime(VIDEO_WAKE_CHECK_MS);
    expect(video.seeks).toEqual([]);
  });
});
