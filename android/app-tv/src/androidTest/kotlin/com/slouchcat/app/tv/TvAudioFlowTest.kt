package com.slouchcat.app.tv

import android.graphics.Bitmap
import android.os.SystemClock
import android.view.KeyEvent
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeTimeoutException
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.performImeAction
import androidx.compose.ui.test.performTextInput
import androidx.lifecycle.Lifecycle
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.tv.audio.AudioFeature
import com.slouchcat.core.session.DataStoreRelaySettings
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.player.AudioPlayback
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * Stage 4 «Ton» in the TV emulator against a real server (scripts/android-tv-e2e.sh, concept 26
 * 5.6): the audiobook and the podcast of scripts/android-test-video.sh as libraries of their own;
 * the book plays in the audio player, Next of the remote changes the chapter, Home ends the
 * session and its progress is at the server, back in front it waits paused; an episode of the
 * podcast plays; with `slouchcatRadio` (RADIO=1) a test station starts from «Webradio», its page
 * shows it, «Radio beenden» ends it. Each test stands on its own: it sets the TV up when it is not
 * and signs it out at the end (one web session for the administration, [TestLibrary]).
 *
 * Arguments as TvFlowTest; `slouchcatShots` saves photos into the app's `files/shots/` (`s4…`).
 */
@RunWith(AndroidJUnit4::class)
class TvAudioFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8697").toHttpUrl()
    private val relay: HttpUrl = (args.getString("slouchcatRelay") ?: "http://10.0.2.2:8180").toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val media = args.getString("slouchcatMedia")
    private val radio = args.getString("slouchcatRadio") != null
    private val context = instrumentation.targetContext
    private val graph: TvGraph get() = (context.applicationContext as TvApplication).graph

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun playback(): AudioPlayback = graph.registry.features.filterIsInstance<AudioFeature>().single().audioOf(graph, context).playback

    private fun now(): AudioNow? {
        var now: AudioNow? = null
        instrumentation.runOnMainSync { now = playback().now.value }
        return now
    }

    private fun title(): AudioNow.Title? = now() as? AudioNow.Title

    private fun prepareServer() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        val library = TestLibrary(server, email, password, pin)
        library.primaryWithPin()
        val folder = requireNotNull(media) { "run through scripts/android-tv-e2e.sh, which writes the media" }
        library.ensureAudio(BOOKS, "audiobooks", "$folder/Playback/Audiobooks", "audiobook")
        library.ensureAudio(PODCASTS, "podcasts", "$folder/Playback/Podcasts", "podcast")
        if (radio) library.ensureRadio()
    }

    private fun isFocusedOn(text: String): Boolean =
        compose.onAllNodes(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true))).fetchSemanticsNodes().isNotEmpty()

    private fun present(text: String): Boolean =
        compose.onAllNodes(hasText(text, substring = true) or hasContentDescription(text, substring = true), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = WAIT_MS, unmerged: Boolean = true) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = unmerged).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = WAIT_MS) = waitFor(hasText(text, substring = true) or hasContentDescription(text, substring = true), timeout)

    private fun key(code: Int, times: Int = 1) = repeat(times) {
        instrumentation.sendKeyDownUpSync(code)
        compose.waitForIdle()
    }

    private fun focused(text: String) {
        try {
            waitFor(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true)), unmerged = false)
        } catch (timeout: ComposeTimeoutException) {
            val now = compose.onAllNodes(isFocused()).fetchSemanticsNodes().map { node ->
                (node.config.getOrNull(SemanticsProperties.ContentDescription).orEmpty() + node.config.getOrNull(SemanticsProperties.Text).orEmpty().map { it.text }).joinToString(" ")
            }
            throw AssertionError("«$text» should have the focus, it is on $now", timeout)
        }
    }

    private fun rail(label: String) {
        key(KeyEvent.KEYCODE_DPAD_LEFT, LEFT_MAX)
        key(KeyEvent.KEYCODE_DPAD_UP, RAIL_MAX)
        repeat(RAIL_MAX) {
            if (isFocusedOn(label)) {
                key(KeyEvent.KEYCODE_DPAD_CENTER)
                return
            }
            key(KeyEvent.KEYCODE_DPAD_DOWN)
        }
        throw AssertionError("no entry «$label» in the rail")
    }

    /** The card [text] of a grid: down into it, then along it. */
    private fun toCard(text: String) {
        repeat(ROW_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_DOWN) }
        repeat(ROW_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        repeat(ROW_MAX * 2) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_LEFT) }
        focused(text)
    }

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        SystemClock.sleep(FRAME_MS)
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun type(label: String, text: String) {
        focused(label)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasSetTextAction() and hasContentDescription(label))
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performTextInput(text)
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performImeAction()
        compose.waitForIdle()
    }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    /** From the start of the app to «Start»: set up by hand when the TV is not, then the profile with its PIN. */
    private fun launch(): ActivityScenario<MainActivity> {
        runBlocking { DataStoreRelaySettings(context, "tv").set(relay) }
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        compose.waitUntil(START_MS) { ready() || present(s(R.string.tv_setup_welcome)) || present(s(R.string.profiles_picker_title)) }
        if (!ready() && present(s(R.string.tv_setup_welcome))) {
            focused(s(R.string.tv_setup_phoneTitle))
            key(KeyEvent.KEYCODE_DPAD_RIGHT)
            key(KeyEvent.KEYCODE_DPAD_CENTER)
            waitForText(s(R.string.tv_hand_serverLead))
            type(s(R.string.tv_hand_serverLabel), server.toString().trimEnd('/'))
            waitForText(server.host)
            focused(s(R.string.tv_hand_next))
            key(KeyEvent.KEYCODE_DPAD_CENTER)
            waitForText(s(R.string.tv_hand_emailLead))
            type(s(R.string.tv_hand_emailLabel), email)
            focused(s(R.string.tv_hand_next))
            key(KeyEvent.KEYCODE_DPAD_CENTER)
            waitForText(s(R.string.tv_hand_passwordLead, server.host))
            type(s(R.string.tv_hand_passwordLabel), password)
            focused(s(R.string.tv_hand_signIn))
            key(KeyEvent.KEYCODE_DPAD_CENTER)
        }
        if (!ready()) {
            waitForText(s(R.string.profiles_picker_title))
            key(KeyEvent.KEYCODE_DPAD_CENTER)
            waitForText(s(R.string.tv_pin_prompt))
            pin.forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
            focused("1")
            key(KeyEvent.KEYCODE_DPAD_DOWN, 3)
            key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
            focused(s(R.string.profiles_pin_submit))
            key(KeyEvent.KEYCODE_DPAD_CENTER)
        }
        compose.waitUntil(START_MS) { ready() }
        return scenario
    }

    /** The audiobook as the TV's session sees it at the server. */
    private fun serverPosition(name: String): Long {
        val items = SlouchcatApi.create(::ItemsApi, server, graph.http)
        val library = items.listVisibleLibraries().orEmpty().first { it.name == BOOKS }
        val id = items.listLibraryItems(library.id)!!.items.first { it.title == name }.id
        return items.getItem(id)!!.progress?.positionMs ?: 0
    }

    @After
    fun cleanUp() = runBlocking {
        instrumentation.runOnMainSync { playback().stop() }
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
    }

    /** The audiobook: page, audio player, chapter with Next, Home ends the session with its progress, paused back. */
    @Test
    fun audiobookPlaysAChapterAndHomeKeepsTheProgress() {
        prepareServer()
        val scenario = launch()
        rail(BOOKS)
        waitForText(BOOK)
        shot("s4-01-library-books")
        toCard(BOOK)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.item_audio_chapters))
        shot("s4-02-book")
        // «Anhören» or «Weiterhören» (an earlier run), the big button has the focus.
        compose.waitUntil(WAIT_MS) { isFocusedOn(s(R.string.tv_audio_listen)) || isFocusedOn(s(R.string.item_action_resumeListening)) || isFocusedOn(s(R.string.item_action_againListening)) }
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        compose.waitUntil(PLAY_MS) { title()?.state?.playing == true }
        focused(s(R.string.player_pause))
        shot("s4-03-listen-book")
        // Next of the remote: the second chapter (one per file, 60 s each).
        key(KeyEvent.KEYCODE_MEDIA_NEXT)
        compose.waitUntil(PLAY_MS) { (title()?.state?.positionMs ?: 0) >= CHAPTER_MS }
        // Back: the page closes, the book plays on with «Läuft gerade».
        key(KeyEvent.KEYCODE_BACK)
        waitForText(s(R.string.tv_audio_nowPlaying))
        assertTrue(title()?.state?.playing == true)
        shot("s4-04-now-playing")
        // Home: the session ends with the last progress, at the server it is in the second chapter.
        scenario.moveToState(Lifecycle.State.CREATED)
        compose.waitUntil(STOP_MS) { serverPosition(BOOK) >= CHAPTER_MS }
        scenario.moveToState(Lifecycle.State.RESUMED)
        compose.waitUntil(PLAY_MS) { title()?.state?.let { !it.loading && !it.playing } == true }
        scenario.close()
    }

    /** The podcast: its page with the episodes, «Neueste Folge» plays in the audio player. */
    @Test
    fun podcastEpisodePlays() {
        prepareServer()
        val scenario = launch()
        rail(PODCASTS)
        waitForText(PODCAST)
        toCard(PODCAST)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.item_episodes_title))
        waitForText(s(R.string.tv_audio_new))
        shot("s4-05-podcast")
        focused(s(R.string.app_audio_newest))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        compose.waitUntil(PLAY_MS) { title()?.state?.playing == true }
        assertTrue(title()?.state?.item?.kind == com.slouchcat.core.api.generated.models.ItemKind.PODCAST_EPISODE)
        // The audio player of an episode: «Podcast · <date>» over it.
        waitForText(s(R.string.kind_podcast).uppercase())
        shot("s4-06-listen-episode")
        key(KeyEvent.KEYCODE_BACK)
        scenario.close()
    }

    /** The radio (RADIO=1): a test station starts with OK, its page, «Radio beenden». */
    @Test
    fun radioStartsAndEnds() {
        if (!radio) return
        prepareServer()
        val scenario = launch()
        rail(s(R.string.app_radio_title))
        waitForText(s(R.string.tv_radio_hint))
        compose.waitUntil(WAIT_MS) { compose.onAllNodes(isFocused()).fetchSemanticsNodes().isNotEmpty() }
        shot("s4-07-radio")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        compose.waitUntil(PLAY_MS) { (now() as? AudioNow.Radio)?.state?.playing == true }
        waitForText(s(R.string.radio_page_playing))
        shot("s4-08-radio-playing")
        key(KeyEvent.KEYCODE_DPAD_UP, 3)
        focused(s(R.string.radio_player_nowPlaying))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.radio_player_pause))
        shot("s4-09-radio-listen")
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.radio_player_stop))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        compose.waitUntil(WAIT_MS) { now() == null }
        waitForText(s(R.string.tv_radio_hint))
        scenario.close()
    }

    private companion object {
        const val BOOKS = "Hörbücher-Test"
        const val PODCASTS = "Podcasts-Test"
        const val BOOK = "The Quiet Harbour"
        const val PODCAST = "Night Shift Stories"
        const val CHAPTER_MS = 60_000L
        const val WAIT_MS = 30_000L
        const val START_MS = 60_000L
        const val PLAY_MS = 90_000L
        const val STOP_MS = 20_000L
        const val FRAME_MS = 500L
        const val RAIL_MAX = 14
        const val LEFT_MAX = 8
        const val ROW_MAX = 4
    }
}
