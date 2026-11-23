package com.slouchcat.app.mobile.store

import android.Manifest
import android.graphics.Bitmap
import android.os.Build
import android.os.SystemClock
import androidx.activity.compose.setContent
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
import com.slouchcat.app.mobile.AppGraph
import com.slouchcat.app.mobile.BuildConfig
import com.slouchcat.app.mobile.HandedScanner
import com.slouchcat.app.mobile.MainActivity
import com.slouchcat.app.mobile.SlouchcatApplication
import com.slouchcat.app.mobile.isShown
import com.slouchcat.app.mobile.navigation.SlouchcatNavHost
import com.slouchcat.app.mobile.screenReport
import com.slouchcat.app.mobile.shell.AppLink
import com.slouchcat.app.mobile.shell.features
import com.slouchcat.app.mobile.tvlink.TvLinkFeature
import com.slouchcat.app.mobile.waitForNode
import com.slouchcat.core.api.ApiCaller
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SessionEvents
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.AuthApi
import com.slouchcat.core.api.generated.apis.ItemsApi
import com.slouchcat.core.api.generated.apis.SystemApi
import com.slouchcat.core.api.generated.models.AppLoginRequest
import com.slouchcat.core.api.generated.models.SelectProfileRequest
import com.slouchcat.core.design.SlouchcatTheme
import com.slouchcat.core.pairing.ApiRedeemer
import com.slouchcat.core.pairing.RelayClient
import com.slouchcat.core.pairing.TvPairing
import com.slouchcat.core.session.AccountStore
import com.slouchcat.core.session.ActiveProfile
import com.slouchcat.core.session.MemoryRelaySettings
import com.slouchcat.core.session.SavedServer
import com.slouchcat.core.session.ServerStore
import com.slouchcat.core.session.SessionState
import com.slouchcat.core.session.StoredCredentials
import com.slouchcat.core.session.TokenStore
import com.slouchcat.core.strings.R
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import okhttp3.OkHttpClient
import org.junit.Before
import org.junit.FixMethodOrder
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.MethodSorters
import java.io.File

/**
 * The photos of the store listing (docs/store/grafiken.md) against the demo server of
 * scripts/store-screenshots.sh: the four open films of the Blender Foundation, an audiobook of
 * scripts/android-test-video.sh and an account with two profiles. Only that script runs it
 * (`-e class`, with the argument `slouchcatStore`); the flow of scripts/android-e2e.sh leaves it
 * out (`notClass`).
 *
 * The script runs the methods one by one: `phone` and `phoneTvLink` on the phone, `tablet` on the
 * tablet, `player` on both (on the phone with a display of 1080 × 1920, so the landscape photo is
 * 1920 × 1080). Photos go to the app's `files/store/` as `<nn>-<motif>.png`.
 *
 * Arguments: `slouchcatServer`, `slouchcatRelay`, `slouchcatEmail`, `slouchcatPassword`,
 * `slouchcatPin` (of the primary profile), `slouchcatLibrary` (the name of the film library in
 * the side column of the tablet).
 */
@RunWith(AndroidJUnit4::class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
class StoreShotsTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private val instrumentation = InstrumentationRegistry.getInstrumentation()
    private val args = InstrumentationRegistry.getArguments()
    private val server: HttpUrl = (args.getString("slouchcatServer") ?: "http://10.0.2.2:8897").toHttpUrl()
    private val relay: HttpUrl? = args.getString("slouchcatRelay")?.toHttpUrl()
    private val email = args.getString("slouchcatEmail") ?: "demo@example.com"
    private val password = args.getString("slouchcatPassword") ?: "demo-password-0123"
    private val pin = args.getString("slouchcatPin") ?: "246810"
    private val library = args.getString("slouchcatLibrary") ?: "Films"
    private val context = instrumentation.targetContext
    private val identity = ClientIdentity(ClientIdentity.Kind.MOBILE, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    @Before
    fun onlyForTheStore() = check(args.getString("slouchcatStore") != null) { "store photos only through scripts/store-screenshots.sh" }

    private fun shown(matcher: SemanticsMatcher) = compose.isShown(matcher, unmerged = true)

    private fun present(matcher: SemanticsMatcher) = compose.isShown(matcher)

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 30_000) = compose.waitForNode(matcher, timeout)

    private fun waitForText(text: String, timeout: Long = 30_000) = waitFor(hasText(text, substring = true), timeout)

    private fun button(text: String) = compose.onAllNodes((hasText(text) or hasContentDescription(text)) and hasClickAction()).onLast()

    private fun press(text: String, timeout: Long = 30_000) {
        val matcher = (hasText(text) or hasContentDescription(text)) and hasClickAction()
        compose.waitUntil(timeout) { present(matcher) }
        button(text).performSemanticsAction(SemanticsActions.OnClick)
    }

    private fun field(label: String) = compose.onNode(hasSetTextAction() and hasText(label))

    private val graph: AppGraph get() = (context.applicationContext as SlouchcatApplication).graph

    private fun jump(address: String) = instrumentation.runOnMainSync { check(graph.links.request(address)) { "no page for $address" } }

    private fun ready(): Boolean = (graph.session.state.value as? SessionState.Ready)?.profile != null

    /**
     * A photo once the screen holds still: [STILL_COUNT] screenshots in a row the same (pictures
     * loaded, the page settled; two alone once caught a poster still on its way), at most [settleMs]
     * long; a screen that never holds still (a running clock) is taken as it is then. Not
     * `Showcase.shot`: that one is bound to `slouchcatShots` and `files/shots`.
     */
    private fun photo(name: String, settleMs: Long = SETTLE_MS) {
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        val deadline = SystemClock.uptimeMillis() + settleMs
        var last: Bitmap? = null
        var bitmap: Bitmap
        var same = 1
        while (true) {
            bitmap = requireNotNull(instrumentation.uiAutomation.takeScreenshot()) { "no screenshot" }
            same = if (last != null && bitmap.sameAs(last)) same + 1 else 1
            if (same >= STILL_COUNT) break
            last?.recycle()
            last = bitmap
            if (SystemClock.uptimeMillis() > deadline) break
            compose.waitForIdle()
            instrumentation.uiAutomation.waitForIdle(STILL_MS, STILL_MAX_MS)
        }
        val dir = File(context.getExternalFilesDir(null), "store").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    /** The id of the title named [title] in the profile's libraries, read as the app. */
    private fun itemId(title: String): String {
        val items = SlouchcatApi.create(::ItemsApi, server, graph.http)
        var found: String? = null
        compose.waitUntil(SCAN_MS) {
            // A failed request or an empty answer is «not yet there», not the end of the wait.
            found = runCatching {
                items.listVisibleLibraries().orEmpty().firstNotNullOfOrNull { lib ->
                    items.listLibraryItems(lib.id)?.items?.firstOrNull { it.title == title }?.id
                }
            }.getOrNull()
            found != null
        }
        return checkNotNull(found)
    }

    /** From the address to a chosen profile; [pickerPhoto] photographs the profile choice on the way. */
    private fun signIn(pickerPhoto: String? = null) {
        compose.waitUntil(30_000) { ready() || shown(hasText(s(R.string.app_setup_title))) || shown(hasText(s(R.string.login_title)) and isHeading()) }
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
        val primary = hasContentDescription(s(R.string.profiles_picker_primary), substring = true) and hasClickAction()
        compose.waitUntil(30_000) { ready() || present(primary) }
        if (!ready()) {
            if (pickerPhoto != null) {
                waitForText(s(R.string.profiles_picker_title))
                photo(pickerPhoto)
            }
            compose.onAllNodes(primary).onLast().performClick()
            compose.waitUntil(30_000) { ready() || shown(hasText(s(R.string.profiles_pin_hint))) }
            if (!ready()) {
                pin.forEach { button(it.toString()).performClick() }
                compose.onAllNodes(hasContentDescription(s(R.string.profiles_pin_submit))).onLast().performClick()
            }
        }
        compose.waitUntil(30_000) { ready() }
    }

    /** The start page with its rows: the films are there. */
    private fun startPage() {
        waitFor(hasText(TEARS) or hasContentDescription(TEARS, substring = true), SCAN_MS)
        waitFor(hasText(SINTEL) or hasContentDescription(SINTEL, substring = true))
    }

    /**
     * The big button of a detail page: «Abspielen», or «Weiterschauen» once the player of this
     * language ran (on the phone before the tablet).
     */
    private val play get() = (hasText(s(R.string.app_item_play)) or hasText(s(R.string.app_item_resume))) and hasClickAction()

    /** The detail page of Tears of Steel: heading and its big button; [tech] opens «Datei und Technik». */
    private fun detail(id: String, tech: Boolean = false) {
        jump("item/$id")
        waitFor(hasText(TEARS) and isHeading())
        try {
            compose.waitUntil(30_000) { present(play) }
        } catch (e: androidx.compose.ui.test.ComposeTimeoutException) {
            throw AssertionError("no «${s(R.string.app_item_play)}»\n${compose.screenReport("detail-play")}", e)
        }
        // The whole button in view (the wide page puts it below the cover, at the lower edge).
        compose.onAllNodes(play).onFirst().performScrollTo()
        if (tech) {
            button(s(R.string.app_item_tech)).performScrollTo().performClick()
            waitForText(s(R.string.app_item_techAudio))
        }
    }

    /** Phone: 01 start, 02 detail, 04 downloads, 05 profile choice, 06 audiobook. */
    @Test
    fun phone() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) instrumentation.uiAutomation.grantRuntimePermission(context.packageName, Manifest.permission.POST_NOTIFICATIONS)
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn(pickerPhoto = "05-profiles")
        startPage()
        photo("01-home")

        val tears = itemId(TEARS)
        detail(tears, tech = true)
        photo("02-detail")

        // Downloads: Sintel in the lowest quality, done.
        val sintel = itemId(SINTEL)
        jump("item/$sintel")
        waitFor(hasText(SINTEL) and isHeading())
        button(s(R.string.app_downloads_actionDownload)).performScrollTo().performClick()
        waitFor(hasText(s(R.string.app_downloads_dialogTitle)) and isHeading())
        val low = hasText(s(R.string.offline_quality_low), substring = true) and isSelectable()
        compose.waitUntil(60_000) { present(low) }
        compose.onAllNodes(low).onFirst().performScrollTo().performClick()
        button(s(R.string.app_downloads_start)).performClick()
        jump("downloads")
        compose.waitUntil(DOWNLOAD_MS) { runBlocking { graph.offline.engine.allDownloads() }.any { it.complete } }
        waitForText(s(R.string.app_downloads_sectionDone))
        photo("04-downloads")

        // The audiobook plays in the sound player.
        val book = itemId(BOOK)
        jump("item/$book")
        waitFor(hasText(BOOK) and isHeading())
        press(s(R.string.app_item_play))
        waitFor(hasContentDescription(s(R.string.app_audio_close)))
        compose.waitUntil(30_000) { graph.audio.playback.now.value != null }
        photo("06-audiobook", settleMs = AUDIO_SETTLE_MS)
        instrumentation.runOnMainSync { graph.audio.playback.stop() }
        scenario.close()
    }

    /** Tablet: 01 start with the side column, 02 the film library as a grid, 03 detail. */
    @Test
    fun tablet() {
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn()
        startPage()
        photo("01-home")
        button(library).performClick()
        waitFor(hasText(library) and isHeading())
        waitFor(hasText(TEARS) or hasContentDescription(TEARS, substring = true))
        photo("02-library")
        // The wide page shows cover, texts and the button without opening «Datei und Technik».
        detail(itemId(TEARS))
        photo("03-detail")
        scenario.close()
    }

    /**
     * The player while spooling: Tears of Steel plays, pauses, and a finger drags the knob along
     * the bar; the photo shows the bubble with the preview tile of the server's trickplay set.
     */
    @Test
    fun player() {
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        signIn()
        val tears = itemId(TEARS)
        // The preview tiles of the film (library setting «Im Voraus»): ready before the player opens.
        val items = SlouchcatApi.create(::ItemsApi, server, graph.http)
        val file = items.getItem(tears)!!.mediaFiles.first().id
        compose.waitUntil(TRICKPLAY_MS) { runCatching { items.getTrickplay(tears, file)?.state?.value }.getOrNull() == "complete" }
        detail(tears)
        compose.onAllNodes(play).onFirst().performSemanticsAction(SemanticsActions.OnClick)
        val close = hasContentDescription(s(R.string.app_player_close)) and hasClickAction()
        val pause = hasContentDescription(s(R.string.player_pause)) and hasClickAction()
        compose.waitUntil(60_000) {
            if (!present(close)) compose.onRoot().performTouchInput { click(Offset(width * 0.5f, height * 0.3f)) }
            present(pause)
        }
        // A spot with a picture, then pause: the photo shows a still frame under the bubble.
        compose.onAllNodes(hasContentDescription(s(R.string.player_seek))).onFirst().performSemanticsAction(SemanticsActions.SetProgress) { it(SEEK_FROM) }
        compose.waitUntil(30_000) {
            if (!present(close)) compose.onRoot().performTouchInput { click(Offset(width * 0.5f, height * 0.3f)) }
            present(pause)
        }
        press(s(R.string.player_pause))
        compose.waitUntil(10_000) { present(hasContentDescription(s(R.string.player_play)) and hasClickAction()) }
        // The finger stays down while the photo is taken.
        val bar = compose.onAllNodes(hasContentDescription(s(R.string.player_seek))).onFirst()
        bar.performTouchInput {
            down(Offset(width * SEEK_FROM, height / 2f))
            moveTo(Offset(width * (SEEK_FROM + 0.05f), height / 2f))
            moveTo(Offset(width * SEEK_TO, height / 2f))
        }
        photo("03-player")
        bar.performTouchInput { up() }
        scenario.close()
    }

    /** «Mehr › Fernseher verbinden» up to the question before pairing (07); the TV is :core:pairing over the local relay. */
    @Test
    fun phoneTvLink() {
        val relay = requireNotNull(relay) { "the photo needs the local relay: -e slouchcatRelay http://10.0.2.2:<port>" }
        val plain = SlouchcatHttp.client(identity)
        val health = SlouchcatApi.create(::SystemApi, server, plain).getHealth()!!
        val login = SlouchcatApi.create(::AuthApi, server, plain).appLogin(AppLoginRequest(email, password, "Pixel", AppLoginRequest.Kind.PHONE))!!
        val credentials = object : com.slouchcat.core.api.SessionCredentials {
            override val server = this@StoreShotsTest.server
            override val bearerToken = login.token
            override val deviceId: String? = null
        }
        val auth = SlouchcatApi.create(::AuthApi, server, SlouchcatHttp.client(identity, credentials))
        val primary = auth.listMyProfiles()!!.single { it.isPrimary }
        auth.selectProfile(primary.id, if (primary.hasPin) SelectProfileRequest(pin) else null)
        val scanner = HandedScanner()
        val feature = TvLinkFeature(scanner = scanner, relaySettings = { MemoryRelaySettings(relay) })
        lateinit var linkGraph: AppGraph
        instrumentation.runOnMainSync {
            linkGraph = AppGraph(
                context,
                features.map { if (it is TvLinkFeature) feature else it },
                MemoryServers(SavedServer(server, health.version, serverKey = health.serverKey)),
                MemoryTokens(StoredCredentials(login.token, login.deviceId)),
                MemoryAccounts(email, ActiveProfile(primary.id, primary.name, isPrimary = true)),
            )
        }
        val scenario = ActivityScenario.launch(MainActivity::class.java)
        scenario.onActivity { it.setContent { SlouchcatTheme { SlouchcatNavHost(linkGraph) } } }
        waitFor(hasContentDescription(s(R.string.app_more_title)) or hasText(s(R.string.app_more_title)))
        instrumentation.runOnMainSync { linkGraph.links.request(AppLink("more", emptyList())) }
        waitForText(s(R.string.app_tv_moreHint))
        button(s(R.string.app_tv_title)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_intro_title))

        // The TV shows its code (the TV app's own pairing code); the phone scans it and asks.
        val tvIdentity = ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)
        val redeemer = ApiRedeemer(ApiCaller(SessionEvents())) { _, _ -> SlouchcatHttp.client(tvIdentity, builder = OkHttpClient.Builder().followRedirects(false)) }
        val tv = TvPairing(RelayClient(OkHttpClient(), relay), redeemer)
        val meeting = (runBlocking { tv.start() } as TvPairing.Started.Ok).meeting
        scanner.next = CompletableDeferred()
        button(s(R.string.app_tv_intro_scan)).performScrollTo().performClick()
        waitForText(s(R.string.app_tv_scan_aim))
        scanner.next.complete(meeting.code.text)
        waitForText(s(R.string.app_tv_confirm_title))
        waitForText(email)
        photo("07-tv-link")
        scenario.close()
    }

    private class MemoryServers(server: SavedServer) : ServerStore {
        private val flow = MutableStateFlow<SavedServer?>(server)
        override val server: Flow<SavedServer?> = flow
        override suspend fun save(server: SavedServer) {
            flow.value = server
        }
        override suspend fun confirmKey(serverKey: String) = Unit
        override suspend fun updateVersion(version: String) = Unit
        override suspend fun clear() {
            flow.value = null
        }
    }

    private class MemoryTokens(private var stored: StoredCredentials?) : TokenStore {
        override suspend fun read() = stored
        override suspend fun write(credentials: StoredCredentials) {
            stored = credentials
        }
    }

    private class MemoryAccounts(private var email: String?, private var profile: ActiveProfile?) : AccountStore {
        override suspend fun read() = email to profile
        override suspend fun write(email: String?, profile: ActiveProfile?) {
            this.email = email
            this.profile = profile
        }
    }

    private companion object {
        const val TEARS = "Tears of Steel"
        const val SINTEL = "Sintel"
        const val BOOK = "The Quiet Harbour"
        /** The first scan of four films and an audiobook, probes included (seconds on this machine). */
        const val SCAN_MS = 120_000L

        /** A film of 15 minutes converted in software for the download, then fetched (about 2 min here). */
        const val DOWNLOAD_MS = 600_000L

        /** The preview tiles of the backfill in software encoding, behind the scan's own jobs (seconds here). */
        const val TRICKPLAY_MS = 900_000L

        /** Long enough for the posters of a page to load from the local server. */
        const val SETTLE_MS = 15_000L

        /** The sound player's clock runs every second: it never holds still, so a short wait. */
        const val AUDIO_SETTLE_MS = 3_000L
        const val STILL_COUNT = 3
        const val STILL_MS = 500L
        const val STILL_MAX_MS = 3_000L
        const val SEEK_FROM = 0.40f
        const val SEEK_TO = 0.62f
    }
}
