package com.slouchcat.app.mobile.audio

import android.app.Application
import android.content.Intent
import android.media.AudioManager
import androidx.media3.common.Player
import androidx.media3.common.C
import androidx.media3.common.util.UnstableApi
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.EngineKind
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.TrustedControllers
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.player.testing.station
import com.slouchcat.core.api.ApiResult
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.Robolectric
import org.robolectric.RobolectricTestRunner
import org.robolectric.Shadows.shadowOf
import org.robolectric.annotation.Config
import org.robolectric.shadows.ShadowLooper

/** An application with one [AudioPlayback] over Media3 players and fake servers (no session, no network). */
class AudioTestApp : Application(), AudioHost {
    val radioServer = FakeRadioServer()
    val titles = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id)) } }
    val engines = mutableListOf<Media3Engine>()
    private val facts = DeviceFacts(listOf(DecoderInfo("audio/mp4a-latm")), emptySet(), 2, DisplayInfo(1080, 2400), DeviceType.Phone, "Test")

    val audioHttp = OkHttpClient()

    @androidx.annotation.OptIn(UnstableApi::class)
    override val audioPlayback: AudioPlayback by lazy {
        AudioPlayback(
            scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate),
            engines = { kind -> Media3Engine(this, audioHttp, withSession = false, kind = kind).also { engines += it } },
            radioServer = { radioServer },
            capabilities = { CapabilityRules.build(facts, narrowing = it) },
            settings = MemoryDeviceSettingsStore().settings,
            clock = { 0L },
        )
    }

    val online = AudioSource("online") { titles }
}

/**
 * The media session service of stage 7: no session without something loaded (never a start of its
 * own), the session over the player of what plays, moved to the other player at a change, the end
 * when nothing is loaded any more; the players carry the audio attributes of speech and music.
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], application = AudioTestApp::class)
@androidx.annotation.OptIn(UnstableApi::class)
class AudioServiceTest {
    private val app: AudioTestApp get() = androidx.test.core.app.ApplicationProvider.getApplicationContext()

    private fun idle() = ShadowLooper.idleMainLooper()

    /** Runs the main looper until [done] holds (at most 5 s): the player answers some events from its own thread. */
    private fun until(done: () -> Boolean) {
        val end = System.nanoTime() + 5_000_000_000L
        while (System.nanoTime() < end) {
            idle()
            if (done()) return
            Thread.onSpinWait()
        }
    }

    @Test
    fun withNothingLoadedTheServiceEndsAtOnce() {
        val service = Robolectric.buildService(AudioService::class.java).create().get()
        assertTrue(shadowOf(service).isStoppedBySelf)
        assertTrue(service.sessions.isEmpty())
    }

    @Test
    fun theSessionFollowsWhatPlaysAndEndsWithIt() {
        app.audioPlayback.playTitle("book", app.online)
        idle()
        val controller = Robolectric.buildService(AudioService::class.java).create()
        val service = controller.get()
        val book = app.engines.single()
        assertSame(book.player, service.sessions.single().player)
        assertEquals(C.AUDIO_CONTENT_TYPE_SPEECH, book.player.audioAttributes.contentType)
        assertFalse(shadowOf(service).isStoppedBySelf)

        // A station: the session moves to the radio's player, the book's player is released.
        app.audioPlayback.playRadio(station(7))
        idle()
        val radio = app.engines.last()
        assertSame(radio.player, service.sessions.single().player)
        assertEquals(C.AUDIO_CONTENT_TYPE_MUSIC, radio.player.audioAttributes.contentType)
        assertEquals(listOf("start:7"), app.radioServer.calls.take(1))

        app.audioPlayback.stop()
        idle()
        assertTrue(shadowOf(service).isStoppedBySelf)
        assertTrue(service.sessions.isEmpty())
        controller.destroy()
    }

    @Test
    fun swipedAwayWhilePausedEndsTheSoundAndTheService() {
        app.audioPlayback.playTitle("book", app.online)
        idle()
        val service = Robolectric.buildService(AudioService::class.java).create().get()
        app.engines.single().pause()
        service.onTaskRemoved(null)
        idle()
        // Code review m-3: not only the service, the sound too, with its end at the server.
        assertTrue(shadowOf(service).isStoppedBySelf)
        assertNull(app.audioPlayback.now.value)
        assertTrue(app.titles.stops.isNotEmpty())
        assertTrue(service.sessions.isEmpty())
    }

    @Test
    fun swipedAwayWhilePlayingPlaysOn() {
        app.audioPlayback.playTitle("book", app.online)
        idle()
        val service = Robolectric.buildService(AudioService::class.java).create().get()
        app.engines.single().play()
        service.onTaskRemoved(null)
        idle()
        assertFalse(shadowOf(service).isStoppedBySelf)
        assertNotNull(app.audioPlayback.now.value)
        assertTrue(app.titles.stops.isEmpty())
        assertEquals(1, service.sessions.size)
    }

    @Test
    fun headphonesPulledOutPauseTheSound() {
        app.audioPlayback.playTitle("book", app.online)
        idle()
        val player = app.engines.single().player
        player.play()
        idle()
        // Media3 registers its receiver from the player's background thread (AudioBecomingNoisyManager
        // posts registerReceiver there): a broadcast sent before that reaches nobody.
        val noisy = Intent(AudioManager.ACTION_AUDIO_BECOMING_NOISY)
        until { shadowOf(app).hasReceiverForIntent(noisy) }
        app.sendBroadcast(noisy)
        // The player takes the broadcast on its own thread and pauses from there: one pass of the main
        // looper can come too early (it did once on a CI runner).
        until { !player.playWhenReady }
        assertFalse(player.playWhenReady)
    }

    @Test
    fun theSessionTurnsAwayStrangersAndTakesNoMedia() {
        val own = 10_123
        // By UID, not by the package a connection names (security review L-1).
        assertEquals(TrustedControllers.Level.Own, TrustedControllers.level(own, own, notification = false, trusted = false))
        assertEquals(TrustedControllers.Level.Own, TrustedControllers.level(own, 1_000, notification = true, trusted = true))
        assertEquals(TrustedControllers.Level.Trusted, TrustedControllers.level(own, 10_456, notification = false, trusted = true))
        assertEquals(TrustedControllers.Level.None, TrustedControllers.level(own, 10_456, notification = false, trusted = false))
        assertFalse(TrustedControllers.COMMANDS.contains(Player.COMMAND_SET_MEDIA_ITEM))
        // A trusted app (also one with access to the notifications) plays, pauses, jumps and stops.
        listOf(Player.COMMAND_PLAY_PAUSE, Player.COMMAND_SEEK_BACK, Player.COMMAND_SEEK_FORWARD, Player.COMMAND_STOP).forEach {
            assertTrue(TrustedControllers.TRUSTED_COMMANDS.contains(it))
        }
        listOf(Player.COMMAND_SET_MEDIA_ITEM, Player.COMMAND_CHANGE_MEDIA_ITEMS, Player.COMMAND_SET_SPEED_AND_PITCH, Player.COMMAND_SEEK_IN_CURRENT_MEDIA_ITEM).forEach {
            assertFalse(TrustedControllers.TRUSTED_COMMANDS.contains(it))
        }
        assertNotNull(EngineKind.Speech)
    }

    @Test
    fun theServiceIsNotExported() {
        val info = app.packageManager.getServiceInfo(android.content.ComponentName(app, AudioService::class.java), 0)
        assertFalse(info.exported)
    }
}
