package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.Track
import java.util.Locale

/**
 * Which group of the player is which track of the server (concept 26 stage 0 point 4): per kind
 * the n-th embedded track of the server by `index` is the n-th group of that kind in Media3, the
 * order of the file. Before the app selects a group it checks language and title; when they do not
 * agree, it takes the way over the server (a new start with the track).
 */
object TrackMatching {
    /** The player group of [track] among the embedded [server] tracks of its kind, `null` when the order does not agree. */
    fun group(server: List<Track>, engine: List<EngineTrack>, track: Track): Int? {
        if (track.source != Track.Source.EMBEDDED || track.index < 0) return null
        val embedded = server.filter { it.source == Track.Source.EMBEDDED && it.index >= 0 }.sortedBy { it.index }
        val fileGroups = engine.withIndex().filter { !it.value.sideloaded }
        if (embedded.size != fileGroups.size) return null
        val n = embedded.indexOfFirst { it.id == track.id }.takeIf { it >= 0 } ?: return null
        val (index, group) = fileGroups[n]
        if (!sameLanguage(track.language, group.language)) return null
        if (!track.title.isNullOrBlank() && !group.label.isNullOrBlank() && track.title != group.label) return null
        return index
    }

    /** The group the app added from `subtitleUrl`, if any. */
    fun sideloaded(engine: List<EngineTrack>): Int? = engine.indexOfFirst { it.sideloaded }.takeIf { it >= 0 }

    /** Two language codes name the same language; an unknown one matches anything. */
    fun sameLanguage(a: String?, b: String?): Boolean {
        val x = normalize(a) ?: return true
        val y = normalize(b) ?: return true
        return x == y
    }

    /** ISO 639-1 for a code of 639-1, 639-2/T or 639-2/B (`de`, `deu`, `ger`); `null` for none or `und`. */
    fun normalize(code: String?): String? {
        val c = code?.trim()?.lowercase()?.substringBefore('-')?.substringBefore('_')
        if (c.isNullOrEmpty() || c == "und" || c == "mul" || c == "zxx") return null
        if (c.length == 2) return c
        BIBLIOGRAPHIC[c]?.let { return it }
        return TERMINOLOGY[c] ?: c
    }

    private val TERMINOLOGY: Map<String, String> by lazy {
        Locale.getISOLanguages().associateBy { runCatching { Locale.forLanguageTag(it).isO3Language }.getOrDefault(it) }
    }

    /** The ISO 639-2/B codes that differ from /T (files from older tools carry them). */
    private val BIBLIOGRAPHIC = mapOf(
        "ger" to "de", "fre" to "fr", "dut" to "nl", "chi" to "zh", "cze" to "cs", "gre" to "el", "per" to "fa",
        "rum" to "ro", "slo" to "sk", "alb" to "sq", "arm" to "hy", "baq" to "eu", "bur" to "my", "geo" to "ka",
        "ice" to "is", "mac" to "mk", "mao" to "mi", "may" to "ms", "tib" to "bo", "wel" to "cy",
    )
}
