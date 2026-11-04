import { describe, expect, it } from 'vitest';
import { supportsWebAudioBoost } from './webAudio';

const agents = {
  chrome:
    'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36',
  edge: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36 Edg/141.0.0.0',
  firefox: 'Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0',
  androidChrome:
    'Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Mobile Safari/537.36',
  macSafari:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15',
  iphoneSafari:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 26_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Mobile/15E148 Safari/604.1',
  iphoneChrome:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 26_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/141.0.0.0 Mobile/15E148 Safari/604.1',
} as const;

const check = (userAgent: string, maxTouchPoints = 0, hasAudioContext = true) =>
  supportsWebAudioBoost({ userAgent, maxTouchPoints, hasAudioContext });

describe('supportsWebAudioBoost (concept 13, 11.10)', () => {
  it('allows Chromium and Firefox on desktop and Android', () => {
    expect(check(agents.chrome)).toBe(true);
    expect(check(agents.edge)).toBe(true);
    expect(check(agents.firefox)).toBe(true);
    expect(check(agents.androidChrome, 5)).toBe(true);
  });

  it('hides it in Safari on the Mac', () => {
    expect(check(agents.macSafari)).toBe(false);
  });

  it('hides it on every browser of the iPhone, and on an iPad that calls itself a Mac', () => {
    expect(check(agents.iphoneSafari, 5)).toBe(false);
    expect(check(agents.iphoneChrome, 5)).toBe(false);
    expect(check(agents.macSafari, 5)).toBe(false);
  });

  it('needs an AudioContext', () => {
    expect(check(agents.chrome, 0, false)).toBe(false);
  });
});
