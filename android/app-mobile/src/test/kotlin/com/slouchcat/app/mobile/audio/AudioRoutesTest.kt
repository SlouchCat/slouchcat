package com.slouchcat.app.mobile.audio

import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.performSemanticsAction
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.browse.PlayerRoute
import com.slouchcat.app.mobile.home.HomeFeature
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.player.PlayerFeature
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.testing.FakeOfflineServer
import com.slouchcat.app.mobile.testing.offlineIdentity
import com.slouchcat.app.mobile.testing.phoneServer
import com.slouchcat.app.mobile.testing.testEngine
import com.slouchcat.app.mobile.testing.testOfflineData
import com.slouchcat.core.api.ApiResult
import com.slouchcat.core.browse.testing.EMAIL
import com.slouchcat.core.browse.testing.FakeAccounts
import com.slouchcat.core.browse.testing.FakeLibraries
import com.slouchcat.core.browse.testing.FakeServers
import com.slouchcat.core.browse.testing.FakeTokens
import com.slouchcat.core.browse.testing.HTTPS_SERVER
import com.slouchcat.core.browse.testing.PRIMARY
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.player.AudioSource
import com.slouchcat.core.player.CapabilityRules
import com.slouchcat.core.player.DecoderInfo
import com.slouchcat.core.player.DeviceFacts
import com.slouchcat.core.player.DeviceType
import com.slouchcat.core.player.DisplayInfo
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeEngine
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config
import kotlin.coroutines.EmptyCoroutineContext

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/**
 * The sound in the real NavHost with the graph's hooks (`AudioData.attach`): opening the audio
 * player from the small player only shows what plays (code review M-1: no new start, no change of
 * play or pause); opening a video ends the sound with its session at the server (M-3, the wiring of
 * `PlayerFeature`); another profile opened offline ends it too (security review M-1).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
class AudioRoutesTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    @get:Rule
    val temp = TemporaryFolder()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val io = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val titles = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id)) } }
    private val facts = DeviceFacts(listOf(DecoderInfo("audio/mp4a-latm")), emptySet(), 2, DisplayInfo(1080, 2400), DeviceType.Phone, "Test")
    private val engines = mutableListOf<FakeEngine>()
    private val playback = AudioPlayback(
        scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate),
        engines = { FakeEngine { 0L }.also { engines += it } },
        radioServer = { FakeRadioServer() },
        capabilities = { CapabilityRules.build(facts, narrowing = it) },
        settings = MemoryDeviceSettingsStore().settings,
        clock = { 0L },
    )
    private val online = AudioSource("online") { titles }

    private val graph = AppGraph(
        context,
        features = listOf(
            HomeFeature,
            MoreFeature,
            AudioFeature,
            PlayerFeature(
                engine = { _, _ -> FakeEngine { 0L } },
                server = { phoneServer() },
                settings = { MemoryDeviceSettingsStore() },
                facts = { _, type -> DeviceFacts(emptyList(), emptySet(), 2, DisplayInfo(2424, 1080), type, "Test phone") },
                background = EmptyCoroutineContext,
            ),
        ),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        librariesRepository = FakeLibraries(ApiResult.Ok(emptyList())),
        offlineData = { testOfflineData(testEngine(context, temp.newFolder("offline"), FakeOfflineServer(), io, { offlineIdentity(PRIMARY.id) })) },
        audioData = { graph -> AudioData(playback, online).also { it.attach(graph) } },
    )

    @After
    fun tearDown() = io.cancel()

    private fun opener(title: String) = SemanticsMatcher("opens the audio player") {
        it.config.getOrNull(SemanticsActions.OnClick)?.label == s(R.string.app_audio_open, title)
    }

    /** The book plays (its fake player is ready and runs); returns its title. */
    private fun playing(): String {
        compose.setContent { SlouchcatNavHost(graph) }
        compose.runOnUiThread {
            graph.audio
            playback.playTitle("book", online)
        }
        compose.waitUntil(10_000) { engines.isNotEmpty() && engines.last().loads.isNotEmpty() }
        compose.runOnUiThread { engines.last().ready() }
        compose.waitUntil(10_000) { (playback.now.value as? AudioNow.Title)?.state?.playing == true }
        return (playback.now.value as AudioNow.Title).state.item!!.title
    }

    @Test
    fun theSmallPlayerOpensThePlayerWithoutANewStart() {
        val title = playing()
        val engine = engines.single()
        val seeks = engine.seeks.toList()
        compose.waitUntil(10_000) { compose.onAllNodes(opener(title)).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(opener(title)).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        // The audio player is open: its close button is there.
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription(s(R.string.app_audio_close))).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
        assertEquals(listOf("book"), titles.starts.map { it.first })
        assertTrue(titles.stops.isEmpty())
        // The same player plays on: no new load, no jump, no pause.
        assertEquals(listOf(engine), engines)
        assertEquals(1, engine.loads.size)
        assertEquals(seeks, engine.seeks)
        assertTrue(engine.wantsPlay)
        assertEquals(true, (playback.now.value as AudioNow.Title).state.playing)
    }

    @Test
    fun aVideoEndsTheSound() {
        playing()
        // The video page (PlayerFeature) opens: its start ends the sound.
        compose.runOnUiThread { graph.links.request(AppLink("start", listOf(PlayerRoute("film")))) }
        compose.waitUntil(10_000) { titles.stops.isNotEmpty() }
        compose.waitUntil(10_000) { playback.now.value == null }
        assertTrue(engines.single().released)
    }

    @Test
    fun anotherProfileOpenedOfflineEndsTheSound() {
        playing()
        // Downloads › Profil wechseln: another profile opens offline, the session stays (security review M-1).
        compose.runOnUiThread { graph.offline.viewer.value = PRIMARY.id + 1 }
        compose.waitUntil(10_000) { playback.now.value == null }
        assertNull(playback.now.value)
        compose.waitUntil(10_000) { titles.stops.isNotEmpty() }
    }
}
