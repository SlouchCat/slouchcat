package com.slouchcat.app.mobile

import android.Manifest
import android.graphics.Bitmap
import android.os.Build
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.click
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasContentDescription
import androidx.compose.ui.test.hasSetTextAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isHeading
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onFirst
import androidx.compose.ui.test.onLast
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performSemanticsAction
import androidx.compose.ui.test.performTextInput
import androidx.compose.ui.test.performTouchInput
import androidx.test.core.app.ActivityScenario
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.slouchcat.app.mobile.downloads.offline
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionCredentials
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.AccountApi
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.api.log.Log
import com.slouchcat.core.offline.Contact
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

/**
 * Stage 6 in the emulator (concept 27 13, «6 Downloads»): the film of scripts/android-test-video.sh
 * is downloaded from its detail page in the lowest quality, the network goes away (Wi-Fi and mobile
 * data off, as the flight mode does for the app), the app shows the offline library and plays the
 * film from the device, seeks, leaves; with the network back the sync uploads the offline progress
 * and the server has the position. Then another session of the same account signs this device out
 * under «Meine Geräte»: at the next contact the files are gone. It logs the fetch rate, the time
 * to the first picture offline and the duration of the sync (tag `Slouchcat/e2e-offline`).
 * With `slouchcatShots` it photographs the steps (prefix `o`).
 */
@RunWith(AndroidJUnit4::class)
class OfflineFlowTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8597").toHttpUrl()
    private val media = args.getString("slouchcatMedia")
    private val email = args.getString("slouchcatEmail") ?: "e2e-owner@example.test"
    private val password = args.getString("slouchcatPassword") ?: "e2e-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val shots = args.getString("slouchcatShots") != null
    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val context = instrumentation.targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    private fun shown(matcher: SemanticsMatcher) = compose.onAllNodes(matcher, useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    private fun present(matcher: SemanticsMatcher) = compose.onAllNodes(matcher).fetchSemanticsNodes().isNotEmpty()

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 20_000) = compose.waitUntil(timeout) { shown(matcher) }

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private val measures = mutableListOf<String>()

    /** A measure into the log and into `offline-measures.txt` beside the photos (docs/perf/K27-android-handy.md). */
    private fun measure(line: String) {
        Log.i(TAG, "measure", "value" to line)
        measures += line
        val dir = File(context.getExternalFilesDir(null), "shots").apply { mkdirs() }
        File(dir, "offline-measures.txt").writeText(measures.joinToString("\n", postfix = "\n"))
    }

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun jump(address: String) = instrumentation.runOnMainSync { assertTrue(graph.links.request(address)) }

    private fun shell(command: String) = instrumentation.uiAutomation.executeShellCommand(command).close()

    /** Wi-Fi and mobile data off or on: for the app the same as the flight mode. */
    private fun network(on: Boolean) {
        shell("svc wifi ${if (on) "enable" else "disable"}")
        shell("svc data ${if (on) "enable" else "disable"}")
    }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    private fun signIn() {
        compose.waitUntil(20_000) { ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading()) }
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

    /** The account (without the seeded media) and the library of the test clips, before the app signs in. */
    private fun prepareServer() {
        val setup = SlouchcatApi.create(::SetupApi, server, SlouchcatHttp.client(identity))
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        DetailFlowTest.Admin(server, email, password, pin).ensureLibrary(requireNotNull(media) { "run through scripts/android-e2e.sh" }, name = LIBRARY, folder = "/Playback")
    }

    private fun film(): String {
        val items = SlouchcatApi.create(::ItemsApi, server, graph.http)
        val library = items.listVisibleLibraries()!!.single { it.name == LIBRARY }
        val id = items.listLibraryItems(library.id)!!.items.single { it.title == FILM }.id
        val deadline = System.nanoTime() + WAIT_NS
        while (items.getItem(id)!!.mediaFiles.firstOrNull()?.videoCodec == null) {
            check(System.nanoTime() < deadline) { "the test film was not probed" }
            Thread.sleep(POLL_MS)
        }
        return id
    }

    private val engine get() = graph.offline.engine

    private fun downloads() = runBlocking { engine.allDownloads() }

    @After
    fun cleanUp() = runBlocking {
        network(true)
        if (graph.session.state.value is SessionState.Ready) graph.sessionActions.signOut()
        graph.serverStore.clear()
    }

    @Test
    fun downloadPlayOfflineSyncAndRevoke() {
        // The notification of the transfer may show without asking (Android 13 and later).
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) instrumentation.uiAutomation.grantRuntimePermission(context.packageName, Manifest.permission.POST_NOTIFICATIONS)
        network(true)
        Runtime.getRuntime().exec(arrayOf("logcat", "-c")).waitFor()
        prepareServer()
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn()
        val film = film()

        // Order from the detail page: the dialog, the lowest quality.
        jump("item/$film")
        waitFor(hasText(FILM) and isHeading())
        button(s(R.string.app_downloads_actionDownload)).performScrollTo().performClick()
        waitFor(hasText(s(R.string.app_downloads_dialogTitle)) and isHeading())
        val low = hasText(s(R.string.offline_quality_low), substring = true) and isSelectable()
        compose.waitUntil(30_000) { present(low) }
        shot("o01-dialog")
        compose.onAllNodes(low).onFirst().performScrollTo().performClick()
        val ordered = System.nanoTime()
        button(s(R.string.app_downloads_start)).performClick()

        // The server prepares, the transfer fetches; the page shows it.
        jump("downloads")
        waitFor(hasText(s(R.string.app_downloads_sectionWaiting)) or hasText(s(R.string.app_downloads_sectionActive)) or hasText(s(R.string.app_downloads_sectionDone)))
        shot("o02-downloads-running")
        var fetchStart = 0L
        val deadline = System.nanoTime() + WAIT_NS
        while (downloads().none { it.complete }) {
            check(System.nanoTime() < deadline) { "the download did not complete: ${downloads().map { it.state.value + "/" + it.fetchError }}" }
            if (fetchStart == 0L && downloads().any { it.manifest != null }) fetchStart = System.nanoTime()
            Thread.sleep(POLL_MS)
        }
        val done = downloads().single { it.complete }
        val seconds = (System.nanoTime() - (if (fetchStart > 0) fetchStart else ordered)) / 1e9
        measure("fetch bytes=${done.bytesWritten} seconds=${"%.2f".format(seconds)} order-to-complete=${"%.1f".format((System.nanoTime() - ordered) / 1e9)}s")
        waitFor(hasText(s(R.string.app_downloads_sectionDone)))
        shot("o03-downloads-done")

        // Without network: the offline banner, playing from the device.
        network(false)
        compose.waitUntil(30_000) { runBlocking { engine.sync() } == Contact.Offline }
        jump("downloads")
        waitFor(hasText(s(R.string.app_downloads_offlineBanner)))
        shot("o04-offline")
        val play = hasContentDescription(s(R.string.app_downloads_play, FILM)) and hasClickAction()
        compose.onAllNodes(play).onFirst().performScrollTo().performClick()
        val seek = hasContentDescription(s(R.string.player_seek))
        val pause = hasContentDescription(s(R.string.player_pause)) and hasClickAction()
        val opened = System.nanoTime()
        // Playing: the controls show «Pause» (a tap on the picture shows them).
        compose.waitUntil(30_000) {
            if (!present(pause)) compose.onRoot().performTouchInput { click(Offset(width * 0.5f, height * 0.3f)) }
            present(pause)
        }
        measure("offline player playing after ms=${(System.nanoTime() - opened) / 1_000_000}")
        compose.onAllNodes(seek).onFirst().performSemanticsAction(SemanticsActions.SetProgress) { it(0.5f) }
        compose.waitUntil(30_000) {
            if (!present(pause)) compose.onRoot().performTouchInput { click(Offset(width * 0.5f, height * 0.3f)) }
            present(pause)
        }
        shot("o05-offline-player")
        // Pause writes the progress event and the position on the device.
        compose.onAllNodes(pause).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        compose.waitUntil(20_000) { (downloads().single().positions.values.maxOfOrNull { it.positionMs } ?: 0) > 0 }
        instrumentation.runOnMainSync { scenario.onActivity { it.onBackPressedDispatcher.onBackPressed() } }
        val offlinePosition = downloads().single().positions.values.maxOf { it.positionMs }
        val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-v", "raw", "-s", "Slouchcat/player:I"))
        java.io.InputStreamReader(process.inputStream).readLines().filter { it.contains("first picture") }.forEach { measure("offline $it") }

        // Network back: the sync uploads the progress; the server has the position.
        network(true)
        var synced = Contact.Offline
        val started = System.nanoTime()
        compose.waitUntil(60_000) {
            synced = runBlocking { engine.sync() }
            synced == Contact.Online
        }
        measure("sync after network back ms=${(System.nanoTime() - started) / 1_000_000}")
        val items = SlouchcatApi.create(::ItemsApi, server, graph.http)
        compose.waitUntil(30_000) { (items.getItem(film)!!.progress?.positionMs ?: 0) > 0 }
        measure("offline position local=$offlinePosition server=${items.getItem(film)!!.progress?.positionMs}")

        // Another session of the account signs this device out (as under «Meine Geräte»): files gone.
        val device = requireNotNull(graph.session.knownDeviceId)
        val folder = engine.files.folder(done.id)
        assertTrue(folder.exists())
        val other = SlouchcatApi.create(::AuthApi, server, SlouchcatHttp.client(identity))
            .appLogin(AppLoginRequest(email, password, "e2e second phone", AppLoginRequest.Kind.PHONE))!!
        val otherHttp = SlouchcatHttp.client(
            identity,
            object : SessionCredentials {
                override val server = this@OfflineFlowTest.server
                override val bearerToken = other.token
                override val deviceId: String? = null
            },
        )
        SlouchcatApi.create(::AccountApi, server, otherHttp).signOutMyDevice(device)
        compose.waitUntil(60_000) {
            runBlocking { engine.sync() }
            downloads().isEmpty()
        }
        assertFalse(folder.exists())
        scenario.close()
    }

    private companion object {
        const val TAG = "e2e-offline"
        const val LIBRARY = "Android playback media"
        const val FILM = "Two Voices"
        const val WAIT_NS = 300_000_000_000L
        const val POLL_MS = 500L
    }
}
