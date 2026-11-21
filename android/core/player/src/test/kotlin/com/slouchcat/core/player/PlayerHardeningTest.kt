package com.slouchcat.core.player

import androidx.media3.common.Player
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.api.log.LogSink
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

/**
 * The hardening of the security review of stage 5: addresses, media session, Media3's own log.
 * Robolectric: Media3's command sets live in an Android `SparseBooleanArray`.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36])
class PlayerHardeningTest {
    private val before = Log.sink

    @After
    fun sink() {
        Log.sink = before
    }

    @Test
    fun onlyTheServersOwnOriginAndOnlyHttp() {
        val https = "https://media.example.test:8443/".toHttpUrl()
        assertEquals("https://media.example.test:8443/api/v1/items/a/stream?st=x", ServerPaths.absolute(https, "/api/v1/items/a/stream?st=x"))
        assertEquals("https://media.example.test:8443/api/v1/x?st=y", ServerPaths.absolute(https, "https://media.example.test:8443/api/v1/x?st=y"))
        // The same host unencrypted, another port, another host, a scheme-relative address: none.
        assertNull(ServerPaths.absolute(https, "http://media.example.test:8443/api/v1/x?st=y"))
        assertNull(ServerPaths.absolute(https, "https://media.example.test:9443/api/v1/x?st=y"))
        assertNull(ServerPaths.absolute(https, "https://other.example.test:8443/api/v1/x?st=y"))
        assertNull(ServerPaths.absolute(https, "//other.example.test/api/v1/x?st=y"))
        assertNull(ServerPaths.absolute(https, "ftp://media.example.test:8443/x"))
    }

    @Test
    fun theMediaSessionTakesOnlyTrustedControllers() {
        assertTrue(TrustedControllers.accepts("com.slouchcat.app", "com.slouchcat.app", notification = false, trusted = false))
        assertTrue(TrustedControllers.accepts("com.slouchcat.app", "com.android.systemui", notification = true, trusted = false))
        assertTrue(TrustedControllers.accepts("com.slouchcat.app", "com.android.bluetooth", notification = false, trusted = true))
        assertFalse(TrustedControllers.accepts("com.slouchcat.app", "org.example.listener", notification = false, trusted = false))
        val commands = TrustedControllers.COMMANDS
        assertTrue(commands.contains(Player.COMMAND_PLAY_PAUSE))
        assertFalse(commands.contains(Player.COMMAND_SET_MEDIA_ITEM))
        assertFalse(commands.contains(Player.COMMAND_CHANGE_MEDIA_ITEMS))
    }

    @Test
    fun media3sOwnMessagesLoseTheStreamToken() {
        val lines = mutableListOf<String>()
        Log.sink = object : LogSink {
            override fun write(priority: Int, tag: String, message: String) {
                lines += "$tag $message"
            }
        }
        Media3Logs.e("HlsPlaylistParser", "Unexpected line: seg/3.m4s?st=secret-token", IllegalStateException("init.mp4?st=secret-token"))
        Media3Logs.w("ExoPlayerImpl", "Playback error at http://server.test/api/v1/items/a/stream?st=secret-token", null)
        Media3Logs.i("ExoPlayerImpl", "url ?st=secret-token", null)
        assertEquals(2, lines.size)
        assertTrue(lines.none { "secret-token" in it })
        assertTrue(lines.first().startsWith("media3/HlsPlaylistParser"))
    }
}
