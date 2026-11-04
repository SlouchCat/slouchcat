import { act, render, renderHook, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  activeCueTexts,
  shiftCues,
  SubtitleLayer,
  useSubtitleCues,
  type CueLike,
} from './SubtitleLayer';
import { DEFAULT_SUBTITLE_STYLE } from './useSubtitleStyle';

const cue = (startTime: number, endTime: number, text: string): CueLike => ({
  startTime,
  endTime,
  text,
});

/** A text track as the hook sees it: mode, cues and cuechange. */
class FakeTextTrack extends EventTarget {
  mode: TextTrackMode = 'disabled';
  constructor(public cues: CueLike[]) {
    super();
  }
}

/** A <track> element with its text track; `load` fires once the cues are there. */
class FakeTrackElement extends EventTarget {
  constructor(public track: FakeTextTrack) {
    super();
  }
}

function setup(cues: CueLike[]) {
  const video = document.createElement('video');
  let time = 0;
  Object.defineProperty(video, 'currentTime', { get: () => time, configurable: true });
  Object.defineProperty(video, 'paused', { get: () => true, configurable: true });
  const textTrack = new FakeTextTrack(cues);
  const element = new FakeTrackElement(textTrack);
  const refs = {
    video: { current: video },
    track: { current: element as unknown as HTMLTrackElement },
  };
  const at = (seconds: number) => {
    time = seconds;
    act(() => {
      video.dispatchEvent(new Event('timeupdate'));
    });
  };
  return { refs, textTrack, element, at };
}

describe('activeCueTexts and shiftCues (V-6)', () => {
  const cues = [cue(1, 3, 'Eins'), cue(2, 4, '<i>Zwei</i>'), cue(5, 6, '<v Anna>Drei')];

  it('names the cues that show at a time, without WebVTT markup', () => {
    expect(activeCueTexts(cues, 0.5)).toEqual([]);
    expect(activeCueTexts(cues, 2.5)).toEqual(['Eins', 'Zwei']);
    expect(activeCueTexts(cues, 3)).toEqual(['Zwei']);
    expect(activeCueTexts(cues, 5.5)).toEqual(['Drei']);
    expect(activeCueTexts(null, 2)).toEqual([]);
  });

  it('moves every cue and back', () => {
    const moved = [cue(1, 3, 'Eins')];
    shiftCues(moved, 0.4);
    expect(moved[0]).toMatchObject({ startTime: 1.4, endTime: 3.4 });
    shiftCues(moved, -0.4);
    expect(moved[0]!.startTime).toBeCloseTo(1);
    expect(moved[0]!.endTime).toBeCloseTo(3);
  });
});

describe('useSubtitleCues (11.10)', () => {
  it('keeps the track hidden and draws the cues at the time moved by the offset', () => {
    const { refs, textTrack, at } = setup([cue(10, 12, 'Hallo')]);
    const hook = renderHook(
      ({ offsetMs }) =>
        useSubtitleCues({ ...refs, src: '/s.vtt', offsetMs, pictureInPicture: false }),
      { initialProps: { offsetMs: 0 } },
    );
    expect(textTrack.mode).toBe('hidden');
    at(10.2);
    expect(hook.result.current).toEqual(['Hallo']);
    at(12.1);
    expect(hook.result.current).toEqual([]);

    // +0.5 s: the cue shows half a second later (V-6), and ends later.
    hook.rerender({ offsetMs: 500 });
    at(10.2);
    expect(hook.result.current).toEqual([]);
    at(10.6);
    expect(hook.result.current).toEqual(['Hallo']);
    at(12.4);
    expect(hook.result.current).toEqual(['Hallo']);
    // -0.3 s: earlier.
    hook.rerender({ offsetMs: -300 });
    at(9.8);
    expect(hook.result.current).toEqual(['Hallo']);
  });

  it('follows cuechange and the cues that load later', () => {
    const { refs, textTrack, element, at } = setup([]);
    const hook = renderHook(() =>
      useSubtitleCues({ ...refs, src: '/s.vtt', offsetMs: 0, pictureInPicture: false }),
    );
    at(1);
    expect(hook.result.current).toEqual([]);
    textTrack.cues = [cue(0.5, 2, 'Spät geladen')];
    act(() => {
      element.dispatchEvent(new Event('load'));
    });
    expect(hook.result.current).toEqual(['Spät geladen']);
    textTrack.cues = [cue(0.5, 2, 'Anders')];
    act(() => {
      textTrack.dispatchEvent(new Event('cuechange'));
    });
    expect(hook.result.current).toEqual(['Anders']);
  });

  it('in picture in picture shows the browser’s cues, moved by the offset, and puts them back', () => {
    const cues = [cue(10, 12, 'Hallo')];
    const { refs, textTrack, at } = setup(cues);
    const hook = renderHook(
      ({ pictureInPicture, offsetMs }) =>
        useSubtitleCues({ ...refs, src: '/s.vtt', offsetMs, pictureInPicture }),
      { initialProps: { pictureInPicture: false, offsetMs: 500 } },
    );
    at(10.6);
    expect(hook.result.current).toEqual(['Hallo']);

    hook.rerender({ pictureInPicture: true, offsetMs: 500 });
    expect(textTrack.mode).toBe('showing');
    expect(cues[0]).toMatchObject({ startTime: 10.5, endTime: 12.5 });
    // The layer draws nothing: the window shows the browser's cues.
    expect(hook.result.current).toEqual([]);
    // A new offset while the window is open moves the cues again from where they were.
    hook.rerender({ pictureInPicture: true, offsetMs: -200 });
    expect(cues[0]!.startTime).toBeCloseTo(9.8);

    hook.rerender({ pictureInPicture: false, offsetMs: -200 });
    expect(textTrack.mode).toBe('hidden');
    expect(cues[0]!.startTime).toBeCloseTo(10);
    expect(cues[0]!.endTime).toBeCloseTo(12);
    at(9.9);
    expect(hook.result.current).toEqual(['Hallo']);
  });

  it('draws nothing without a text subtitle', () => {
    const { refs } = setup([cue(0, 100, 'Hallo')]);
    const hook = renderHook(() =>
      useSubtitleCues({ ...refs, src: undefined, offsetMs: 0, pictureInPicture: false }),
    );
    expect(hook.result.current).toEqual([]);
  });
});

describe('SubtitleLayer', () => {
  it('draws the lines with the style of this device and moves up while the bar shows', () => {
    const { container, rerender } = render(
      <SubtitleLayer
        texts={['Erste Zeile', 'Zweite Zeile']}
        style={{ ...DEFAULT_SUBTITLE_STYLE, size: 'xl', color: 'yellow', background: 'box' }}
        raised={false}
      />,
    );
    const layer = container.querySelector('[data-subtitles]');
    expect(layer).toHaveAttribute('data-size', 'xl');
    expect(layer).toHaveAttribute('data-color', 'yellow');
    expect(layer).toHaveAttribute('data-background', 'box');
    expect(layer).not.toHaveAttribute('data-raised');
    expect(screen.getByText('Erste Zeile')).toBeVisible();
    rerender(<SubtitleLayer texts={['Erste Zeile']} style={DEFAULT_SUBTITLE_STYLE} raised />);
    expect(container.querySelector('[data-subtitles]')).toHaveAttribute('data-raised');
    rerender(<SubtitleLayer texts={[]} style={DEFAULT_SUBTITLE_STYLE} raised />);
    expect(container.querySelector('[data-subtitles]')).toBeNull();
  });
});
