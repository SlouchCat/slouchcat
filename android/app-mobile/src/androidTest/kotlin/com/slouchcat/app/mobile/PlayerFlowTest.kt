package com.slouchcat.app.mobile

import android.graphics.Bitmap
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.test.click
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performTouchInput
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.SetupRequest
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
import java.io.InputStreamReader

/**
 * The player in the emulator with real video (K27 stage 5): scripts/android-e2e.sh writes the
 * clips of scripts/android-test-video.sh; the test adds them as a library, signs the app in and
 * plays: the film directly (the position runs), seeking, pause, the other audio track, subtitles,
 * a lower quality (an HLS transcode), back to the detail page with its new progress (and the
 * server has it); an episode with «Vorspann überspringen», the card of the next episode and the
 * next episode. It logs the time to the first picture of each start (tag `Slouchcat/player`).
 * With `slouchcatShots` it photographs the player, each sheet and the marks (prefix `p`).
 */
@RunWith(AndroidJUnit4::class)
class PlayerFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val media = args.getString("slouchcatMedia")
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val context = InstrumentationRegistry.getInstrumentation().targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = 20_000) = waitFor(hasText(text, substring = true), timeout)

    private fun shown(matcher: SemanticsMatcher) = compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    /** A button: its label and its click sit in one node only in the merged tree. */
    private fun present(matcher: SemanticsMatcher) = compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty()

    private fun waitForButton(matcher: SemanticsMatcher, timeout: Long = 20_000) = compose.waitUntil(timeout) { present(matcher) }

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        InstrumentationRegistry.getInstrumentation().waitForIdleSync()
        val bitmap = InstrumentationRegistry.getInstrumentation().uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private var resumed: MainActivity? = null

    private fun back() = InstrumentationRegistry.getInstrumentation().runOnMainSync { resumed?.onBackPressedDispatcher?.onBackPressed() }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun jump(address: String) = InstrumentationRegistry.getInstrumentation().runOnMainSync { assertTrue(graph.links.request(address)) }

    /** The app's own session: what the server holds for the profile (no further sign-in). */
    private fun items() = SlouchcatApi.create(::ItemsApi, server, graph.http)

    private class Titles(val film: String, val episodes: List<String>, val episodeTitles: List<String>)

    private fun prepareServer() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        DetailFlowTest.Admin(server, email, password, pin).ensureLibrary(
            requireNotNull(media) { "run through scripts/android-e2e.sh, which writes the media" },
            name = LIBRARY,
            folder = "/Playback",
        )
    }

    /** The ids, read as the app; waits until the film and the episodes are probed and the intro is known. */
    private fun titles(): Titles {
        val items = items()
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val all = items.listLibraryItems(library.id)!!.items
        val film = all.single { it.title == "Two Voices" }.id
        val show = items.getItem(all.single { it.title == "Clip Show" }.id)!!
        val refs = items.getItem(show.seasons!!.single().id)!!.episodes!!.sortedBy { it.episodeNumber }
        val episodes = refs.map { it.id }
        val deadline = System.nanoTime() + PROBE_NS
        while ((listOf(film) + episodes).any { items.getItem(it)!!.mediaFiles.firstOrNull()?.videoCodec == null }) {
            check(System.nanoTime() < deadline) { "the test clips were not probed" }
            Thread.sleep(POLL_MS)
        }
        return Titles(film, episodes, refs.map { it.title })
    }

    private fun signIn() {
        compose.waitUntil(20_000) {
            ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading())
        }
        if (ready()) return
        if (shown(hasText(s(R.string.app_setup_title)))) {
            field(s(R.string.app_setup_addressLabel)).performTextInput(server.toString().trimEnd('/'))
            button(s(R.string.app_setup_connect)).performScrollTo().performClick()
            waitForText(s(R.string.app_setup_found))
            button(s(R.string.app_setup_next)).performScrollTo().performClick()
            waitFor(hasText(s(R.string.login_title)) and isHeading())
        }
        field(s(R.string.login_email)).performTextInput(email)
        field(s(R.string.login_password)).performTextInput(password)
        button(s(R.string.login_submit)).performScrollTo().performClick()
        compose.waitUntil(20_000) { ready() || shown(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()) }
        if (!ready()) {
            compose.onAllNodes(hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()).onLast().performClick()
            compose.waitUntil(20_000) { ready() || shown(hasText(s(R.string.profiles_pin_hint))) }
            if (!ready()) {
                pin.forEach { button(it.toString()).performClick() }
                compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
            }
        }
        compose.waitUntil(20_000) { ready() }
    }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    /** The position the server holds for [id] (`getItem`). */
    private fun serverPosition(id: String): Long = try {
        items().getItem(id)!!.progress?.positionMs ?: 0
    } catch (e: java.io.IOException) {
        // The emulator's network may blink under load (the gate then refuses one request): ask again.
        -1
    }

    private val close get() = hasContentDescription(s(R.string.app_player_close)) and hasClickAction()

    /**
     * A tap on the picture shows the controls (they hide while it plays): above the middle, where
     * no button sits, so it never pauses.
     */
    private fun showControls() {
        if (present(close)) return
        compose.onRoot().performTouchInput { click(Offset(width * 0.5f, height * 0.3f)) }
        waitForButton(close, 5_000)
    }

    private fun tool(label: Int) {
        showControls()
        press(s(label))
    }

    /**
     * The node's own click (a tap could land while the controls fade out), once the button is
     * there: play and pause hide while the player buffers after a jump.
     */
    private fun press(label: String) {
        val matcher = (hasText(label) or hasContentDescription(label)) and hasClickAction()
        waitForButton(matcher, 15_000)
        button(label).performSemanticsAction(SemanticsActions.OnClick)
    }

    /**
     * The times to the first picture the controller logged (`logcat` of the own process), also into
     * `first-pictures.txt` beside the photos (the measure of docs/perf/K27-android-handy.md).
     */
    private fun firstPictures(): List<String> {
        val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-v", "raw", "-s", "Slouchcat/player:I"))
        val lines = InputStreamReader(process.inputStream).readLines().filter { it.contains("first picture") }
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "first-pictures.txt").writeText(lines.joinToString("\n", postfix = "\n"))
        return lines
    }

    @After
    fun forgetTheServer() = runBlocking {
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
        graph.serverStore.clear()
    }

    @Test
    fun filmAndEpisodes() {
        prepareServer()
        Runtime.getRuntime().exec(arrayOf("logcat", "-c")).waitFor()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { resumed = it }
        signIn()
        val titles = titles()

        // The film plays directly; the position runs (the server has it after the first 10 s).
        jump("item/${titles.film}")
        waitFor(hasText("Two Voices") and isHeading())
        button(s(R.string.app_item_play)).performScrollTo().performClick()
        waitForButton(close)
        compose.waitUntil(20_000) { firstPictures().isNotEmpty() }
        showControls()
        shot("p01-player-controls")
        compose.waitUntil(30_000) { serverPosition(titles.film) >= 8_000 }
        compose.waitUntil(10_000) { !present(close) }
        shot("p02-player")
        tool(R.string.app_player_infoButton)
        waitForText(s(R.string.app_player_info_direct))
        shot("p03-sheet-info")
        press(s(R.string.common_close))

        // Seek to the middle through the bar's own action, then pause: the position stands.
        showControls()
        compose.onAllNodes(hasContentDescription(s(R.string.player_seek))).onFirst()
            .performSemanticsAction(SemanticsActions.SetProgress) { it(0.5f) }
        compose.waitUntil(15_000) {
            showControls()
            present((hasContentDescription(s(R.string.player_pause))) and hasClickAction())
        }
        press(s(R.string.player_pause))
        compose.waitUntil(10_000) { serverPosition(titles.film) in 29_000..40_000 }
        val paused = serverPosition(titles.film)

        // The other audio track (in the player: direct play, the groups match the server's tracks).
        tool(R.string.player_tracks_audio)
        waitForButton(isSelectable() and hasText("Deutsch", substring = true))
        shot("p04-sheet-audio")
        compose.onAllNodes(isSelectable() and hasText("English", substring = true)).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        // Subtitles from the file.
        tool(R.string.player_tracks_subtitles)
        waitForButton(isSelectable() and hasText(s(R.string.player_tracks_off)))
        shot("p05-sheet-subtitles")
        compose.onAllNodes(isSelectable() and hasText("Deutsch", substring = true)).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        tool(R.string.player_settings_speed)
        shot("p06-sheet-speed")
        press(s(R.string.common_close))
        tool(R.string.app_player_infoButton)
        waitForText("English")
        waitForText(s(R.string.app_player_info_direct))
        press(s(R.string.common_close))
        compose.waitUntil(15_000) {
            showControls()
            present((hasContentDescription(s(R.string.player_play))) and hasClickAction())
        }
        press(s(R.string.player_play))
        shot("p07-player-subtitles")

        // A lower quality: the server transcodes, HLS (a narrower profile than the device's).
        tool(R.string.player_settings_quality)
        waitForButton(isSelectable() and hasText("480p"))
        shot("p08-sheet-quality")
        compose.onAllNodes(isSelectable() and hasText("480p")).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        // The transcode's first picture (the measure of an HLS start).
        compose.waitUntil(40_000) { firstPictures().any { "hls" in it } }
        tool(R.string.app_player_infoButton)
        waitForText(s(R.string.app_player_info_transcode, "480p", s(R.string.player_streamInfo_reasons_quality)), 30_000)
        waitForText("HLS")
        shot("p09-sheet-info-hls")
        press(s(R.string.common_close))

        // Back: the detail page with «Weiterschauen»; the server has the position.
        showControls()
        compose.onAllNodes(close).onLast().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(hasText("Two Voices") and isHeading())
        waitForText(s(R.string.app_item_resume))
        shot("p10-detail-after")
        assertTrue("the server holds ${serverPosition(titles.film)}, paused at $paused", serverPosition(titles.film) >= paused)

        // An episode: the intro, its skip button, the card of the next episode, the next episode.
        jump("item/${titles.episodes[0]}")
        waitFor(hasText(titles.episodeTitles[0]) and isHeading())
        button(s(R.string.app_item_play)).performScrollTo().performClick()
        waitForButton(hasText(s(R.string.app_player_skipIntro)) and hasClickAction(), 40_000)
        showControls()
        shot("p11-player-intro")
        press(s(R.string.app_player_skipIntro))
        tool(R.string.app_player_episodes)
        waitForText(s(R.string.app_player_nowPlaying))
        shot("p12-sheet-episodes")
        press(s(R.string.common_close))
        showControls()
        compose.onAllNodes(hasContentDescription(s(R.string.player_seek))).onFirst()
            .performSemanticsAction(SemanticsActions.SetProgress) { it(44f / 70f) }
        waitForButton(hasText(s(R.string.app_player_playNow)) and hasClickAction(), 20_000)
        shot("p13-player-countdown")
        // The countdown plays the next episode by itself.
        waitFor(hasText(titles.episodeTitles[1], substring = true) and isHeading(), 30_000)
        // Its first picture: four starts by now (the film, its lower quality, the two episodes).
        compose.waitUntil(20_000) { firstPictures().size >= 4 }
        shot("p14-player-next-episode")
        showControls()
        compose.onAllNodes(close).onLast().performSemanticsAction(SemanticsActions.OnClick)
        waitFor(hasText(titles.episodeTitles[0]) and isHeading())

        // «Mehr › Einstellungen › Wiedergabe»: the device's qualities and the profile's languages.
        jump("settings/playback")
        waitFor(hasText(s(R.string.app_playback_autoplay)))
        waitForText(s(R.string.app_playback_audioLanguage))
        shot("p15-settings-playback")
        press(s(R.string.app_playback_mobile))
        waitForButton(isSelectable() and hasText(s(R.string.app_playback_choice_saver)))
        shot("p16-settings-quality-sheet")
        press(s(R.string.common_cancel))
        press(s(R.string.settings_playback_subtitleMode_label))
        waitForButton(isSelectable() and hasText(s(R.string.settings_playback_subtitleMode_always)))
        shot("p17-settings-subtitles-sheet")
        compose.onAllNodes(isSelectable() and hasText(s(R.string.settings_playback_subtitleMode_always))).onFirst()
            .performSemanticsAction(SemanticsActions.OnClick)
        waitForText(s(R.string.settings_playback_saved))
        back()

        // Each start logged its first picture: direct, HLS (the lower quality), the episodes.
        val measured = firstPictures()
        assertTrue("too few first pictures: $measured", measured.size >= 4)
        assertTrue("no HLS start: $measured", measured.any { it.contains("hls") })
    }

    private companion object {
        const val LIBRARY = "Android playback media"
        const val PROBE_NS = 120_000_000_000L
        const val POLL_MS = 500L
    }
}
