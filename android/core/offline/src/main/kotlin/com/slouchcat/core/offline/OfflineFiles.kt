package com.slouchcat.core.offline

import java.io.File
import java.io.RandomAccessFile

/**
 * The files of the downloads (concept 27 9.4): one folder per download below [root] with
 * `media.mp4`, `media.m3u8`, `<track>.vtt`, `trickplay-<sheet>.webp` and `poster`. The app keeps
 * [root] in `noBackupFilesDir`: no permission, gone with the app, never in a backup or a device
 * transfer. Unencrypted, like the PWA's files: copy protection is no aim.
 */
class OfflineFiles(val root: File) {
    fun folder(id: String): File = File(root, id.also(::requireId))

    fun media(id: String) = File(folder(id), Playlist.MEDIA)

    fun playlist(id: String) = File(folder(id), PLAYLIST)

    fun subtitle(id: String, track: Long) = File(folder(id), "$track.vtt")

    fun trickplay(id: String, sheet: Int) = File(folder(id), "trickplay-$sheet.webp")

    fun poster(id: String) = File(folder(id), POSTER)

    fun remove(id: String): Boolean {
        val dir = folder(id)
        return !dir.exists() || dir.deleteRecursively()
    }

    /** Folders without a record (a removal that failed, a crash between the two): deleted at the next start. */
    fun sweep(known: Set<String>) {
        root.listFiles()?.filter { it.isDirectory && it.name !in known }?.forEach { it.deleteRecursively() }
    }

    /** Bytes of every download on the device. */
    fun usedBytes(): Long = root.walkBottomUp().filter { it.isFile }.sumOf { it.length() }

    fun usedBytes(id: String): Long = folder(id).walkBottomUp().filter { it.isFile }.sumOf { it.length() }

    /** Free bytes of the volume the downloads live on. */
    fun freeBytes(): Long = root.apply { mkdirs() }.usableSpace

    fun totalBytes(): Long = root.apply { mkdirs() }.totalSpace

    /**
     * Cuts `media.mp4` to [length], the part that was durably written: after a crash the file can be
     * as long as expected and still carry an unwritten tail (stage 0 point 6).
     */
    fun truncateMedia(id: String, length: Long) {
        val file = media(id)
        if (!file.exists()) return
        if (file.length() > length) RandomAccessFile(file, "rw").use { it.setLength(length) }
    }

    fun writePlaylist(id: String, text: String) {
        val target = playlist(id)
        val temp = File(target.parentFile, "$PLAYLIST.part")
        temp.writeText(text)
        if (!temp.renameTo(target)) {
            target.writeText(text)
            temp.delete()
        }
    }

    companion object {
        const val PLAYLIST = "media.m3u8"
        const val POSTER = "poster"

        /** Download ids are ULIDs; anything else never names a folder (no `..`, no separator). */
        private val ID = Regex("^[0-9A-HJKMNP-TV-Z]{26}$")

        fun requireId(id: String) = require(ID.matches(id)) { "not a download id" }
    }
}
