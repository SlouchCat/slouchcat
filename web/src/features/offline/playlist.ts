import type { DownloadManifest } from './types';

// The VOD playlist of a download (spike decision AN-20): the server's fragment index becomes an
// HLS media playlist with byte ranges into the one fMP4 file, so hls.js knows every position in
// advance and seeks without the browser searching the fragments itself.

/** Name of the media file in the playlist; the offline loader serves it from OPFS. */
export const PLAYLIST_MEDIA_URI = 'media.mp4';

const seconds = (ms: number) => (ms / 1000).toFixed(3);

export function buildPlaylist(manifest: DownloadManifest, mediaUri = PLAYLIST_MEDIA_URI): string {
  if (manifest.segments.length === 0) throw new Error('manifest without segments');
  const longest = Math.max(...manifest.segments.map((segment) => segment.durationMs));
  const lines = [
    '#EXTM3U',
    '#EXT-X-VERSION:7',
    `#EXT-X-TARGETDURATION:${Math.ceil(longest / 1000)}`,
    '#EXT-X-MEDIA-SEQUENCE:0',
    '#EXT-X-PLAYLIST-TYPE:VOD',
    '#EXT-X-INDEPENDENT-SEGMENTS',
    `#EXT-X-MAP:URI="${mediaUri}",BYTERANGE="${manifest.initLength}@0"`,
  ];
  for (const segment of manifest.segments) {
    lines.push(
      `#EXTINF:${seconds(segment.durationMs)},`,
      `#EXT-X-BYTERANGE:${segment.length}@${segment.offset}`,
      mediaUri,
    );
  }
  lines.push('#EXT-X-ENDLIST', '');
  return lines.join('\n');
}
