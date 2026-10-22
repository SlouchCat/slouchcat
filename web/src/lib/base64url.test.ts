import { describe, expect, it } from 'vitest';
import { base64UrlToBytes } from './base64url';

describe('base64UrlToBytes', () => {
  it('decodes the URL alphabet with and without padding', () => {
    // 0xfb 0xff 0xbf is "+/+/" in base64 and "-_-_" in base64url.
    expect([...base64UrlToBytes('-_-_')]).toEqual([0xfb, 0xff, 0xbf]);
    expect([...base64UrlToBytes('AQ')]).toEqual([1]);
    expect([...base64UrlToBytes('AQ==')]).toEqual([1]);
    expect([...base64UrlToBytes('AQID')]).toEqual([1, 2, 3]);
    expect([...base64UrlToBytes('')]).toEqual([]);
  });

  it('gives the 65 bytes of an uncompressed P-256 VAPID key', () => {
    const key =
      'BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U';
    const bytes = base64UrlToBytes(key);
    expect(bytes).toHaveLength(65);
    expect(bytes[0]).toBe(4);
  });

  it('refuses characters of standard base64 and impossible lengths', () => {
    expect(() => base64UrlToBytes('a+b/')).toThrow('not base64url');
    expect(() => base64UrlToBytes('AQIDB')).toThrow('not base64url');
  });
});
