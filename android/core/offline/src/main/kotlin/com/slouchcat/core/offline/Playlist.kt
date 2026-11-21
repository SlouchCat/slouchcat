package com.slouchcat.core.offline

import com.slouchcat.core.api.generated.models.DownloadManifest
import java.util.Locale

/**
 * The HLS playlist of a download (stage 0 point 5, as the PWA's playlist.ts): the server's fragment
 * index becomes a VOD media playlist with byte ranges into the one fMP4 file, so Media3 knows every
 * position in advance and seeks (the file alone has no `sidx` and is not seekable).
 */
object Playlist {
    /** Name of the media file next to the playlist. */
    const val MEDIA = "media.mp4"

    private fun seconds(ms: Long) = String.format(Locale.ROOT, "%.3f", ms / 1000.0)

    fun build(manifest: DownloadManifest, media: String = MEDIA): String {
        require(manifest.segments.isNotEmpty()) { "manifest without segments" }
        val longest = manifest.segments.maxOf { it.durationMs }
        val lines = mutableListOf(
            "#EXTM3U",
            "#EXT-X-VERSION:7",
            "#EXT-X-TARGETDURATION:${(longest + 999) / 1000}",
            "#EXT-X-MEDIA-SEQUENCE:0",
            "#EXT-X-PLAYLIST-TYPE:VOD",
            "#EXT-X-INDEPENDENT-SEGMENTS",
            "#EXT-X-MAP:URI=\"$media\",BYTERANGE=\"${manifest.initLength}@0\"",
        )
        for (segment in manifest.segments) {
            lines += "#EXTINF:${seconds(segment.durationMs)},"
            lines += "#EXT-X-BYTERANGE:${segment.length}@${segment.offset}"
            lines += media
        }
        lines += "#EXT-X-ENDLIST"
        return lines.joinToString("\n", postfix = "\n")
    }

    /** Whether the segments end where the file ends (a file whose tail a crash left unwritten fails the size check first). */
    fun coversFile(manifest: DownloadManifest): Boolean {
        val last = manifest.segments.lastOrNull() ?: return false
        return last.offset + last.length == manifest.sizeBytes
    }
}
