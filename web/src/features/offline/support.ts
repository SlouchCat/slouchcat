import type { ItemRef } from '@slouchcat/api-client';

// What the device offers for offline playback. Imported by the start bundle (detail page, root
// layout), so it stays free of IndexedDB and OPFS code: those load with the offline chunk.

/** Kinds the server downloads (O-1, O-2; audiobooks and podcast episodes since M6). */
export const isDownloadable = (kind: ItemRef['kind']) =>
  kind === 'movie' ||
  kind === 'episode' ||
  kind === 'season' ||
  kind === 'audiobook' ||
  kind === 'podcast_episode';

/** The parts of the browser the checks read; injectable for tests. */
export interface UserAgentPlatform {
  userAgent: string;
  maxTouchPoints: number;
}

const browserPlatform = (): UserAgentPlatform => ({
  userAgent: navigator.userAgent,
  maxTouchPoints: navigator.maxTouchPoints || 0,
});

/**
 * Whether this browser can keep downloads: a secure context (HTTPS or localhost) with IndexedDB
 * and the origin private file system. Plain HTTP in the local network has neither.
 */
export function offlineSupported(): boolean {
  return (
    typeof window !== 'undefined' &&
    window.isSecureContext === true &&
    typeof indexedDB !== 'undefined' &&
    typeof navigator.storage?.getDirectory === 'function'
  );
}

/** iPhone, iPod or iPad; iPadOS asks for desktop pages and reports a Mac with a touch screen. */
export function isIos(platform: UserAgentPlatform = browserPlatform()): boolean {
  const ua = platform.userAgent;
  return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && platform.maxTouchPoints > 1);
}

/**
 * Apple devices and Safari (O-7): WebKit may delete the storage of a site that was not used for a
 * while, so the storage panel shows a hint there. Chrome, Edge and Firefox on a Mac do not count.
 */
export function isAppleStorage(platform: UserAgentPlatform = browserPlatform()): boolean {
  if (isIos(platform)) return true;
  const ua = platform.userAgent;
  return (
    /Version\/\d+[.\d]* (?:Mobile\/\S+ )?Safari\//.test(ua) &&
    !/Chrome|Chromium|CriOS|FxiOS|Firefox|Edg|OPR|Android/.test(ua)
  );
}

export type NetworkKind = 'wifi' | 'cellular' | 'unknown';

/**
 * What the Network Information API says about the connection (Chromium on Android); `unknown`
 * where the browser does not tell (desktop Firefox, Safari).
 */
export function networkKind(): NetworkKind {
  const type = (navigator as Navigator & { connection?: { type?: string } }).connection?.type;
  if (type === 'wifi' || type === 'ethernet') return 'wifi';
  if (type === 'cellular') return 'cellular';
  return 'unknown';
}
