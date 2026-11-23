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
import androidx.media3.common.util.UnstableApi
import androidx.test.core.app.ApplicationProvider
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.home.HomeFeature
import com.slouchcat.app.mobile.more.MoreFeature
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
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
import com.slouchcat.core.player.Media3Engine
import com.slouchcat.core.player.MemoryDeviceSettingsStore
import com.slouchcat.core.player.testing.FakeRadioServer
import com.slouchcat.core.player.testing.FakeServer
import com.slouchcat.core.player.testing.audioInfo
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import okhttp3.OkHttpClient
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

private fun s(id: Int, vararg args: Any): String = ApplicationProvider.getApplicationContext<Context>().getString(id, *args)

/**
 * The sound in the real NavHost: opening the audio player from the small player only shows what
 * plays (code review M-1: no new start, no jump to the beginning); a video ends the sound with its
 * session at the server (M-3).
 */
@RunWith(RobolectricTestRunner::class)
@Config(sdk = [36], qualifiers = "w412dp-h915dp")
@androidx.annotation.OptIn(UnstableApi::class)
class AudioRoutesTest {
    @get:Rule
    val compose = createAndroidComposeRule<ComponentActivity>()

    private val context = ApplicationProvider.getApplicationContext<Context>()
    private val titles = FakeServer().apply { answer = { id, _ -> ApiResult.Ok(audioInfo(id)) } }
    private val facts = DeviceFacts(listOf(DecoderInfo("audio/mp4a-latm")), emptySet(), 2, DisplayInfo(1080, 2400), DeviceType.Phone, "Test")
    private val http = OkHttpClient()
    private val playback = AudioPlayback(
        scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate),
        engines = { kind -> Media3Engine(context, http, withSession = false, kind = kind) },
        radioServer = { FakeRadioServer() },
        capabilities = { CapabilityRules.build(facts, narrowing = it) },
        settings = MemoryDeviceSettingsStore().settings,
        clock = { 0L },
    )
    private val online = AudioSource("online") { titles }

    private val graph = AppGraph(
        context,
        features = listOf(HomeFeature, MoreFeature, AudioFeature),
        serverStore = FakeServers(HTTPS_SERVER),
        tokenStore = FakeTokens(StoredCredentials("token", "device-1")),
        accountStore = FakeAccounts(EMAIL, PRIMARY),
        librariesRepository = FakeLibraries(ApiResult.Ok(emptyList())),
        audioData = { AudioData(playback, online) },
    )

    private fun opener(title: String) = SemanticsMatcher("opens the audio player") {
        it.config.getOrNull(SemanticsActions.OnClick)?.label == s(R.string.app_audio_open, title)
    }

    @Test
    fun theSmallPlayerOpensThePlayerWithoutANewStart() {
        compose.setContent { SlouchcatNavHost(graph) }
        compose.runOnUiThread { playback.playTitle("book", online) }
        compose.waitUntil(10_000) { (playback.now.value as? AudioNow.Title)?.state?.item != null }
        val title = (playback.now.value as AudioNow.Title).state.item!!.title
        compose.waitUntil(10_000) { compose.onAllNodes(opener(title)).fetchSemanticsNodes().isNotEmpty() }
        compose.onAllNodes(opener(title)).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        // The audio player is open: its close button is there.
        compose.waitUntil(10_000) { compose.onAllNodes(hasContentDescription(s(R.string.app_audio_close))).fetchSemanticsNodes().isNotEmpty() }
        compose.waitForIdle()
        assertEquals(listOf("book"), titles.starts.map { it.first })
        assertTrue(titles.stops.isEmpty())
    }

    @Test
    fun aVideoEndsTheSound() {
        compose.setContent { SlouchcatNavHost(graph) }
        compose.runOnUiThread { playback.playTitle("book", online) }
        compose.waitUntil(10_000) { titles.starts.isNotEmpty() && (playback.now.value as? AudioNow.Title)?.state?.item != null }
        compose.runOnUiThread { graph.stopSound() }
        compose.waitUntil(10_000) { titles.stops.isNotEmpty() }
        assertNull(playback.now.value)
    }
}
