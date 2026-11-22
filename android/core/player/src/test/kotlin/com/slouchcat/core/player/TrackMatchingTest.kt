package com.slouchcat.core.player

import com.slouchcat.core.api.generated.models.Track
import com.slouchcat.core.api.generated.models.Trickplay
import com.slouchcat.core.api.generated.models.TrickplaySheet
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class TrackMatchingTest {
    private val server = listOf(audio(10, 1, "ger", title = "Deutsch"), audio(11, 2, "eng", title = "English"))

    @Test
    fun theNthTrackIsTheNthGroup() {
        val engine = listOf(group("de", "Deutsch"), group("en", "English"))
        assertEquals(0, TrackMatching.group(server, engine, server[0]))
        assertEquals(1, TrackMatching.group(server, engine, server[1]))
    }

    @Test
    fun languageOrTitleThatDisagreeStopTheMatch() {
        assertNull(TrackMatching.group(server, listOf(group("fr"), group("en")), server[0]))
        assertNull(TrackMatching.group(server, listOf(group("de", "Kommentar"), group("en")), server[0]))
        assertNull(TrackMatching.group(server, listOf(group("de")), server[0]))
    }

    @Test
    fun sideloadedGroupsAndSidecarTracksStayOut() {
        val subs = listOf(subtitle(1, 3, "deu"), subtitle(2, -1, "eng", source = Track.Source.SIDECAR))
        val engine = listOf(EngineTrack("de", null, "application/x-subrip"), EngineTrack("en", null, "text/vtt", sideloaded = true))
        assertEquals(0, TrackMatching.group(subs, engine, subs[0]))
        assertNull(TrackMatching.group(subs, engine, subs[1]))
        assertEquals(1, TrackMatching.sideloaded(engine))
    }

    @Test
    fun languageCodes() {
        assertTrue(TrackMatching.sameLanguage("ger", "de"))
        assertTrue(TrackMatching.sameLanguage("deu", "de"))
        assertTrue(TrackMatching.sameLanguage("fre", "fra"))
        assertTrue(TrackMatching.sameLanguage("und", "en"))
        assertTrue(TrackMatching.sameLanguage(null, "en"))
        assertFalse(TrackMatching.sameLanguage("eng", "de"))
    }

    @Test
    fun trickplayTiles() {
        val set = Trickplay(
            version = "v", state = Trickplay.State.RUNNING, intervalMs = 10_000, tileWidth = 320, tileHeight = 180,
            columns = 2, rows = 2, pointsMs = (0L..8L).map { it * 10_000 },
            sheets = listOf(TrickplaySheet(0, "/api/v1/items/a/trickplay/0?v=v"), TrickplaySheet(1, "/api/v1/items/a/trickplay/1?v=v")),
        )
        val tiles = TrickplayTiles(set) { "http://server.test$it" }
        assertEquals(TrickplayTile("http://server.test/api/v1/items/a/trickplay/0?v=v", 0, 0, 320, 180, 0), tiles.at(4_000))
        assertEquals(TrickplayTile("http://server.test/api/v1/items/a/trickplay/0?v=v", 320, 180, 320, 180, 30_000), tiles.at(35_000))
        assertEquals(TrickplayTile("http://server.test/api/v1/items/a/trickplay/1?v=v", 0, 0, 320, 180, 40_000), tiles.at(40_000))
        // Sheet 2 is not made yet.
        assertNull(tiles.at(85_000))
    }

    @Test
    fun serverPaths() {
        val server = okhttp3.HttpUrl.Builder().scheme("http").host("media.example.test").addPathSegment("sub").addPathSegment("").build()
        assertEquals("http://media.example.test/sub/api/v1/items/a/stream?st=x", ServerPaths.absolute(server, "/api/v1/items/a/stream?st=x"))
        assertNull(ServerPaths.absolute(server, "http://other.example.test/api/v1/x"))
    }
}
