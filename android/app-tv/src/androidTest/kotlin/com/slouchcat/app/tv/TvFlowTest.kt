package com.slouchcat.app.tv

import android.graphics.Bitmap
import android.os.Bundle
import android.os.SystemClock
import android.view.KeyEvent
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.semantics.getOrNull
import androidx.compose.ui.test.ComposeTimeoutException
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.hasClickAction
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
import com.slouchcat.core.session.DataStoreRelaySettings
import com.slouchcat.app.tv.ui.QrContent
import com.slouchcat.core.api.ClientIdentity
import com.slouchcat.core.api.SlouchcatApi
import com.slouchcat.core.api.SlouchcatHttp
import com.slouchcat.core.api.generated.apis.SetupApi
import com.slouchcat.core.api.generated.models.SetupRequest
import com.slouchcat.core.strings.R
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl
import okhttp3.HttpUrl.Companion.toHttpUrl
import org.junit.FixMethodOrder
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.junit.runners.MethodSorters
import java.io.File
import java.io.InputStreamReader

/**
 * The way of a new TV through the app against a real server (scripts/android-tv-e2e.sh, concept 26
 * stage 2a): (a) set up by hand with the D-pad and the input of the system, profile with PIN (a
 * wrong one first), «Einstellungen» with «Server und Konto», sign this TV out; (b) set up by scan:
 * the TV shows its QR code, the test hands its content out as an instrumentation status
 * (`pairCode=…`), the script lets `apicheck -pair-phone` play the phone through the local relay,
 * the TV redeems and lands in the profile choice with server and account (K-9). Between profile
 * and settings (a) browses as stage 2b: start page, a title with «Merken», a library from the
 * rail, the search with a keyboard, the watchlist, «Profile verwalten». (c) plays real video with
 * the player of stage 3 (`c_player`).
 *
 * Arguments (`am instrument -e <name> …`): `slouchcatServer` (default `http://10.0.2.2:8697`),
 * `slouchcatRelay` (default `http://10.0.2.2:8180`), `slouchcatEmail`, `slouchcatPassword`,
 * `slouchcatPin`, `slouchcatMedia` (the folder of scripts/android-test-media.sh on the host, which
 * the test adds as a library); `slouchcatShots` saves a photo of every page and menu (the app's `files/shots/`).
 */
@RunWith(AndroidJUnit4::class)
@FixMethodOrder(MethodSorters.NAME_ASCENDING)
class TvFlowTest {
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
    private val context = instrumentation.targetContext

    /** The server as the TV shows it (K-9): scheme, host and port. */
    private val shown = server.toString().trimEnd('/')
    private val identity = ClientIdentity(ClientIdentity.Kind.TV, BuildConfig.VERSION_NAME)

    private fun s(id: Int, vararg args: Any): String = context.getString(id, *args)

    /** The account as a household has it: set up, the primary profile with a PIN. */
    private fun prepareServer() {
        // Once per run: the PIN of the profile has a limit per minute, the tests enter it themselves as well.
        if (prepared) return
        prepared = true
        val plain = SlouchcatHttp.client(identity)
        val setup = SlouchcatApi.create(::SetupApi, server, plain)
        if (setup.getSetupStatus()!!.setupRequired) setup.runSetup(SetupRequest(email = email, password = password))
        // One web session for all tests (TestLibrary): the server allows 10 sign-ins and PIN attempts a
        // minute and address, which the TV's own sign-in and the phone of the script need.
        val library = TestLibrary(server, email, password, pin)
        library.primaryWithPin()
        val folder = requireNotNull(media) { "run through scripts/android-tv-e2e.sh, which writes the media" }
        library.ensure(folder to LIBRARY, "$folder/Playback" to PLAYBACK)
    }

    /** Whether the focus is on an element with [text] now. */
    private fun isFocusedOn(text: String): Boolean =
        compose.onAllNodes(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true))).fetchSemanticsNodes().isNotEmpty()


    /** A page of «Meine Listen» from the menu «Bibliotheken»: «Alle Bibliotheken …», then down to [label], OK. */
    private fun fromMenu(label: String, shotName: String? = null) {
        rail(s(R.string.tv_shell_allLibraries))
        waitForText(s(R.string.tv_shell_myLists))
        shotName?.let(::shot)
        repeat(RAIL_MAX) {
            if (isFocusedOn(label)) {
                key(KeyEvent.KEYCODE_DPAD_CENTER)
                return
            }
            key(KeyEvent.KEYCODE_DPAD_DOWN)
        }
        throw AssertionError("no entry «$label» in the menu")
    }

    /** An entry of the rail from any page: Left into the rail, to its top, down to [label], OK. */
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
        shot("zz-rail-without-$label")
        throw AssertionError("no entry «$label» in the rail")
    }

    private fun waitFor(matcher: SemanticsMatcher, timeout: Long = 30_000, unmerged: Boolean = true) {
        compose.waitUntil(timeout) { compose.onAllNodes(matcher, useUnmergedTree = unmerged).fetchSemanticsNodes().isNotEmpty() }
    }

    private fun waitForText(text: String, timeout: Long = 30_000) = waitFor(hasText(text, substring = true), timeout)

    /** A key of the remote, as the system delivers it. */
    private fun key(code: Int, times: Int = 1) = repeat(times) {
        instrumentation.sendKeyDownUpSync(code)
        compose.waitForIdle()
    }

    /** The element with [text] has the focus (merged: the text sits on a child of the focused button). */
    private fun focused(text: String) {
        try {
            waitFor(isFocused() and (hasText(text, substring = true) or hasContentDescription(text, substring = true)), unmerged = false)
        } catch (timeout: ComposeTimeoutException) {
            // Say what has the focus instead (the reason in the report of the run).
            val now = compose.onAllNodes(isFocused()).fetchSemanticsNodes().map { node ->
                (node.config.getOrNull(SemanticsProperties.ContentDescription).orEmpty() + node.config.getOrNull(SemanticsProperties.Text).orEmpty().map { it.text }).joinToString(" ")
            }
            throw AssertionError("«$text» should have the focus, it is on $now", timeout)
        }
    }

    /** A field: OK opens the input of the system, the text goes in, «Fertig» takes it. */
    private fun type(label: String, text: String) {
        focused(label)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasSetTextAction() and hasContentDescription(label))
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performTextInput(text)
        compose.onNode(hasSetTextAction() and hasContentDescription(label)).performImeAction()
        compose.waitForIdle()
    }

    /** From the digit 1 of the pad down to «Öffnen» (bottom right) with the D-pad, and OK. */
    private fun submitPin() {
        focused("1")
        key(KeyEvent.KEYCODE_DPAD_DOWN, 3)
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        focused(s(R.string.profiles_pin_submit))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
    }

    private fun shot(name: String) {
        if (!shots) return
        compose.waitForIdle()
        instrumentation.waitForIdleSync()
        compose.mainClock.advanceTimeBy(FRAME_MS)
        compose.waitForIdle()
        // The display shows a new page a few frames after Compose drew it.
        SystemClock.sleep(FRAME_MS)
        val bitmap = instrumentation.uiAutomation.takeScreenshot() ?: return
        // The app's own files: the TV image refuses its external files to the test (EACCES on API 34).
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "$name.png").outputStream().use { bitmap.compress(Bitmap.CompressFormat.PNG, 100, it) }
    }

    private fun launch(): ActivityScenario<MainActivity> {
        runBlocking { DataStoreRelaySettings(context, "tv").set(relay) }
        return ActivityScenario.launch(MainActivity::class.java)
    }

    @Test
    fun a_byHandToSigningOut() {
        prepareServer()
        val scenario = launch()

        // The choice: the phone has the focus, the way by hand is to its right.
        waitForText(s(R.string.tv_setup_welcome))
        shot("01-setup")
        focused(s(R.string.tv_setup_phoneTitle))
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.tv_setup_handTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // Address, e-mail, password with the D-pad and the input of the system.
        waitForText(s(R.string.tv_hand_serverLead))
        focused(s(R.string.tv_hand_serverLabel))
        shot("02-hand-server")
        type(s(R.string.tv_hand_serverLabel), server.toString().trimEnd('/'))
        waitForText(server.host)
        focused(s(R.string.tv_hand_next))
        shot("03-hand-server-found")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_emailLead))
        type(s(R.string.tv_hand_emailLabel), email)
        focused(s(R.string.tv_hand_next))
        shot("04-hand-email")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_passwordLead, server.host))
        type(s(R.string.tv_hand_passwordLabel), "wrong-password-0123")
        focused(s(R.string.tv_hand_signIn))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_wrong))
        shot("05-hand-wrong-password")
        type(s(R.string.tv_hand_passwordLabel), password)
        focused(s(R.string.tv_hand_signIn))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // The profile choice says where the TV is connected (K-9); a wrong PIN first.
        waitForText(s(R.string.profiles_picker_title))
        waitForText(s(R.string.tv_profiles_connected, shown, email))
        shot("06-profiles")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_pin_prompt))
        shot("07-pin")
        "0000".forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        submitPin()
        waitForText(s(R.string.profiles_pin_invalid))
        shot("08-pin-wrong")
        pin.forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        shot("09-pin-typed")
        submitPin()

        // «Start» (stage 2b): the rows of the server; the first card has the focus, the film is in its row.
        waitForText(TITLE)
        shot("30-start")
        key(KeyEvent.KEYCODE_DPAD_LEFT)
        shot("11-rail-open")
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        repeat(ROW_MAX) { if (!isFocusedOn(TITLE)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        focused(TITLE)
        // A title: «Merken» to the server, Back to the same card.
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasContentDescription(s(R.string.tv_item_more)))
        // Stage 3: «Abspielen» leads; «Merken» beside it.
        focused(s(R.string.app_item_play))
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.item_watchlist_add))
        shot("31-item")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.item_watchlist_added))
        key(KeyEvent.KEYCODE_BACK)
        // Start reads its rows again after a page (stage 3); the card keeps the focus.
        focused(TITLE)
        // The library from the rail.
        rail(LIBRARY)
        waitForText(s(R.string.library_filters_genre))
        waitForText(TITLE)
        shot("32-library")
        // The search: a keyboard of the TV types into the field.
        rail(s(R.string.nav_search))
        focused("q")
        "night".forEach { key(KeyEvent.KEYCODE_A + (it - 'a')) }
        waitFor(hasText(TITLE) and hasClickAction().not() or hasContentDescription(TITLE, substring = true))
        shot("33-search")
        // The watchlist from the rail: the title of before is on it.
        fromMenu(s(R.string.watchlist_title))
        waitForText(s(R.string.app_lists_edit))
        focused(TITLE)
        shot("34-watchlist")

        // «Einstellungen»: «Profile verwalten» below «Profil wechseln».
        rail(s(R.string.tv_settings_title))
        waitForText(s(R.string.tv_settings_switchPane))
        shot("10-settings")
        focused(s(R.string.tv_shell_switchProfile))
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.app_profiles_manage_title))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.profiles_settings_add))
        shot("35-profiles-manage")
        key(KeyEvent.KEYCODE_BACK)
        focused(s(R.string.app_profiles_manage_title))
        key(KeyEvent.KEYCODE_DPAD_UP)
        focused(s(R.string.tv_shell_switchProfile))
        // «Erweitert»: the relay is changed here (F-21), not on the page with the code.
        // Down past «Wiedergabe» (stage 3) and «Server und Konto», «Über».
        down(s(R.string.tv_settings_advanced))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.tv_relay_label))
        shot("16-advanced-relay")
        key(KeyEvent.KEYCODE_BACK)
        focused(s(R.string.tv_settings_advanced))
        key(KeyEvent.KEYCODE_DPAD_UP, 2)
        focused(s(R.string.tv_settings_server))
        shot("12-settings-server")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_settings_tvHint))
        shot("13-server-menu")
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.tv_settings_signOut))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_settings_signOutBody))
        focused(s(R.string.common_cancel))
        shot("14-sign-out-question")
        key(KeyEvent.KEYCODE_DPAD_LEFT)
        focused(s(R.string.tv_settings_signOutConfirm))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // Signed out: the setup again.
        waitForText(s(R.string.tv_setup_welcome))
        waitForText(s(R.string.tv_settings_signedOut))
        shot("15-signed-out")
        scenario.close()
    }

    @Test
    fun b_byScan() {
        prepareServer()
        val scenario = launch()
        waitForText(s(R.string.tv_setup_welcome))
        focused(s(R.string.tv_setup_phoneTitle))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        // The QR code: its content goes out to the script, which plays the phone.
        val withCode = SemanticsMatcher.keyIsDefined(QrContent)
        waitFor(withCode)
        fun code() = compose.onNode(withCode, useUnmergedTree = true).fetchSemanticsNode().config[QrContent]
        // The pairing lives only while its page is seen (review M-1): the way by hand and back
        // gives a new code.
        val first = code()
        focused(s(R.string.tv_pair_newCode))
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.tv_pair_byHand))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_hand_serverLead))
        key(KeyEvent.KEYCODE_BACK)
        waitFor(withCode and SemanticsMatcher("another code") { it.config[QrContent] != first })
        // Back returns to the button that had the focus.
        focused(s(R.string.tv_pair_byHand))
        shot("20-scan-code")
        val code = code()
        instrumentation.sendStatus(0, Bundle().apply { putString("pairCode", code) })

        // The phone laid the packet down; the TV redeemed it and shows server and account.
        waitForText(s(R.string.tv_pair_linkedLead), timeout = 150_000)
        waitForText(email)
        waitForText(shown)
        shot("21-scan-linked")
        focused(s(R.string.tv_pair_continue))
        key(KeyEvent.KEYCODE_DPAD_CENTER)

        waitForText(s(R.string.profiles_picker_title))
        waitForText(s(R.string.tv_profiles_connected, shown, email))
        shot("22-scan-profiles")
        scenario.close()
    }

    /** Whether anything with [text] is on the screen now. */
    private fun present(text: String): Boolean =
        compose.onAllNodes(hasText(text, substring = true) or hasContentDescription(text, substring = true), useUnmergedTree = true).fetchSemanticsNodes().isNotEmpty()

    /** The card [text] on a grid page: down into the grid, then along it to the right or the left. */
    private fun toCard(text: String) {
        repeat(ROW_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_DOWN) }
        repeat(ROW_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        repeat(ROW_MAX * 2) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_LEFT) }
        focused(text)
    }

    /** Down a menu until [text] has the focus. */
    private fun down(text: String) {
        repeat(MENU_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_DOWN) }
        focused(text)
    }

    /** Along the buttons of the player to [text]: to the right, else back to the left. */
    private fun along(text: String) {
        repeat(MENU_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_RIGHT) }
        repeat(MENU_MAX) { if (!isFocusedOn(text)) key(KeyEvent.KEYCODE_DPAD_LEFT) }
        focused(text)
    }

    /** The controls of the player shown with the focus on the bar (Hoch shows hidden controls, or goes up to the bar). */
    private fun controls() {
        key(KeyEvent.KEYCODE_DPAD_UP)
        focused(bar)
    }

    /** Leaves the player: Zurück hides the controls, Zurück leaves. */
    private fun leavePlayer() {
        controls()
        key(KeyEvent.KEYCODE_BACK)
        key(KeyEvent.KEYCODE_BACK)
    }

    /** From the start of the app to «Start»: set up by hand when the TV is not, then the profile with its PIN. */
    private fun toStart() {
        waitFor(hasText(s(R.string.tv_setup_welcome)) or hasText(s(R.string.profiles_picker_title)) or hasText(TITLE, substring = true), timeout = 60_000)
        // Still signed in with a profile (an earlier test stopped half way): Start already.
        if (present(TITLE)) return
        if (present(s(R.string.tv_setup_welcome))) {
            focused(s(R.string.tv_setup_phoneTitle))
            key(KeyEvent.KEYCODE_DPAD_RIGHT)
            key(KeyEvent.KEYCODE_DPAD_CENTER)
            waitForText(s(R.string.tv_hand_serverLead))
            type(s(R.string.tv_hand_serverLabel), shown)
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
        waitForText(s(R.string.profiles_picker_title))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_pin_prompt))
        pin.forEach { key(KeyEvent.KEYCODE_0 + (it - '0')) }
        submitPin()
    }

    /** The lines «first picture» the player logged (debug build), also into `first-pictures.txt` beside the photos. */
    private fun firstPictures(): List<String> {
        val process = Runtime.getRuntime().exec(arrayOf("logcat", "-d", "-v", "raw", "-s", "SlouchcatTv/player:I"))
        val lines = InputStreamReader(process.inputStream).readLines().filter { it.contains("first picture") }
        val dir = File(context.filesDir, "shots").apply { mkdirs() }
        File(dir, "first-pictures.txt").writeText(lines.joinToString("\n", postfix = "\n"))
        return lines
    }

    private fun waitForFirstPictures(count: Int) =
        compose.waitUntil(PLAY_MS) { firstPictures().size >= count }

    private val bar get() = s(R.string.player_seek)

    /**
     * (c) The player of stage 3 with real video (scripts/android-test-video.sh): the film plays
     * directly, pauses, spools, takes another sound track and subtitles from the menus, converts on
     * the server for a lower quality (HLS), is left with Zurück in two steps and resumes; an episode
     * runs up to the countdown, the next one plays; a file that is no video shows the error picture.
     * The times to the first picture go to `first-pictures.txt`.
     */
    @Test
    fun c_player() {
        prepareServer()
        Runtime.getRuntime().exec(arrayOf("logcat", "-c")).waitFor()
        val scenario = launch()
        toStart()
        waitForText(TITLE)

        // «Einstellungen › Wiedergabe»: quality, languages, series, sound.
        rail(s(R.string.tv_settings_title))
        waitForText(s(R.string.tv_settings_switchPane))
        down(s(R.string.app_playback_title))
        shot("59-settings")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.app_playback_quality))
        waitForText(s(R.string.tv_playback_soundHelp))
        shot("60-settings-playback")
        key(KeyEvent.KEYCODE_BACK)

        // The film plays directly: the controls, pause, spool, sound and subtitles from the menus.
        rail(PLAYBACK)
        waitForText(FILM)
        toCard(FILM)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasContentDescription(s(R.string.tv_item_more)))
        focused(s(R.string.app_item_play))
        shot("61-item-play")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(bar)
        waitForFirstPictures(1)
        shot("62-player-controls")
        // Home and back (TV-NP): the session ends, the film comes back paused at its place.
        scenario.moveToState(Lifecycle.State.CREATED)
        scenario.moveToState(Lifecycle.State.RESUMED)
        waitForFirstPictures(2)
        controls()
        waitForText(s(R.string.tv_player_paused))
        shot("62b-back-from-home")
        // OK plays on (the next steps pause it again).
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        controls()
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.tv_player_paused))
        shot("63-paused")
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 3)
        waitForText("+")
        shot("64-seek-preview")
        // One seek when the keys stop: the bubble goes.
        compose.waitUntil(PLAY_MS) { !present("+") }
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        focused(s(R.string.player_tracks_audio))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused("Deutsch")
        shot("65-menu-audio")
        down("English")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.player_tracks_audio))
        key(KeyEvent.KEYCODE_DPAD_RIGHT)
        focused(s(R.string.player_tracks_subtitles))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.player_tracks_off))
        shot("66-menu-subtitles")
        down("Deutsch")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(s(R.string.player_tracks_subtitles))
        along(s(R.string.tv_player_info))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText(s(R.string.app_player_info_direct))
        waitForText("English")
        shot("67-info")
        key(KeyEvent.KEYCODE_BACK)
        // A quality below the file: the server converts it, HLS.
        along(s(R.string.player_settings_quality))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        shot("69-menu-quality")
        down(QUALITY)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForFirstPictures(3)
        // Zurück hides the controls, then leaves; the title resumes where it stood.
        controls()
        key(KeyEvent.KEYCODE_BACK)
        shot("70-controls-hidden")
        key(KeyEvent.KEYCODE_BACK)
        waitForText(s(R.string.app_item_resume), timeout = PLAY_MS)
        focused(s(R.string.app_item_resume))
        shot("71-item-resume")
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(bar)
        waitForFirstPictures(4)
        leavePlayer()
        focused(s(R.string.app_item_resume))
        key(KeyEvent.KEYCODE_BACK)

        // An episode: the list of the season, then up to the end credits and the countdown.
        rail(PLAYBACK)
        waitForText(SERIES)
        toCard(SERIES)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasContentDescription(s(R.string.tv_item_more)))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        focused(bar)
        waitForFirstPictures(5)
        controls()
        key(KeyEvent.KEYCODE_DPAD_DOWN)
        along(s(R.string.app_player_episodes))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitForText("Episode 2")
        shot("72-menu-episodes")
        key(KeyEvent.KEYCODE_BACK)
        controls()
        key(KeyEvent.KEYCODE_DPAD_RIGHT, 4)
        waitFor(hasText(s(R.string.app_player_playNow)), timeout = PLAY_MS)
        focused(s(R.string.app_player_playNow))
        shot("73-countdown")
        // The countdown runs out: episode 2 plays.
        waitForFirstPictures(6)
        shot("74-next-episode")
        leavePlayer()
        key(KeyEvent.KEYCODE_BACK)

        // A file that is no video: the error picture with «Zurück».
        rail(LIBRARY)
        waitForText(TITLE)
        toCard(TITLE)
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasContentDescription(s(R.string.tv_item_more)))
        focused(s(R.string.app_item_play))
        key(KeyEvent.KEYCODE_DPAD_CENTER)
        waitFor(hasText(s(R.string.common_back)) and hasClickAction(), timeout = PLAY_MS, unmerged = false)
        shot("75-error")
        firstPictures()
        scenario.close()
    }

    private companion object {
        /** The server of this run is set up and has its libraries. */
        var prepared = false
        const val FRAME_MS = 500L

        /** The clips of scripts/android-test-video.sh and their library. */
        const val FILM = "Two Voices"
        const val SERIES = "Clip Show"
        const val PLAYBACK = "Wiedergabe-Test"
        const val QUALITY = "480p"
        const val MENU_MAX = 10

        /** How long the emulator may take to load a stream (a transcode in software). */
        const val PLAY_MS = 90_000L

        /** The film of scripts/android-test-media.sh and the library the test makes of it. */
        const val TITLE = "Night Train"
        const val LIBRARY = "Testmedien"
        const val RAIL_MAX = 12

        /** Left often enough to leave a row from its last card for the rail (Left in the rail does nothing). */
        const val LEFT_MAX = 8
        const val ROW_MAX = 4
    }
}
