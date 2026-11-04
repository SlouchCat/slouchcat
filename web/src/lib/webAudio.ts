/** What `supportsWebAudioBoost` looks at; a test passes its own. */
export interface WebAudioEnvironment {
  userAgent: string;
  /** `navigator.maxTouchPoints`: an iPad asks for the desktop site and calls itself a Mac. */
  maxTouchPoints: number;
  /** The page can create an AudioContext. */
  hasAudioContext: boolean;
}

function currentEnvironment(): WebAudioEnvironment {
  return {
    userAgent: navigator.userAgent,
    maxTouchPoints: navigator.maxTouchPoints,
    hasAudioContext: typeof window.AudioContext === 'function',
  };
}

/**
 * Whether the player may offer the night mode and a volume above 100 % (Web Audio,
 * `createMediaElementSource`, 11.10). Not in Safari: WebKit gave silence for HLS sources routed
 * through Web Audio, and without an Apple device to test on both stay hidden there (concept 13,
 * 11.13). Every browser on iOS and iPadOS is WebKit, whatever it calls itself.
 */
export function supportsWebAudioBoost(env: WebAudioEnvironment = currentEnvironment()): boolean {
  if (!env.hasAudioContext) return false;
  const ua = env.userAgent;
  const ios = /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && env.maxTouchPoints > 1);
  if (ios) return false;
  const safari = /Safari\//.test(ua) && !/Chrome\/|Chromium\/|Edg\/|OPR\/|Firefox\//.test(ua);
  return !safari;
}
