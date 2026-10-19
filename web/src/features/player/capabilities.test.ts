import { describe, expect, it } from 'vitest';
import { HLS_MIME, hlsEngine, playbackCapabilities, type MediaPlatform } from './capabilities';

function platform(
  canPlay: string[],
  mse: string[] | null,
  classicMse = mse !== null,
): MediaPlatform {
  return {
    canPlayType: (type) => (canPlay.includes(type) ? 'maybe' : ''),
    isTypeSupported: mse ? (type) => mse.includes(type) : null,
    classicMse,
  };
}

const h264 = 'video/mp4; codecs="avc1.64001F"';
const hevc = 'video/mp4; codecs="hvc1.2.4.L120.90"';
const aac = 'audio/mp4; codecs="mp4a.40.2"';
const ac3 = 'audio/mp4; codecs="ac-3"';
const eac3 = 'audio/mp4; codecs="ec-3"';

describe('playback capabilities', () => {
  it('reports H.264/AAC over MSE for Chromium and Firefox without HEVC', () => {
    const firefox = platform([h264, aac], [h264, aac]);
    expect(playbackCapabilities(firefox)).toEqual({ video: ['h264'], audio: ['aac'], hls: 'mse' });
    expect(hlsEngine(firefox)).toBe('hlsjs');
  });

  it('reports HEVC only when both the element and MSE can play it', () => {
    expect(playbackCapabilities(platform([h264, hevc, aac], [h264, aac])).video).toEqual(['h264']);
    expect(playbackCapabilities(platform([h264, hevc, aac], [h264, hevc, aac])).video).toEqual([
      'h264',
      'hevc',
    ]);
  });

  it('prefers hls.js over the native HLS of current Chromium', () => {
    const chromium = platform([HLS_MIME, h264, aac], [h264, aac]);
    expect(playbackCapabilities(chromium).hls).toBe('mse');
    expect(hlsEngine(chromium)).toBe('hlsjs');
  });

  it('uses hls.js in desktop Safari, which has MSE', () => {
    const safari = platform([HLS_MIME, h264, hevc, aac, ac3, eac3], [h264, hevc, aac, ac3, eac3]);
    expect(playbackCapabilities(safari)).toEqual({
      video: ['h264', 'hevc'],
      audio: ['aac', 'ac3', 'eac3'],
      hls: 'mse',
    });
    expect(hlsEngine(safari)).toBe('hlsjs');
  });

  it('reports native HLS and the surround codecs for iPhone Safari (ManagedMediaSource only)', () => {
    const iphone = platform([HLS_MIME, h264, hevc, aac, ac3, eac3], [h264], false);
    expect(playbackCapabilities(iphone)).toEqual({
      video: ['h264', 'hevc'],
      audio: ['aac', 'ac3', 'eac3'],
      hls: 'native',
    });
    expect(hlsEngine(iphone)).toBe('native');
  });

  it('reports no HLS without MSE and native support', () => {
    const old = platform([h264, aac], null);
    expect(playbackCapabilities(old)).toEqual({ video: ['h264'], audio: ['aac'], hls: 'none' });
    expect(hlsEngine(old)).toBe('unsupported');
  });
});
