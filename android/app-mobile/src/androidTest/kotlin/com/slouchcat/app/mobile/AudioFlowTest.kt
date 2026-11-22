package com.slouchcat.app.mobile

import android.accessibilityservice.AccessibilityService
import android.content.ComponentName
import android.content.Intent
import android.graphics.Bitmap
import android.os.SystemClock
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.audio.AudioService
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.ItemWatchedRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.player.AudioNow
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * The sound of stage 7 in the emulator with real files (concept 27 8.3): scripts/android-e2e.sh
 * writes the two-part audiobook and the podcast folder of scripts/android-test-video.sh; the test
 * adds its own two libraries over the shared web session (`DetailFlowTest.Admin`), signs the app
 * in and plays the audiobook (chapter, speed), sends the app to the background (the media session
 * plays on), comes back and reads the progress at the server; then a podcast episode; with
 * `slouchcatRadio` (RADIO=1) a station of the test stream for longer than a heartbeat, and its end.
 * At the end it removes its libraries again. With `slouchcatShots` it photographs every page (prefix `a`).
 */
@RunWith(AndroidJUnit4::class)
class AudioFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val media = args.getString("slouchcatMedia")
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val radio = args.getString("slouchcatRadio") != null
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context = instrumentation.targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)
    private val admin by lazy { DetailFlowTest.Admin(server, email, password, pin) }
    private val created = mutableMapOf<Long, String>()

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 30_000) = compose.waitForNode(matcher, timeout)

    private fun shown(matcher: SemanticsMatcher) = compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun present(matcher: SemanticsMatcher) = compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty()

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun press(label: String, timeout: Long = 20_000) {
        val matcher = (hasText(label) or hasContentDescription(label)) and hasClickAction()
        compose.waitUntil(timeout) { present(matcher) }
        button(label).performSemanticsAction(SemanticsActions.OnClick)
    }

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun jump(address: String) = instrumentation.runOnMainSync { assertTrue(graph.links.request(address)) }

    private fun items() = SlouchcatApi.create(::ItemsApi, server, graph.http)

    private fun now(): AudioNow? = graph.audio.playback.now.value

    private fun title() = now() as? AudioNow.Title

    // ---- the server -----------------------------------------------------------------------------

    /** A call of the shared web session (the account's primary profile, `DetailFlowTest.Admin`). */
    private fun call(method: String, path: String, body: String? = null): JsonElement? {
        val request = Request.Builder()
            .url(server.newBuilder().addPathSegments(SlouchcatApi.BASE_PATH).addPathSegments(path).build())
            .method(method, if (method == "GET" || method == "DELETE") null else (body ?: "").toRequestBody("application/json".toMediaType()))
            .build()
        admin.http.newCall(request).execute().use { response ->
            val text = response.body.string()
            check(response.isSuccessful) { "$method $path: ${response.code} $text" }
            return text.takeIf { it.isNotBlank() }?.let(Json::parseToJsonElement)
        }
    }

    private fun libraries() = call("GET", "admin/libraries")!!.jsonArray.map { it.jsonObject }

    /** An audio library over [folder] of the test media, scanned (its own, removed at the end). */
    private fun library(name: String, kind: String, folder: String, content: String) {
        val root = requireNotNull(media) { "run through scripts/android-e2e.sh, which writes the media" } + folder
        val existing = libraries().firstOrNull { it["name"]?.jsonPrimitive?.content == name }
        val id = existing?.get("id")?.jsonPrimitive?.long ?: call(
            "POST",
            "admin/libraries",
            """{"name":"$name","kind":"$kind","roots":[{"path":${Json.encodeToString(root)},"content":"$content","providers":["local"]}]}""",
        )!!.jsonObject["id"]!!.jsonPrimitive.long
        created[id] = name
        compose.waitUntil(SCAN_MS) {
            val state = (libraries().first { it["id"]!!.jsonPrimitive.long == id }["scan"] as JsonObject)["state"]!!.jsonPrimitive.content
            check(state != "failed" && state != "aborted") { "the scan of $name $state" }
            state == "done"
        }
    }

    /** The test stations: the first catalog sync and an offer of every country (web `global-setup.ts`). */
    private fun radioCatalog() {
        if (call("GET", "admin/radio/catalog")!!.jsonObject["stationsTotal"]!!.jsonPrimitive.int == 0) {
            call("POST", "admin/radio/catalog/sync")
            compose.waitUntil(SCAN_MS) { call("GET", "admin/radio/catalog")!!.jsonObject["stationsTotal"]!!.jsonPrimitive.int > 0 }
        }
        val offer = call("GET", "admin/radio/offer")!!.jsonObject
        val steps = offer["countries"]!!.jsonArray.map { it.jsonObject }.filter { it["total"]!!.jsonPrimitive.int > 0 }.flatMap {
            val code = it["code"]!!.jsonPrimitive.content
            listOf("""{"op":"chooseCountry","country":"$code"}""", """{"op":"markAll","country":"$code"}""")
        }
        call("POST", "admin/radio/offer/apply", """{"version":${offer["version"]},"steps":[${steps.joinToString(",")}]}""")
    }

    private fun prepareServer() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        admin.signIn()
        library(BOOKS, "audiobooks", "/Playback/Audiobooks", "audiobook")
        library(PODCASTS, "podcasts", "/Playback/Podcasts", "podcast")
        if (radio) radioCatalog()
    }

    private fun signIn() {
        compose.waitUntil(20_000) {
            ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading())
        }
        if (ready()) return
        if (shown(hasText(s(R.string.app_setup_title)))) {
            field(s(R.string.app_setup_addressLabel)).performTextInput(server.toString().trimEnd('/'))
            button(s(R.string.app_setup_connect)).performScrollTo().performClick()
            waitFor(hasText(s(R.string.app_setup_found), substring = true))
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

    /** The audiobook and the newest episode as the app sees them, once probed; no progress of an earlier run. */
    private fun titles(): Pair<String, String> {
        val items = items()
        var book: String? = null
        var episode: String? = null
        compose.waitUntil(SCAN_MS) {
            val visible = items.listVisibleLibraries().orEmpty()
            val books = visible.firstOrNull { it.name == BOOKS }?.let { items.listLibraryItems(it.id)!!.items }.orEmpty()
            val podcasts = visible.firstOrNull { it.name == PODCASTS }?.let { items.listLibraryItems(it.id)!!.items }.orEmpty()
            book = books.firstOrNull { it.title == "The Quiet Harbour" }?.id
            val podcast = podcasts.firstOrNull { it.title == "Night Shift Stories" }?.id
            episode = podcast?.let { items.getItem(it)!!.episodes?.firstOrNull()?.id }
            book != null && episode != null && (items.getItem(book!!)!!.durationMs ?: 0) > 100_000
        }
        for (id in listOf(book!!, episode!!)) {
            items.setItemWatched(id, ItemWatchedRequest(true))
            items.setItemWatched(id, ItemWatchedRequest(false))
        }
        return book!! to episode!!
    }

    private fun serverProgress(id: String) = runCatching { items().getItem(id)!!.progress }.getOrNull()

    /** The media session of the service as a controller of the system sees it (main thread of the app). */
    private fun sessionPlaying(): Boolean {
        val token = SessionToken(context, ComponentName(context, AudioService::class.java))
        val future = MediaController.Builder(context, token).buildAsync()
        val controller = future.get()
        var playing = false
        instrumentation.runOnMainSync { playing = controller.isPlaying }
        instrumentation.runOnMainSync { controller.release() }
        return playing
    }

    @After
    fun cleanUp() = runBlocking {
        instrumentation.runOnMainSync { graph.audio.playback.stop() }
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
        graph.serverStore.clear()
        // The own libraries go again (another flow on the same server sees nothing of them).
        for ((id, name) in created) runCatching { call("POST", "admin/libraries/$id/deletion", """{"confirmName":${Json.encodeToString(name)}}""") }
    }

    @Test
    fun audiobookInTheBackgroundEpisodeAndRadio() {
        prepareServer()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn()
        val (book, episode) = titles()

        // The audiobook: its page, «Abspielen», the audio player plays.
        jump("item/$book")
        waitFor(hasText("The Quiet Harbour") and isHeading())
        shot("a01-book")
        if (shots) {
            // The download dialog of the audiobook with the server's sizes in kbit/s (mockup photo 48).
            press(s(R.string.app_downloads_actionDownload))
            waitFor(hasText(s(R.string.app_audio_allParts)))
            shot("a01b-book-download")
            press(s(R.string.common_cancel))
        }
        press(s(R.string.app_item_play))
        val started = SystemClock.elapsedRealtime()
        compose.waitUntil(30_000) { title()?.state?.playing == true }
        Log.i("first sound", SystemClock.elapsedRealtime() - started)
        waitFor(hasContentDescription(s(R.string.app_audio_close)))
        shot("a02-listen")

        // The next chapter (the second file of the one timeline), then the speed.
        press(s(R.string.player_audio_nextChapter))
        compose.waitUntil(10_000) { (title()?.state?.positionMs ?: 0) >= 60_000 }
        val speed = hasContentDescription(s(R.string.player_audio_speedButton, "").trim(), substring = true) and hasClickAction()
        compose.waitUntil(10_000) { present(speed) }
        compose.onAllNodes(speed).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        shot("a03-speed")
        compose.onAllNodes(hasText("1,5×") or hasText("1.5×")).onFirst().performClick()
        compose.waitUntil(5_000) { title()?.state?.speed == 1.5f }

        // To the background: the media session plays on (the service in the foreground).
        instrumentation.uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_HOME)
        val before = title()!!.state.positionMs
        compose.waitUntil(30_000) { (title()?.state?.positionMs ?: 0) > before + 10_000 }
        assertTrue("the media session plays in the background", sessionPlaying())
        if (shots) {
            instrumentation.uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_NOTIFICATIONS)
            instrumentation.uiAutomation.waitForIdle(IDLE_MS, NOTIFICATION_SETTLE_MS)
            shot("a04-notification")
            instrumentation.uiAutomation.performGlobalAction(AccessibilityService.GLOBAL_ACTION_BACK)
        }

        // Back in front: the progress at the server follows the device, with the speed.
        context.startActivity(Intent(context, MainActivity::class.java).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK or Intent.FLAG_ACTIVITY_SINGLE_TOP))
        compose.waitUntil(30_000) { (serverProgress(book)?.positionMs ?: 0) > 70_000 }
        assertEquals(1.5, serverProgress(book)!!.playbackRate!!.toDouble(), 0.01)
        instrumentation.runOnMainSync { graph.audio.playback.togglePlay() }
        compose.waitUntil(10_000) { title()?.state?.playing == false }

        // The small player over the start page, then the podcast with its episodes.
        jump("item/$book")
        waitFor(hasContentDescription(s(R.string.app_audio_stop)))
        shot("a05-mini")
        val podcast = items().getItem(episode)!!.podcast!!.id
        jump("item/$podcast")
        waitFor(hasText("Night Shift Stories") and isHeading())
        shot("a06-podcast")
        if (shots) {
            press(s(R.string.app_downloads_actionDownload))
            waitFor(hasText(s(R.string.common_cancel)))
            shot("a06b-podcast-download")
            press(s(R.string.common_cancel))
        }
        compose.waitUntil(20_000) { present(hasContentDescription(s(R.string.item_podcast_play, "Folge 2")) and hasClickAction()) }
        press(s(R.string.item_podcast_play, "Folge 2"))
        compose.waitUntil(30_000) { title()?.state?.let { it.playing && it.item?.id == episode } == true }
        shot("a07-episode")
        // The book ended with its last progress before the episode started.
        compose.waitUntil(10_000) { (serverProgress(book)?.positionMs ?: 0) > 70_000 }
        press(s(R.string.app_audio_close))

        if (radio) {
            jump("radio")
            waitFor(hasText(s(R.string.app_radio_title)) and isHeading())
            compose.waitUntil(30_000) { present(hasText("Testradio Eins")) }
            shot("a08-radio")
            compose.onAllNodes(hasText("Testradio Eins")).onFirst().performClick()
            compose.waitUntil(30_000) { (now() as? AudioNow.Radio)?.state?.playing == true }
            // Longer than a heartbeat (30 s): the server keeps the listen and counts the time.
            val listening = SystemClock.elapsedRealtime()
            compose.waitUntil(45_000) { SystemClock.elapsedRealtime() - listening > HEARTBEAT_MS || (now() as? AudioNow.Radio)?.state?.error != null }
            assertEquals(null, (now() as? AudioNow.Radio)?.state?.error)
            assertTrue((now() as AudioNow.Radio).state.playing)
            shot("a09-radio-playing")
            press(s(R.string.app_audio_stop))
            compose.waitUntil(10_000) { now() == null }
        }
        scenario.close()
    }

    private object Log {
        fun i(what: String, ms: Long) = com.slouchcat.core.api.log.Log.i("audio-flow", what, "ms" to ms)
    }

    private companion object {
        const val BOOKS = "Android Hörbücher"
        const val PODCASTS = "Android Podcasts"
        const val SCAN_MS = 120_000L
        const val HEARTBEAT_MS = 35_000L

        /** The notification shade is drawn: the accessibility events stop for this long, at most the second. */
        const val IDLE_MS = 500L
        const val NOTIFICATION_SETTLE_MS = 5_000L
    }
}
