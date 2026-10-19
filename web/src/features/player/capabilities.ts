import type { components } from '@slouchcat/api-client';

export type PlaybackCapabilities = components['schemas']['PlaybackCapabilities'];
export type HlsSupport = PlaybackCapabilities['hls'];

/** MIME type a media element answers for HLS when it plays HLS itself (Safari). */
export const HLS_MIME = 'application/vnd.apple.mpegurl';

/**
 * Codec probes (W-1 device check; device profiles follow in M4). HEVC is probed as Main 10,
 * because the HEVC files of a typical library are 10-bit; a device with Main only gets a transcode.
 */
const videoProbes: Record<string, string> = {
  h264: 'video/mp4; codecs="avc1.64001F"',
  hevc: 'video/mp4; codecs="hvc1.2.4.L120.90"',
};
const audioProbes: Record<string, string> = {
  aac: 'audio/mp4; codecs="mp4a.40.2"',
  ac3: 'audio/mp4; codecs="ac-3"',
  eac3: 'audio/mp4; codecs="ec-3"',
};

/** The parts of the platform the capability check reads; injectable for tests. */
export interface MediaPlatform {
  canPlayType: (type: string) => string;
  /** isTypeSupported of MediaSource or ManagedMediaSource, or null when the browser has no MSE. */
  isTypeSupported: ((type: string) => boolean) | null;
  /** Whether the classic MediaSource exists (not only Apple's ManagedMediaSource of the iPhone). */
  classicMse: boolean;
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
 * What this browser reports with startPlayback: codecs the media element can play (and, on the
 * hls.js path, MSE can append) plus the kind of HLS support. The server decides the mode.
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
