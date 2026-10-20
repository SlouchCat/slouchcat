import type { components } from '@slouchcat/api-client';

export type PlaybackCapabilities = components['schemas']['PlaybackCapabilities'];
export type HlsSupport = PlaybackCapabilities['hls'];
export type DeviceType = NonNullable<PlaybackCapabilities['type']>;

/** MIME type a media element answers for HLS when it plays HLS itself (Safari). */
export const HLS_MIME = 'application/vnd.apple.mpegurl';

/**
 * Codec probes (W-1, W-7), ffprobe names as keys. HEVC is probed as Main 10, because the HEVC
 * files of a typical library are 10-bit; a device with Main only gets a transcode.
 */
const videoProbes: Record<string, string> = {
  h264: 'video/mp4; codecs="avc1.64001F"',
  hevc: 'video/mp4; codecs="hvc1.2.4.L120.90"',
  av1: 'video/mp4; codecs="av01.0.08M.08"',
  vp9: 'video/webm; codecs="vp9"',
};
const audioProbes: Record<string, string> = {
  aac: 'audio/mp4; codecs="mp4a.40.2"',
  ac3: 'audio/mp4; codecs="ac-3"',
  eac3: 'audio/mp4; codecs="ec-3"',
  opus: 'audio/mp4; codecs="opus"',
  flac: 'audio/mp4; codecs="flac"',
  mp3: 'audio/mpeg',
};
/** Containers the media element may play directly (the server's names). */
const containerProbes: Record<string, string> = {
  mp4: 'video/mp4',
  webm: 'video/webm',
  mkv: 'video/x-matroska',
};

/** Longest device name the contract accepts. */
const MAX_NAME = 80;
/** Largest screen side the contract accepts (device pixels). */
const MAX_SIDE = 16384;
/** Shorter screen side (CSS pixels) below which a touch device counts as a phone. */
const PHONE_SHORT_SIDE = 600;

/** The parts of the platform the capability check reads; injectable for tests. */
export interface MediaPlatform {
  canPlayType: (type: string) => string;
  /** isTypeSupported of MediaSource or ManagedMediaSource, or null when the browser has no MSE. */
  isTypeSupported: ((type: string) => boolean) | null;
  /** Whether the classic MediaSource exists (not only Apple's ManagedMediaSource of the iPhone). */
  classicMse: boolean;
}

/** User-Agent Client Hints (Chromium); absent in Firefox and Safari. */
export interface UserAgentData {
  brands: readonly { brand: string; version: string }[];
  mobile: boolean;
  platform: string;
}

/** The device around the media element: screen, HDR, user agent; injectable for tests. */
export interface DevicePlatform extends MediaPlatform {
  /** Screen size in CSS pixels and the device pixel ratio. */
  screen: { width: number; height: number; devicePixelRatio: number };
  /** `matchMedia(query).matches`. */
  matches: (query: string) => boolean;
  userAgent: string;
  userAgentData: UserAgentData | null;
  maxTouchPoints: number;
}

/** The platform of this browser: a detached video element and (Managed)MediaSource. */
export function browserPlatform(): MediaPlatform {
  const video = document.createElement('video');
  const scope = globalThis as {
    MediaSource?: { isTypeSupported(type: string): boolean };
    ManagedMediaSource?: { isTypeSupported(type: string): boolean };
  };
  const mse = scope.MediaSource ?? scope.ManagedMediaSource;
  return {
    canPlayType: (type) => video.canPlayType(type),
    isTypeSupported: mse ? (type) => mse.isTypeSupported(type) : null,
    classicMse: scope.MediaSource !== undefined,
  };
}

/** This browser's media platform plus its screen and user agent. */
export function browserDevicePlatform(): DevicePlatform {
  const nav = navigator as Navigator & { userAgentData?: UserAgentData };
  return {
    ...browserPlatform(),
    screen: {
      width: window.screen.width || window.innerWidth,
      height: window.screen.height || window.innerHeight,
      devicePixelRatio: window.devicePixelRatio || 1,
    },
    matches: (query) => typeof window.matchMedia === 'function' && window.matchMedia(query).matches,
    userAgent: nav.userAgent,
    userAgentData: nav.userAgentData ?? null,
    maxTouchPoints: nav.maxTouchPoints || 0,
  };
}

/**
 * HLS through hls.js wherever the classic MediaSource exists (Chromium, Firefox, desktop Safari):
 * one engine with the same start position, seeking and error handling everywhere, although
 * current Chromium also answers canPlayType for HLS. Native HLS where only the platform player
 * fits (iPhone Safari, which offers just ManagedMediaSource), else MSE through
 * ManagedMediaSource; without either the browser cannot play HLS at all (ADR 0004).
 */
export function hlsSupport(platform: MediaPlatform): HlsSupport {
  const mse = platform.isTypeSupported?.(videoProbes.h264!) === true;
  if (mse && platform.classicMse) return 'mse';
  if (platform.canPlayType(HLS_MIME) !== '') return 'native';
  return mse ? 'mse' : 'none';
}

/**
 * The codecs this browser can play (and, on the hls.js path, MSE can append) plus the kind of
 * HLS support. The server decides the mode.
 */
export function playbackCapabilities(
  platform: MediaPlatform = browserPlatform(),
): PlaybackCapabilities {
  const hls = hlsSupport(platform);
  const playable = (type: string) =>
    platform.canPlayType(type) !== '' &&
    (hls !== 'mse' || platform.isTypeSupported?.(type) === true);
  const supported = (probes: Record<string, string>) =>
    Object.entries(probes)
      .filter(([, type]) => playable(type))
      .map(([codec]) => codec);
  return { video: supported(videoProbes), audio: supported(audioProbes), hls };
}

/** Major version of a version string ("156.0.7340.12" → "156"). */
const major = (version: string) => version.split('.')[0] ?? version;

// Order matters: Edge and Opera carry "Chrome/" too, Chrome carries "Safari/".
const browserPatterns: [name: string, pattern: RegExp][] = [
  ['Edge', /Edg(?:e|A|iOS)?\/(\d+)/],
  ['Opera', /OPR\/(\d+)/],
  ['Firefox', /(?:Firefox|FxiOS)\/(\d+)/],
  ['Chrome', /(?:Chrome|CriOS)\/(\d+)/],
  ['Safari', /Version\/(\d+)[.\d]* (?:Mobile\/\S+ )?Safari\//],
];

const osPatterns: [name: string, pattern: RegExp][] = [
  ['iPadOS', /iPad/],
  ['iOS', /iPhone|iPod/],
  ['Android', /Android/],
  ['ChromeOS', /CrOS/],
  ['Windows', /Windows/],
  ['macOS', /Mac OS X|Macintosh/],
  ['Linux', /Linux|X11/],
];

/** Client-hint platform names in the words of the UA fallback. */
const hintPlatforms: Record<string, string> = {
  'Chrome OS': 'ChromeOS',
  'Chromium OS': 'ChromeOS',
};

/** "Chrome 156 · Linux": browser, major version and system, from client hints or the UA string. */
export function deviceName(
  platform: Pick<DevicePlatform, 'userAgent' | 'userAgentData'>,
): string | undefined {
  let browser: string | undefined;
  let os: string | undefined;
  const hints = platform.userAgentData;
  if (hints) {
    // Brands list GREASE entries ("Not.A/Brand") and the engine ("Chromium") next to the product.
    const brands = hints.brands.filter(({ brand }) => !/not.a.brand/i.test(brand));
    const product = brands.find(({ brand }) => brand !== 'Chromium') ?? brands[0];
    if (product) {
      const brand = product.brand.replace(/^(Google|Microsoft) /, '');
      browser = `${brand} ${major(product.version)}`;
    }
    if (hints.platform) os = hintPlatforms[hints.platform] ?? hints.platform;
  }
  const ua = platform.userAgent;
  if (!browser) {
    for (const [name, pattern] of browserPatterns) {
      const match = pattern.exec(ua);
      if (match) {
        browser = `${name} ${match[1]}`;
        break;
      }
    }
  }
  os ??= osPatterns.find(([, pattern]) => pattern.test(ua))?.[0];
  const name = [browser, os].filter(Boolean).join(' · ');
  return name ? name.slice(0, MAX_NAME) : undefined;
}

/** Phone, tablet or desktop from client hints, the UA string and the touch screen size. */
export function deviceType(
  platform: Pick<
    DevicePlatform,
    'userAgent' | 'userAgentData' | 'maxTouchPoints' | 'screen' | 'matches'
  >,
): DeviceType {
  const ua = platform.userAgent;
  const shortSide = Math.min(platform.screen.width, platform.screen.height);
  if (/iPad|Tablet/.test(ua) || (/Android/.test(ua) && !/Mobile/.test(ua))) return 'tablet';
  if (/iPhone|iPod/.test(ua) || (/Android/.test(ua) && /Mobile/.test(ua))) return 'phone';
  if (platform.userAgentData?.mobile) return shortSide < PHONE_SHORT_SIDE ? 'phone' : 'tablet';
  // iPadOS asks for desktop pages and reports a Mac; its touch screen gives it away.
  if (platform.maxTouchPoints > 1 && platform.matches('(pointer: coarse)')) {
    return shortSide < PHONE_SHORT_SIDE ? 'phone' : 'tablet';
  }
  return 'desktop';
}

/** Screen side in device pixels, in the range the contract accepts. */
const deviceSide = (css: number, ratio: number) =>
  Math.min(MAX_SIDE, Math.max(1, Math.round(css * ratio)));

/**
 * The device's capability profile for startPlayback (W-7): containers, codecs, HLS support,
 * screen size in device pixels, HDR display, name and type. The server stores it with the device
 * and decides direct play, remux or transcode from it; the client decides nothing.
 */
export function deviceCapabilities(
  platform: DevicePlatform = browserDevicePlatform(),
): PlaybackCapabilities {
  const containers = Object.entries(containerProbes)
    .filter(([, type]) => platform.canPlayType(type) !== '')
    .map(([container]) => container);
  const { width, height, devicePixelRatio } = platform.screen;
  const name = deviceName(platform);
  return {
    containers,
    ...playbackCapabilities(platform),
    maxWidth: deviceSide(width, devicePixelRatio),
    maxHeight: deviceSide(height, devicePixelRatio),
    hdr: platform.matches('(dynamic-range: high)'),
    ...(name ? { name } : {}),
    type: deviceType(platform),
  };
}

/** How an `hls` playback runs in this browser. */
export type HlsEngine = 'native' | 'hlsjs' | 'unsupported';

/** How the player runs: the Range stream, native HLS, hls.js, or not at all. */
export type PlaybackEngine = 'direct' | HlsEngine;

/** Safari plays the master playlist itself; elsewhere hls.js feeds MSE (ADR 0004). */
export function hlsEngine(platform: MediaPlatform): HlsEngine {
  switch (hlsSupport(platform)) {
    case 'native':
      return 'native';
    case 'mse':
      return 'hlsjs';
    default:
      return 'unsupported';
  }
}
