import { describe, expect, it } from 'vitest';
import { isAppleStorage, isDownloadable, isIos, offlineSupported } from './support';

const agents = {
  iphone:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Mobile/15E148 Safari/604.1',
  ipadDesktop:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/19.0 Safari/605.1.15',
  iphoneChrome:
    'Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/150.0.0.0 Mobile/15E148 Safari/604.1',
  macChrome:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36',
  macFirefox: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 15.0; rv:158.0) Gecko/20100101 Firefox/158.0',
  android:
    'Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Mobile Safari/537.36',
  windowsEdge:
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/156.0.0.0 Safari/537.36 Edg/156.0.0.0',
} as const;

describe('storage hint for Apple devices and Safari (O-7)', () => {
  it('shows on iPhone, on iPad (desktop mode with touch) and in Safari on the Mac', () => {
    expect(isAppleStorage({ userAgent: agents.iphone, maxTouchPoints: 5 })).toBe(true);
    expect(isAppleStorage({ userAgent: agents.iphoneChrome, maxTouchPoints: 5 })).toBe(true);
    expect(isAppleStorage({ userAgent: agents.ipadDesktop, maxTouchPoints: 5 })).toBe(true);
    expect(isAppleStorage({ userAgent: agents.ipadDesktop, maxTouchPoints: 0 })).toBe(true);
  });

  it('does not show in Chrome, Firefox or Edge outside iOS', () => {
    for (const userAgent of [
      agents.macChrome,
      agents.macFirefox,
      agents.android,
      agents.windowsEdge,
    ]) {
      expect(isAppleStorage({ userAgent, maxTouchPoints: 0 })).toBe(false);
    }
  });

  it('knows iOS and iPadOS (the Wi-Fi option is hidden there, AN-13)', () => {
    expect(isIos({ userAgent: agents.iphone, maxTouchPoints: 5 })).toBe(true);
    expect(isIos({ userAgent: agents.ipadDesktop, maxTouchPoints: 5 })).toBe(true);
    expect(isIos({ userAgent: agents.ipadDesktop, maxTouchPoints: 0 })).toBe(false);
    expect(isIos({ userAgent: agents.android, maxTouchPoints: 5 })).toBe(false);
  });

  it('needs IndexedDB, OPFS and a secure context for downloads (jsdom has none)', () => {
    expect(offlineSupported()).toBe(false);
  });
});

describe('downloadable kinds', () => {
  it('films, episodes, seasons, audiobooks and podcast episodes; never a container', () => {
    for (const kind of ['movie', 'episode', 'season', 'audiobook', 'podcast_episode'] as const) {
      expect(isDownloadable(kind)).toBe(true);
    }
    for (const kind of ['series', 'extra', 'podcast'] as const) {
      expect(isDownloadable(kind)).toBe(false);
    }
  });
});
