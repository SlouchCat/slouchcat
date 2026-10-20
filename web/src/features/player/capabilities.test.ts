import { describe, expect, it } from 'vitest';
import {
  deviceCapabilities,
  deviceName,
  deviceType,
  HLS_MIME,
  hlsEngine,
  playbackCapabilities,
  type DevicePlatform,
  type MediaPlatform,
} from './capabilities';

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

const chromeLinuxUa =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36';
const firefoxUa = 'Mozilla/5.0 (X11; Linux x86_64; rv:150.0) Gecko/20100101 Firefox/150.0';
const iphoneUa =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1';
const ipadDesktopUa =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Safari/605.1.15';
const androidTabletUa =
  'Mozilla/5.0 (Linux; Android 16; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36';

function device(overrides: Partial<DevicePlatform> = {}): DevicePlatform {
  return {
    ...platform([h264, aac, 'video/mp4', 'video/webm'], [h264, aac]),
    screen: { width: 1920, height: 1080, devicePixelRatio: 1 },
    matches: () => false,
    userAgent: chromeLinuxUa,
    userAgentData: null,
    maxTouchPoints: 0,
    ...overrides,
  };
}

describe('device capabilities (W-7)', () => {
  it('reports containers, codecs, screen in device pixels, HDR, name and type', () => {
    const capabilities = deviceCapabilities(
      device({
        screen: { width: 1280, height: 720, devicePixelRatio: 1.5 },
        matches: (query) => query === '(dynamic-range: high)',
        userAgentData: {
          brands: [
            { brand: 'Not.A/Brand', version: '99' },
            { brand: 'Chromium', version: '156' },
            { brand: 'Google Chrome', version: '156' },
          ],
          mobile: false,
          platform: 'Linux',
        },
      }),
    );
    expect(capabilities).toEqual({
      containers: ['mp4', 'webm'],
      video: ['h264'],
      audio: ['aac'],
      hls: 'mse',
      maxWidth: 1920,
      maxHeight: 1080,
      hdr: true,
      name: 'Chrome 156 · Linux',
      type: 'desktop',
    });
  });

  it('probes the newer codecs and mkv', () => {
    const all = [
      h264,
      'video/mp4; codecs="av01.0.08M.08"',
      'video/webm; codecs="vp9"',
      aac,
      'audio/mp4; codecs="opus"',
      'audio/mp4; codecs="flac"',
      'audio/mpeg',
      'video/x-matroska',
    ];
    const capabilities = deviceCapabilities(device(platform(all, all)));
    expect(capabilities.video).toEqual(['h264', 'av1', 'vp9']);
    expect(capabilities.audio).toEqual(['aac', 'opus', 'flac', 'mp3']);
    expect(capabilities.containers).toEqual(['mkv']);
  });

  it('names the browser from the UA string without client hints', () => {
    expect(deviceName({ userAgent: firefoxUa, userAgentData: null })).toBe('Firefox 150 · Linux');
    expect(deviceName({ userAgent: iphoneUa, userAgentData: null })).toBe('Safari 19 · iOS');
    expect(deviceName({ userAgent: chromeLinuxUa, userAgentData: null })).toBe(
      'Chrome 156 · Linux',
    );
    expect(deviceName({ userAgent: '', userAgentData: null })).toBeUndefined();
    const long = {
      brands: [{ brand: 'x'.repeat(100), version: '1' }],
      mobile: false,
      platform: '',
    };
    expect(deviceName({ userAgent: '', userAgentData: long })).toHaveLength(80);
  });

  it('tells phones, tablets and desktops apart', () => {
    expect(deviceType(device({ userAgent: iphoneUa }))).toBe('phone');
    expect(deviceType(device({ userAgent: androidTabletUa }))).toBe('tablet');
    const ipad = device({
      userAgent: ipadDesktopUa,
      maxTouchPoints: 5,
      matches: (query) => query === '(pointer: coarse)',
      screen: { width: 1024, height: 1366, devicePixelRatio: 2 },
    });
    expect(deviceType(ipad)).toBe('tablet');
    const android = device({
      userAgentData: { brands: [], mobile: true, platform: 'Android' },
      screen: { width: 412, height: 915, devicePixelRatio: 2.6 },
    });
    expect(deviceType(android)).toBe('phone');
    expect(deviceType(device())).toBe('desktop');
  });

  it('keeps the screen size within the contract range', () => {
    const capabilities = deviceCapabilities(
      device({ screen: { width: 9000, height: 0, devicePixelRatio: 3 } }),
    );
    expect(capabilities.maxWidth).toBe(16384);
    expect(capabilities.maxHeight).toBe(1);
  });
});
