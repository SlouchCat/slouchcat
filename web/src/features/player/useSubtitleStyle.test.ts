import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import {
  clampOffset,
  DEFAULT_SUBTITLE_STYLE,
  readSubtitleStyle,
  SUBTITLE_STYLE_KEY,
  useSubtitleStyle,
} from './useSubtitleStyle';

describe('useSubtitleStyle (V-6, per device)', () => {
  beforeEach(() => localStorage.clear());

  it('starts with the defaults and stores every change on this device', () => {
    const { result } = renderHook(() => useSubtitleStyle());
    expect(result.current[0]).toEqual(DEFAULT_SUBTITLE_STYLE);
    act(() => result.current[1]({ size: 'l', color: 'yellow' }));
    act(() => result.current[1]({ offsetMs: 300 }));
    expect(result.current[0]).toMatchObject({ size: 'l', color: 'yellow', offsetMs: 300 });
    expect(JSON.parse(localStorage.getItem(SUBTITLE_STYLE_KEY) ?? '{}')).toMatchObject({
      size: 'l',
      color: 'yellow',
      offsetMs: 300,
    });
    expect(renderHook(() => useSubtitleStyle()).result.current[0].size).toBe('l');
  });

  it('keeps the offset on the 0.1 s grid within ±30 s', () => {
    expect(clampOffset(149)).toBe(100);
    expect(clampOffset(-250)).toBe(-200);
    expect(clampOffset(45_000)).toBe(30_000);
    expect(clampOffset(Number.NaN)).toBe(0);
  });

  it('falls back field by field for junk', () => {
    localStorage.setItem(
      SUBTITLE_STYLE_KEY,
      JSON.stringify({ size: 'huge', background: 'box', color: 7, offsetMs: '1' }),
    );
    expect(readSubtitleStyle()).toEqual({ ...DEFAULT_SUBTITLE_STYLE, background: 'box' });
    localStorage.setItem(SUBTITLE_STYLE_KEY, '{');
    expect(readSubtitleStyle()).toEqual(DEFAULT_SUBTITLE_STYLE);
  });
});
