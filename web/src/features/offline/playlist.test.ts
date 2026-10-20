import { describe, expect, it } from 'vitest';
import { manifest } from '@/test/offlineFixtures';
import { buildPlaylist } from './playlist';

describe('buildPlaylist', () => {
  it('writes a VOD playlist with the init map and a byte range per segment', () => {
    expect(buildPlaylist(manifest()).split('\n')).toEqual([
      '#EXTM3U',
      '#EXT-X-VERSION:7',
      '#EXT-X-TARGETDURATION:4',
      '#EXT-X-MEDIA-SEQUENCE:0',
      '#EXT-X-PLAYLIST-TYPE:VOD',
      '#EXT-X-INDEPENDENT-SEGMENTS',
      '#EXT-X-MAP:URI="media.mp4",BYTERANGE="1000@0"',
      '#EXTINF:4.000,',
      '#EXT-X-BYTERANGE:4000@1000',
      'media.mp4',
      '#EXTINF:4.000,',
      '#EXT-X-BYTERANGE:4000@5000',
      'media.mp4',
      '#EXT-X-ENDLIST',
      '',
    ]);
  });

  it('rounds the target duration up to the longest segment', () => {
    const playlist = buildPlaylist({
      ...manifest(),
      segments: [
        { startMs: 0, durationMs: 4170, offset: 1000, length: 10 },
        { startMs: 4170, durationMs: 2002, offset: 1010, length: 10 },
      ],
    });
    expect(playlist).toContain('#EXT-X-TARGETDURATION:5\n');
    expect(playlist).toContain('#EXTINF:4.170,\n#EXT-X-BYTERANGE:10@1000\nmedia.mp4\n');
    expect(playlist).toContain('#EXTINF:2.002,\n#EXT-X-BYTERANGE:10@1010\nmedia.mp4\n');
  });

  it('names another media URI when asked', () => {
    expect(buildPlaylist(manifest(), 'file.mp4')).toContain(
      '#EXT-X-MAP:URI="file.mp4",BYTERANGE="1000@0"',
    );
  });

  it('refuses a manifest without segments', () => {
    expect(() => buildPlaylist({ ...manifest(), segments: [] })).toThrow();
  });
});
